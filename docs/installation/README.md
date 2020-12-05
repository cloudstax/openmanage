* [Installation on AWS](https://github.com/cloudstax/firecamp/tree/master/docs/installation#installation-on-aws)
  * [Install the FireCamp Cluster](https://github.com/cloudstax/firecamp/tree/master/docs/installation#install-the-firecamp-cluster)
  * [Install the Application Cluster](https://github.com/cloudstax/firecamp/tree/master/docs/installation#install-the-application-cluster)
  * [Delete the FireCamp Cluster](https://github.com/cloudstax/firecamp/tree/master/docs/installation#delete-the-firecamp-cluster)
* [The FireCamp Service CLI](https://github.com/cloudstax/firecamp/tree/master/docs/installation#the-firecamp-service-cli)
  * [Create the Stateful Service](https://github.com/cloudstax/firecamp/tree/master/docs/installation#create-the-stateful-service)
  * [Delete the Stateful Service](https://github.com/cloudstax/firecamp/tree/master/docs/installation#delete-the-stateful-service)

# Installation on AWS

## Install the FireCamp Cluster
The FireCamp cluster can be easily installed via [AWS FireCamp QuickStart](https://aws.amazon.com/quickstart/architecture/cloudstax-firecamp/). It will create an ECS or Docker Swarm cluster across 3 AvailabilityZones. So the stateful services such as MongoDB could have 3 replicas on 3 AvailabilityZones, to tolerate the single availability zone failure.

AWS FireCamp QuickStart does not install the latest developing build. If you want to test/develop the latest build, please use the "cloudstax" bucket or create your own bucket. The "cloudstax" bucket has the [master template](https://s3.amazonaws.com/cloudstax/firecamp/releases/latest/templates/firecamp-master.template) and [existingvpc template](https://s3.amazonaws.com/cloudstax/firecamp/releases/latest/templates/firecamp.template). If you use your own bucket, please remember to update the "QSS3BucketName" and "QSS3KeyPrefix" accordingly.

1. Go to [AWS FireCamp QuickStart](https://aws.amazon.com/quickstart/architecture/cloudstax-firecamp/), click "Deploy on AWS into a new VPC", then click "Next".

2. "Specify Details": specify below fields, then click "Next".
* Specify the "Stack name", such as "t1".

  ![](https://s3.amazonaws.com/cloudstax/firecamp/docs/install/cfstack+StackName.png)

* Select 3 "AvailabilityZones".

![](https://s3.amazonaws.com/cloudstax/firecamp/docs/install/cfstack+AvailabilityZones.png)

* Specify the "ClusterName", such as "t1".

![](https://s3.amazonaws.com/cloudstax/firecamp/docs/install/cf+clustername.png)

* Select the "ContainerPlatform", "ecs" or "swarm".

![](https://s3.amazonaws.com/cloudstax/firecamp/docs/install/cf+platform.png)

* Select the "KeyPairName" for SSH to the Bastion and cluster nodes.

  ![](https://s3.amazonaws.com/cloudstax/firecamp/docs/install/cf+stack+KeyPairName.png)

* Select the cluster node Instance Type. For the simple function tests, you may select t2.micro type.

  ![](https://s3.amazonaws.com/cloudstax/firecamp/docs/install/cfstack+NodeInstanceType.png)

* Modify the "RemoteAccessCIDR" that could SSH to the Bastion nodes.

  ![](https://s3.amazonaws.com/cloudstax/firecamp/docs/install/cfstack+RemoteAccessCIDR.png)

3. "Options": click "Next".
4. "Review": check the acknowledge for IAM, and click "Create".

![](https://s3.amazonaws.com/cloudstax/firecamp/docs/install/cfstack+AckIAM.png)


You could also use AWS CLI to create a cluster:
1. Create the parameters json file, such as stack-master.json. This json config file is for AWS region us-east-1. To create a Docker Swarm cluster, simply change the "ContainerPlatform" from "ecs" to "swarm".
Currently the template supports 2 and 3 AZs. It is recommended to use 3 AZs for the production environment.
```
[
  {
    "ParameterKey": "ClusterName",
    "ParameterValue": "t1"
  },
  {
    "ParameterKey": "AvailabilityZones",
    "ParameterValue": "us-east-1a,us-east-1b,us-east-1c"
  },
  {
    "ParameterKey": "ContainerPlatform",
    "ParameterValue": "ecs"
  },
  {
    "ParameterKey": "KeyPairName",
    "ParameterValue": "mykeyfile"
  },
  {
    "ParameterKey": "RemoteAccessCIDR",
    "ParameterValue": "0.0.0.0/0"
  }
]
```

2. Run AWS CLI to create the cluster.
```
#!/bin/sh

aws cloudformation create-stack --stack-name t1 --disable-rollback --capabilities CAPABILITY_IAM --template-url path_to_your_s3_bucket/firecamp/latest/templates/firecamp-master.template --parameters file://stack-master.json
```

## Install the Application Cluster
The application could be run on a separate cluster or the FireCamp cluster. The nodes in the FireCamp Cluster could access the stateful services. The CloudFormation template also creates the AppAccessSecurityGroup to protect the outside access to the stateful services running on the FireCamp cluster. You could create the application nodes on the AppAccessSecurityGroup and the same VPC with the FireCamp cluster.

The AppAccessSecurityGroup and VPCID could be found in the CloudFormation Stack output.

![](https://s3.amazonaws.com/cloudstax/firecamp/docs/install/cf+outputs.png)

## Delete the FireCamp Cluster
Use firecamp-service-cli to delete all stateful services created on the cluster. **If any stateful service is not deleted, the cloudformation stack deletion will fail.** There is no need to delete the manage service, CloudFormation will automatically clean up it at deletion.

After all stateful services are deleted, delete the CloudFormation stack, which will delete all resources created by the stack.

# The FireCamp Service CLI
A Bastion AutoScaleGroup is created and is the only one that could SSH to the cluster nodes, and access the FireCamp manage server. The nodes in the FireCamp Cluster could also access the manage server.

After the stack is created, could ssh to the Bastion node, get the FireCamp service cli of the current release such as 0.9.x, `wget https://github.com/jazzl0ver/firecamp/releases/download/latest/firecamp-service-cli.tgz`

## Create the Stateful Service
The MongoDB or PostgreSQL cluster could be simply created using the firecamp-service-cli. In case the service creation command fails, could simply retry it. **It is recommended that you should change the root user's password after the service is created.**

For example, the cluster "t1" is created at region us-east-1. To create a 3 replias MongoDB cluster on 3 availability zones, with 100GB data volume and 10GB journal volume for each replica. The default DB user/password is dbadmin/changeme. Run:
```
firecamp-service-cli -op=create-service -service-type=mongodb -region=us-east-1 -cluster=t1 -service-name=mymongo -mongo-shards=1 -mongo-replicas-pershard=3 -mongo-replicaset-only=true -volume-size=100 -journal-volume-size=10 -admin=admin -password=changeme
```

To create a 3 nodes PostgreSQL cluster on 3 availability zones, with 100GB data volume and 10GB journal volume for each replica. The default DB user/password is postgres/changeme, and the PostgreSQL replication user/password is repluser/replpassword. Run:
```
firecamp-service-cli -op=create-service -service-type=postgresql -region=us-east-1 -cluster=t1 -service-name=mypg -replicas=3 -journal-volume-size=10 -volume-size=100 -passwd=changeme -replication-user=repluser -replication-passwd=replpassword
```

To encrypt the data and/or journal volumes add **-volume-encrypted=true** and/or **-journal-volume-encrypted=true** to the **firecamp-service-cli** command line. It works in AWS environments only.

For more details, please refer to each service's tutorials in the service's readme, such as [MongoDB](https://github.com/cloudstax/firecamp/tree/master/catalog/mongodb).

The general service creation steps:
1. Create the Volumes and persist the metadata to the FireCamp DB. This is usually very fast. But if AWS is slow at creating the Volume, this step will be slow as well.
2. Create the service on the container orchestration framework, such as AWS ECS, and wait for all service containers running. The speed depends on how fast the container orchestration framework could start all containers.
3. Wait some time (10 seconds) for the service containers to stabilize.
4. If the service, such as MongoDB, requires the additional initialization work, start the service initialization task. The speed also depends on how fast the container orchestration framework schedules the task.
5. The initialization task will initialize the service. For example, the MongoDB initialization task does:
  * Initialize the MongoDB replication.
  * Wait some time (10 seconds) for MongoDB to stabilize.
  * Create the admin user.
  * Notify the manage server to do the post initialization work.
  * The manage server will do the post initialization works: Update the mongod.conf of every replica to enable MongoDB Authentication and restart all MongoDB containers.

In case the service creation fails, please simply retry it.


## Delete the Stateful Service
The service could be deleted by one command. For example, to delete the MongoDB or PostgreSQL cluster, run:
```
firecamp-service-cli -op=delete-service -service-type=mongodb -region=us-east-1 -cluster=t1 -service-name=mymongo
```
```
firecamp-service-cli -op=delete-service -service-type=postgresql -region=us-east-1 -cluster=t1 -service-name=mypg
```

**The service deletion will NOT delete the service volumes. Instead, it returns the volumes. You would need to delete the volumes by yourself.** This is to avoid the potential service deletion by mistake.

The service deletion is usually fast. While, occassionally, some AWS operations such as detaching the EBS volume, deleting Route53 record, may take around 100 seconds.
