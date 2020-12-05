#!/bin/bash
set -e

rootdir=/data
datadir=/data/kafka
confdir=/data/conf

# after release 0.9.5, sys.conf and java.env files are not created any more.
# instead service.conf and emember.conf are created for the common service configs
# and the service member configs. The default mongod.conf will be updated with the configs
# in service.conf and member.conf
syscfgfile=$confdir/sys.conf
javaenvfile=$confdir/java.env

serverpropfile=$confdir/server.properties
logcfgfile=$confdir/log4j.properties
servicecfgfile=$confdir/service.conf
membercfgfile=$confdir/member.conf

kafkacfgdir=/kafka/config
kafkaserverpropfile=$kafkacfgdir/server.properties

# sanity check to make sure the volume is mounted to /data.
if [ ! -d "$rootdir" ]; then
  echo "error: $rootdir not exist. Please make sure the volume is mounted to $rootdir." >&2
  exit 1
fi
if [ ! -d "$datadir" ]; then
  mkdir "$datadir"
fi
if [ ! -d "$confdir" ]; then
  echo "error: $confdir not exist." >&2
  exit 1
fi
# sanity check to make sure the config files are created.
if [ ! -f "$serverpropfile" ]; then
  echo "error: $serverpropfile not exist." >&2
  exit 1
fi
if [ ! -f "$logcfgfile" ]; then
  echo "error: $logcfgfile not exist." >&2
  exit 1
fi

# allow the container to be started with `--user`
if [ "$1" = 'kafka-server-start.sh' -a "$(id -u)" = '0' ]; then
  rootdiruser=$(stat -c "%U" $rootdir)
  if [ "$rootdiruser" != "$KAFKA_USER" ]; then
    echo "chown -R $KAFKA_USER $rootdir"
    chown -R "$KAFKA_USER" "$rootdir"
  fi
  diruser=$(stat -c "%U" $datadir)
  if [ "$diruser" != "$KAFKA_USER" ]; then
    chown -R "$KAFKA_USER" "$datadir"
  fi
  chown -R "$KAFKA_USER" "$confdir"

  exec su-exec "$KAFKA_USER" "$BASH_SOURCE" "$@"
fi

# copy config files to /kafka/config
cp $serverpropfile $kafkacfgdir
cp $logcfgfile $kafkacfgdir

# after release 0.9.5
if [ -f $servicecfgfile ]; then
  # load service and member configs
  . $servicecfgfile
  . $membercfgfile

  # update server.properties file
  sed -i 's/broker.id=0/broker.id='$SERVICE_MEMBER_INDEX'/g' $kafkaserverpropfile
  sed -i 's/broker.rack=rack/broker.rack='$AVAILABILITY_ZONE'/g' $kafkaserverpropfile
  sed -i 's/delete.topic.enable=true/delete.topic.enable='$ALLOW_TOPIC_DEL'/g' $kafkaserverpropfile
  sed -i 's/num.partitions=8/num.partitions='$NUMBER_PARTITIONS'/g' $kafkaserverpropfile
  sed -i 's/bindip/'$BIND_IP'/g' $kafkaserverpropfile
  sed -i 's/advertisedip/'$SERVICE_MEMBER'/g' $kafkaserverpropfile
  sed -i 's/replication.factor=3/replication.factor='$REPLICATION_FACTOR'/g' $kafkaserverpropfile
  sed -i 's/log.min.isr=2/log.min.isr='$MIN_INSYNC_REPLICAS'/g' $kafkaserverpropfile
  sed -i 's/min.insync.replicas=2/min.insync.replicas='$MIN_INSYNC_REPLICAS'/g' $kafkaserverpropfile
  sed -i 's/retention.hours=168/retention.hours='$RETENTION_HOURS'/g' $kafkaserverpropfile

  # create jmx remote password and access files
  echo "$JMX_REMOTE_USER $JMX_REMOTE_PASSWD" > $kafkacfgdir/jmxremote.password
  echo "$JMX_REMOTE_USER $JMX_REMOTE_ACCESS" > $kafkacfgdir/jmxremote.access
  chmod 0400 $kafkacfgdir/jmxremote.password
  chmod 0400 $kafkacfgdir/jmxremote.access

  # set java options
  export KAFKA_HEAP_OPTS="-Xmx${HEAP_SIZE_MB}m -Xms${HEAP_SIZE_MB}m"
  export KAFKA_JVM_PERFORMANCE_OPTS="-server -XX:+UseG1GC -XX:MaxGCPauseMillis=20 -XX:InitiatingHeapOccupancyPercent=35 -XX:+DisableExplicitGC -Djava.awt.headless=true -XX:G1HeapRegionSize=16M -XX:MetaspaceSize=96m -XX:MinMetaspaceFreeRatio=50 -XX:MaxMetaspaceFreeRatio=80"
  export KAFKA_JMX_OPTS="-Djava.rmi.server.hostname=$SERVICE_MEMBER -Dcom.sun.management.jmxremote.rmi.port=$JMX_PORT -Dcom.sun.management.jmxremote=true -Dcom.sun.management.jmxremote.password.file=$kafkacfgdir/jmxremote.password -Dcom.sun.management.jmxremote.access.file=$kafkacfgdir/jmxremote.access -Dcom.sun.management.jmxremote.authenticate=true -Dcom.sun.management.jmxremote.ssl=false"

else
  # load the sys config file. the syscfgfile exists before 0.9.6
  . $syscfgfile
  # load the java env file
  . $javaenvfile
fi

echo $SERVICE_MEMBER
echo ""

echo $KAFKA_HEAP_OPTS
echo $KAFKA_JVM_PERFORMANCE_OPTS
export KAFKA_HEAP_OPTS=$KAFKA_HEAP_OPTS
export KAFKA_JVM_PERFORMANCE_OPTS=$KAFKA_JVM_PERFORMANCE_OPTS
export JMX_PORT=$JMX_PORT

echo "$@"
exec "$@"
