
To build a new release, a few steps are required. This will be enhanced in the future.

1. Update FireCamp software release version
1) Change the version in Makefile from "latest" to the release version such as "1.0".
2) Chagne the version in common/types.go from "latest" to the release version such as "1.0".
3) Update the "Release" to such as "1.0" and "QSS3KeyPrefix" to "firecamp/releases/1.0" in firecamp-master.template and firecamp.template

Update the README.md and docs/installation/README.md to the new version as well.

2. Upload the new files
Create the new release folder in the "cloudstax" bucket, such as firecamp/releases/1.0. Create subfolders: "templates", "scripts", "packages". Upload all templates under packages/aws-cloudformation/ to "templates", upload packages/aws-cloudformation/init.sh to "scripts", and upload $GOPATH/bin/firecamp-service-cli.tgz and $GOPATH/bin/firecamp-swarminit.tgz to "packages". The swarminit command is used by packaging/aws-cloudformation/init.sh to initialize the Docker Swarm cluster.

Note: the default template allows '.' in the QSS3KeyPrefix. AWS QuickStart always points to the latest version, such as aws/vpc/latest for QSAWSVPCS3KeyPrefix, and '.' is not allowed in the key prefix. For the new release, when updating the AWS QuickStart git, remove '.' in the QSS3KeyPrefix and point to the latest version in QuickStart.
Also change "QSS3BucketName" from "cloudstax" to "quickstart-reference".


3. Docker Images
make docker to build the docker images for this release, and upload to docker hub. If Amazon ECS agent git creates a new version, update cloudstax/amazon-ecs-agent, build a new container and push to docker hub.


4. The plugin log file rotates
If vendor is updated, check to make sure the glog log file size is adjusted.

glog rotates the log file at MaxSize, glog_file.go, which is 1024*1024*1800 by default. Change to 64MB. So we could keep 20 log files for troubleshooting.

Note: the docker images of the stateful services are tagged with the service's version, such as 3.4 for MongoDB. If the docker image is changed in the master branch, for example, the init task script is updated, change the service version to "latest" in both service catalog go file such as catalog/mongodb/mongodbcatalog.go and scripts/builddocker.sh. After test on the latest branch passes, test to make sure it does not break the previous releases, as the previous releases points to the same service version such as MongoDB 3.4.
