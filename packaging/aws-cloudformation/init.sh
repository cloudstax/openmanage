#!/bin/bash -x

version=$1
clusterName=$2
containerPlatform=$3
containerPlatformRole=$4
azs=$5

org="jazzl0ver/"

if [ "$version" = "" -o "$clusterName" = "" -o "$azs" = "" ]; then
  echo "version $version, clusterName $clusterName and azs $azs should not be empty"
  exit 1
fi

if [ "$containerPlatform" != "ecs" -a "$containerPlatform" != "swarm" ]; then
  echo "invalid container platform $containerPlatform"
  exit 1
fi

if [ "$containerPlatformRole" != "worker" -a "$containerPlatformRole" != "manager" ]; then
  echo "invalid container platform role $containerPlatformRole"
  exit 1
fi

echo "init version $version, cluster $clusterName, container platform $containerPlatform, role $containerPlatformRole, azs $azs"

# create the tag for EC2's root volume
EC2_AVAIL_ZONE=`curl -s http://169.254.169.254/latest/meta-data/placement/availability-zone`
EC2_REGION="`echo \"$EC2_AVAIL_ZONE\" | sed 's/[a-z]$//'`"
EC2_INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id)
ROOT_DISK_ID=$(aws ec2 describe-volumes --filters Name=attachment.instance-id,Values=${EC2_INSTANCE_ID} --query "Volumes[*].[VolumeId]" --region=${EC2_REGION} --out text)
aws ec2 create-tags --resources ${ROOT_DISK_ID} --tags Key=Name,Value=${clusterName}-${containerPlatformRole} --region ${EC2_REGION}
if [ "$?" != "0" ]; then
  # retry tag root volume
  aws ec2 create-tags --resources ${ROOT_DISK_ID} --tags Key=Name,Value=${clusterName}-${containerPlatformRole} --region ${EC2_REGION}
  if [ "$?" != "0" ]; then
    echo "tag ec2 root volume error"
    exit 2
  fi
fi


# 1. tune the system configs.
# set madvise for THP (Transparent Huge Pages), as THP might impact the performance for some services
# such as MongoDB and Redis. Set to madvise as Cassandra may benefit from THP.
# https://www.kernel.org/doc/Documentation/vm/transhuge.txt
echo madvise | sudo tee /sys/kernel/mm/transparent_hugepage/enabled

# increase somaxconn to 512 for such as Redis
echo "net.core.somaxconn=512" >> /etc/sysctl.conf
echo "net.ipv4.tcp_max_syn_backlog=512" >> /etc/sysctl.conf

# follow https://docs.datastax.com/en/dse/5.1/dse-dev/datastax_enterprise/config/configRecommendedSettings.html
# They look general settings for other stateful services as well.
# set idle connection timeout
echo "net.ipv4.tcp_keepalive_time=60" >> /etc/sysctl.conf
echo "net.ipv4.tcp_keepalive_probes=3" >> /etc/sysctl.conf
echo "net.ipv4.tcp_keepalive_intvl=10" >> /etc/sysctl.conf
# To the handle thousands of concurrent connections used by the database
echo "net.core.rmem_max=16777216" >> /etc/sysctl.conf
echo "net.core.wmem_max=16777216" >> /etc/sysctl.conf
echo "net.core.rmem_default=16777216" >> /etc/sysctl.conf
echo "net.core.wmem_default=16777216" >> /etc/sysctl.conf
echo "net.core.optmem_max=40960" >> /etc/sysctl.conf
echo "net.ipv4.tcp_rmem=4096 87380 16777216" >> /etc/sysctl.conf
echo "net.ipv4.tcp_wmem=4096 65536 16777216" >> /etc/sysctl.conf

# increase max_map_count to 1048575 for Cassandra.
# ElasticSearch suggests to use 262144. 1048575 would be fine for ElasticSearch.
# https://docs.datastax.com/en/dse/5.1/dse-dev/datastax_enterprise/config/configRecommendedSettings.html#configRecommendedSettings__user-resource-limits
# https://www.elastic.co/guide/en/elasticsearch/reference/current/docker.html
echo "vm.max_map_count=1048575" >> /etc/sysctl.conf

# set vm.swappiness to 1 to avoid swapping for ElasticSearch. This could also benefit
# other services, as usually one node should only run one stateful service in production.
# https://en.wikipedia.org/wiki/Swappiness
echo "vm.swappiness=1" >> /etc/sysctl.conf

# set overcommit to 1 as required by Redis. Would not cause issue to other services
echo "vm.overcommit_memory=1" >> /etc/sysctl.conf

sysctl -p /etc/sysctl.conf

# 2. install docker.
yum install -y docker
if [ "$?" != "0" ]; then
  echo "install docker error"
  exit 2
fi

# 3. Create the firecamp data directory.
# The volume plugin will write the service member to the data directory, so the log plugin
# could know the target service member for the log stream.
mkdir -p /var/lib/firecamp

# Set the ulimit at docker engine.
# docker ulimit does not support unlimited. Set memlock to -1.
# https://github.com/moby/moby/issues/12515
ulimits="--default-ulimit nofile=100000:100000 --default-ulimit nproc=64000:64000 --default-ulimit memlock=-1"

# 4. Container platform specific initialization.
if [ "$containerPlatform" = "ecs" ]; then
  # Kafka uses a very large number of files, increase the file descriptor count.
  # AWS AMI sets the ulimit for docker daemon, OPTIONS=\"--default-ulimit nofile=1024:4096\".
  # The container inherits the docker daemon ulimit.
  # The docker daemon config file is different on different Linux. AWS AMI is /etc/sysconfig/docker.
  # Ubuntu is /etc/init/docker.conf, DOCKER_OPTS
  sed -i "s/OPTIONS=\"--default-ulimit.*/OPTIONS=\"$ulimits\"/g" /etc/sysconfig/docker

  service docker start
  if [ "$?" != "0" ]; then
    echo "service docker start error"
    exit 2
  fi

  sleep 3

  # follow https://github.com/aws/amazon-ecs-agent#usage to set up ecs agent
  # Set up directories the agent uses
  mkdir -p /var/log/ecs /etc/ecs /var/lib/ecs/data
  touch /etc/ecs/ecs.config

  # starting at 1.15, ecs agent requires to have the logging driver configured in ecs.config, or else ecs agent will fail.
  echo "ECS_AVAILABLE_LOGGING_DRIVERS=[\"json-file\",\"awslogs\"]" >> /etc/ecs/ecs.config
  echo "ECS_CLUSTER=$clusterName" >> /etc/ecs/ecs.config

  # Set up necessary rules to enable IAM roles for tasks
  sysctl -w net.ipv4.conf.all.route_localnet=1
  iptables -t nat -A PREROUTING -p tcp -d 169.254.170.2 --dport 80 -j DNAT --to-destination 127.0.0.1:51679
  iptables -t nat -A OUTPUT -d 169.254.170.2 -p tcp -m tcp --dport 80 -j REDIRECT --to-ports 51679

  # Run the agent
  docker run --name ecs-agent \
    --detach=true \
    --restart=always -d \
    --volume=/var/run/docker.sock:/var/run/docker.sock \
    --volume=/var/log/ecs:/log \
    --volume=/var/lib/ecs/data:/data \
    --net=host \
    --env-file=/etc/ecs/ecs.config \
    --env=ECS_LOGFILE=/log/ecs-agent.log \
    --env=ECS_DATADIR=/data/ \
    --env=ECS_ENABLE_TASK_IAM_ROLE=true \
    --env=ECS_ENABLE_TASK_IAM_ROLE_NETWORK_HOST=true \
    ${org}firecamp-amazon-ecs-agent:latest
  if [ "$?" != "0" ]; then
    # pulling the docker images fails occasionally. retry it.
    sleep 3
    docker run --name ecs-agent \
      --detach=true \
      --restart=always -d \
      --volume=/var/run/docker.sock:/var/run/docker.sock \
      --volume=/var/log/ecs:/log \
      --volume=/var/lib/ecs/data:/data \
      --net=host \
      --env-file=/etc/ecs/ecs.config \
      --env=ECS_LOGFILE=/log/ecs-agent.log \
      --env=ECS_DATADIR=/data/ \
      --env=ECS_ENABLE_TASK_IAM_ROLE=true \
      --env=ECS_ENABLE_TASK_IAM_ROLE_NETWORK_HOST=true \
      ${org}firecamp-amazon-ecs-agent:latest
    if [ "$?" != "0" ]; then
      echo "run firecamp ecs agent error"
      exit 3
    fi
  fi


  # wait for ecs agent to be ready
  sleep 6

  # install firecamp docker volume plugin
  mkdir -p /var/log/firecamp
  docker plugin install --grant-all-permissions ${org}firecamp-volume:$version
  if [ "$?" != "0" ]; then
    # volume plugin installation may fail. for example, pulling image from docker may timeout.
    # wait and retry
    sleep 10
    docker plugin install --grant-all-permissions ${org}firecamp-volume:$version
  fi

  # If ecs agent is not ready, plugin is installed but not enabled.
  # The command is taken as failed. not check the install result here.
  # The following enabled check covers the actual install failure.

  # check if volume plugin is enabled
  enabled=$(docker plugin inspect ${org}firecamp-volume:$version | grep Enabled | grep true)
  if [ "$?" != "0" ]; then
    echo "install firecamp volume plugin error, stop ecs agent"
    # stop ecs agent to avoid this instance to be included in the cluster.
    docker stop ecs-agent
    echo "stop ecs agent result $?"
    exit 3
  fi
  if [ -z "$enabled" ]; then
    # volume plugin not enabled. this may happen if ecs agent is not ready. wait some time and try to enable it again.
    sleep 10
    docker plugin enable ${org}firecamp-volume:$version
    if [ "$?" != "0" ]; then
      echo "enable firecamp volume plugin error, stop ecs agent"
      # stop ecs agent to avoid this instance to be included in the cluster.
      docker stop ecs-agent
      echo "stop ecs agent result $?"
      exit 3
    fi
  fi

  # install firecamp docker log plugin
  docker plugin install --grant-all-permissions ${org}firecamp-log:$version CLUSTER="$clusterName"
  if [ "$?" != "0" ]; then
    # wait and retry
    sleep 10
    docker plugin install --grant-all-permissions ${org}firecamp-log:$version CLUSTER="$clusterName"
    if [ "$?" != "0" ]; then
      echo "enable firecamp log plugin error, stop volume plugin and ecs agent"
      # stop firecamp volume plugin
      docker plugin disable -f ${org}firecamp-volume:$version
      echo "disable firecamp volume plugin result $?"
      # stop ecs agent to avoid this instance to be included in the cluster.
      docker stop ecs-agent
      echo "stop ecs agent result $?"
      exit 3
    fi
  fi

fi

if [ "$containerPlatform" = "swarm" ]; then
  # Set the availability zone label to engine for deploying a service to the expected zone.
  # Another option is to use swarm node label. While, the node label could only be added on the manager node.
  # Would have to talk with the manager service to add label. Sounds complex. Simply use engine label.

  # get node's local az
  localAZ=$(curl http://169.254.169.254/latest/meta-data/placement/availability-zone)

  # parse azs to array
  OIFS=$IFS
  IFS=','
  read -a zones <<< "${azs}"
  IFS=$OIFS

  # add label for 1 and 2 availability zones
  # TODO support max 3 zones now.
  labels="--label $localAZ=true"
  for zone in "${zones[@]}"
  do
    if [ "$zone" = "$localAZ" ]; then
      continue
    fi
    if [ "$zone" \< "$localAZ" ]; then
      labels+=" --label $zone.$localAZ=true"
    else
      labels+=" --label $localAZ.$zone=true"
    fi
  done

  # set ulimit and labels
  sed -i "s/OPTIONS=\"--default-ulimit.*/OPTIONS=\"$ulimits $labels\"/g" /etc/sysconfig/docker

  service docker start
  if [ "$?" != "0" ]; then
    echo "service docker start error"
    exit 2
  fi

  if [ "$version" = "latest" ]; then
    ver=$version
  else
    ver="v$version"
  fi

  # get swarminit command to init swarm
  for i in `seq 1 3`
  do
    wget -O /tmp/firecamp-swarminit.tgz https://github.com/jazzl0ver/firecamp/releases/download/$ver/firecamp-swarminit.tgz
    if [ "$?" = "0" ]; then
      break
    elif [ "$i" = "3" ]; then
      echo "failed to get https://github.com/jazzl0ver/firecamp/releases/download/$ver/firecamp-swarminit.tgz"
      exit 2
    else
      # wget fail, sleep and retry
      sleep 3
    fi
  done

  tar -zxf /tmp/firecamp-swarminit.tgz -C /tmp
  chmod +x /tmp/firecamp-swarminit

  # initialize the swarm node
  /tmp/firecamp-swarminit -cluster="$clusterName" -role="$containerPlatformRole" -availability-zones="$azs"
  if [ "$?" != "0" ]; then
    echo "firecamp-swarminit error, stop docker engine"
    service docker stop
    echo "stop docker engine result $?"
    exit 3
  fi

  if [ "$containerPlatformRole" = "worker" ]; then
    # worker node, install the firecamp plugin
    # install firecamp docker volume plugin
    mkdir -p /var/log/firecamp
    docker plugin install --grant-all-permissions ${org}firecamp-volume:$version PLATFORM="swarm" CLUSTER="$clusterName"
    if [ "$?" != "0" ]; then
      echo "install firecamp volume plugin error, stop docker engine"
      service docker stop
      echo "stop docker engine result $?"
      exit 3
    fi
  fi

  # install firecamp docker log plugin on worker and manage nodes
  docker plugin install --grant-all-permissions ${org}firecamp-log:$version CLUSTER="$clusterName"
  if [ "$?" != "0" ]; then
    # wait and retry
    sleep 10
    docker plugin install --grant-all-permissions ${org}firecamp-log:$version CLUSTER="$clusterName"
    if [ "$?" != "0" ]; then
      echo "enable firecamp log plugin error, stop docker engine"
      if [ "$containerPlatformRole" = "worker" ]; then
        echo "stop firecamp volume plugin"
        docker plugin disable -f ${org}firecamp-volume:$version
        echo "stop firecamp volume plugin result $?"
      fi
      service docker stop
      echo "stop docker engine result $?"
      exit 3
    fi
  fi
fi
