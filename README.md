# [FireCamp](https://github.com/cloudstax/firecamp)

**CloudStax is shutting down. As a result, work on FireCamp will end and the Github repository will be archived.**

FireCamp is an open source platform to easily setup, manage and scale the stateful services. The customer could easily run any service with no management overhead on any cloud.

The FireCamp platform is one big step towards the free Serverless Cloud. FireCamp decouples the Function-as-a-Service (FaaS) engine from the Vendor’s services, such as DynamoDB. The single function could then run on any FaaS engine, and simply talk to the same stateful services, such as Cassandra, on any cloud. It makes it easy to develop and test the functions locally with the stateful service, and run in any cloud at scale with the same stateful service.

FireCamp Dockerizes the stateful services and run them on top of the popular container orchestration frameworks, such as [Amazon EC2 Container Service](https://aws.amazon.com/ecs/), [Docker Swarm](https://docs.docker.com/engine/swarm/), [Mesos](http://mesos.apache.org/) and [Kubernetes](https://kubernetes.io). FireCamp deeply integrates with the popular open source stateful services, such as MongoDB, Cassandra, Kafka, PostgreSQL, MySQL, ZooKeeper, Redis, ElasticSearch, etc. FireCamp manages the service members, such as MongoDB replicas, and their data volumes together. When the container of a service member moves to a different node, FireCamp will maintain the membership and move the data volume as well.

With FireCamp, you are able to quickly and efficiently respond to the stateful service demands:
* **Deploy a stateful service quickly.**
* **Handle the failure automatically.**
* **Scale the stateful service on the fly.**
* **Upgrade the stateful service smoothly.**
* **Manage the stateful service data easily.**
* **Move from one cloud to another freely.**

## Key Features

**Cross Container Orchestration Frameworks**: The FireCamp platform is built on top of the popular container orchestration frameworks. Currently FireCamp supports [Amazon EC2 Container Service](https://aws.amazon.com/ecs/) and [Docker Swarm](https://docs.docker.com/engine/swarm/) on Amazon Web Services. Other container orchestration frameworks ([Mesos](http://mesos.apache.org/) and [Kubernetes](https://kubernetes.io)) will be supported in the future.

**Cross Clouds**: Currently support Amazon Web Services. Will support Google Cloud, Microsoft Azure Cloud, Private and Hybrid Cloud in the future.

**One Click Deploy**: FireCamp's Catalog service integrates the stateful services. The customer could simply call a single command to deploy a stateful service, such as a MongoDB ReplicaSet.

**Fast and Auto Failover**: FireCamp binds the data volumes and membership with the container. When a node crashes, the container orchestration frameworks will reschedule the container to another node. FireCamp will automatically move the data volume and the membership. There is no data copy involved during the failover.

**Security**: FireCamp will enforce the security at both the platform level and the service level.
* The AppAccessSecurityGroup: FireCamp creates the AppAccessSecurityGroup to restricts the access to the stateful services. Only the EC2 instances in the AppAccessSecurityGroup could access the stateful services. The customer should have the application running on the EC2 of the AppAccessSecurityGroup and the same VPC.
* The Bastion node: the Bastion AutoScaleGroup is created and is the only one that could SSH to the FireCamp cluster nodes and talk with the FireCamp manage service.
* Service security: user and password are required to access the service, such as MongoDB. And other service internal security features are enabled, such as MongoDB access control between members of a ReplicaSet.

**Flexible Custom Plugins**: FireCamp could be easily expanded to support the custom plugins, such as PostGIS for PostgreSQL, CouchDB input plugin for Logstash, etc. The custom container image could be created from the base image, such as [PostGIS](https://github.com/cloudstax/firecamp/tree/master/catalog/postgres/9.6/postgis-dockerfile/Dockerfile), and specify the image when creating the service.

**Easy Scale**: FireCamp makes it easy to scale out the stateful services. For the primary based service, such as MongoDB, FireCamp integrates with Cloud Volume Snapshot to simplify adding a new replica. For example, to add a new replica to an existing MongoDB ReplicaSet, FireCamp will pause one current replica, create a Snapshot of the replica's Cloud Volume, create a new Volume from the Snapshot, and assign the new volume to the new replica.

**Smooth Upgrade**: FireCamp is aware of the service's membership, and follows the service's rule to maintain the membership. For example, when upgrade a MongoDB ReplicaSet, it is better to gracefully stop and upgrade the primary member first. FireCamp will automatically detect who is the primary member, upgrade it first, and then upgrade other members one by one.

**Easy Data Management**: FireCamp provides the policy based data management. The customer could easily setup the policy to manage the data Snapshot, Backup, Restore, etc.

**GEO Access and Protection**: FireCamp could manage the service membership across multiple Regions, and support backup data to the remote region.

**Service Aware Monitoring**: No matter where the container of one service member moves and how many times it move, FireCamp keeps tracking a member's logs and metrics for the member. FireCamp will also track the containers of different services running on the same host, and correlate them for the troubleshooting.

## How does it work?

Basically, the FireCamp platform maintains the service membership and data volume for the service. When the container moves from one node to another node, FireCamp could recognize the member of the new container and mount the original data volume. For more details, please refer to [Architecture](https://github.com/cloudstax/firecamp/tree/master/docs/architect) and [Work Flow](https://github.com/cloudstax/firecamp/tree/master/docs/workflows).

## Installation
The FireCamp cluster could be easily installed using AWS CloudFormation for AWS ECS and Docker Swarm.
1. [Create FireCamp cluster via AWS QuickStart](https://aws.amazon.com/quickstart/architecture/cloudstax-firecamp/). This will create an ECS or Docker Swarm cluster across 3 availability zones. The cluster only has the private network address and not accessible from the external internet. The Bastion node is also created and is the only node that could SSH to the cluster.
2. SSH to the Bastion node and wget firecamp cli of the installed release. For example, to get the cli of release 1.2, `wget https://s3.amazonaws.com/cloudstax/firecamp/releases/1.2/packages/firecamp-service-cli.tgz`.
3. Use the firecamp cli to create the stateful service. Refer to each catalog service for the detail tutorials of the service, such as [MongoDB](https://github.com/cloudstax/firecamp/tree/master/catalog/mongodb#tutorials).

For the Installation details, please refer to [Installation](https://github.com/cloudstax/firecamp/tree/master/docs/installation).

## How do applications access the service?

The applications should run in the same VPC and the AppAccessSecurityGroup, and access the stateful services via the DNS names.

Every service member will get a unique dns name. For example, the cluster is testcluster, Cassandra service name is mycas with 3 replicas. FireCamp will assign the dns name, mycas-0.testcluster-firecamp.com, mycas-1.testcluster-firecamp.com, and mycas-2.testcluster-firecamp.com, to the 3 Cassandra containers. The applications could simply access the Cassandra cluster via these 3 dns names.

When the EC2 instance goes down, let's say the EC2 of mycas-1.testcluster-firecamp.com, AutoScaleGroup will start a new EC2. ECS will schedule the Cassandra container to the new EC2. FireCamp will attach the original EBS volumes that belongs to mycas-1.testcluster-firecamp.com, and update the address of mycas-1.testcluster-firecamp.com to the new EC2's privateIP in Route53.

If the applications use JVM, you will need to set JVM TTL. Please check such as [Cassandra Readme](https://github.com/cloudstax/firecamp/tree/master/catalog/cassandra): "By default, JVM caches a successful DNS lookup forever. If you use Cassandra Java CQL driver, please set JVM TTL to a reasonable value such as 60 seconds. So when Cassandra container moves to another node, Java CQL driver could lookup the new address."

## Catalog Services
* [MongoDB](https://github.com/cloudstax/firecamp/tree/master/catalog/mongodb)
* [PostgreSQL](https://github.com/cloudstax/firecamp/tree/master/catalog/postgres)
* [Cassandra](https://github.com/cloudstax/firecamp/tree/master/catalog/cassandra)
* [ZooKeeper](https://github.com/cloudstax/firecamp/tree/master/catalog/zookeeper)
* [Kafka](https://github.com/cloudstax/firecamp/tree/master/catalog/kafka)
* [KafkaConnect](https://github.com/cloudstax/firecamp/tree/master/catalog/kafkaconnect)
* [KafkaManager](https://github.com/cloudstax/firecamp/tree/master/catalog/kafkamanager)
* [Redis](https://github.com/cloudstax/firecamp/tree/master/catalog/redis)
* [CouchDB](https://github.com/cloudstax/firecamp/tree/master/catalog/couchdb)
* [Consul](https://github.com/cloudstax/firecamp/tree/master/catalog/consul)
* [ElasticSearch](https://github.com/cloudstax/firecamp/tree/master/catalog/elasticsearch)
* [Kibana](https://github.com/cloudstax/firecamp/tree/master/catalog/kibana)
* [Logstash](https://github.com/cloudstax/firecamp/tree/master/catalog/logstash)
* [Telegraf](https://github.com/cloudstax/firecamp/tree/master/catalog/telegraf)
* MySQL: coming soon.

## Questions and Issues
If you have any questions or feedbacks, please feel free to check / ask on [Google Groups](https://groups.google.com/forum/#!forum/firecamp-users), or [Stackoverflow](https://stackoverflow.com/questions/tagged/firecamp)

If you hit an issue or bug, or want to propose some features or improvements, please feel free to create an issue on github. When creating the issue or bug, please help to include the detail information so we could troubleshoot and fix as soon as possible.
-   *Reproduce Steps.* Include the detail steps to reproduce the problem.
-   *Specific.* Include as much detail as possible: which version, what environment, etc.
-   *Unique.* Please do not duplicate existing tickets.
-   *Scoped to a Single Bug.* One bug per report.
