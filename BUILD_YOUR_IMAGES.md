The default 'make docker' builds cloudstax/firecamp-xxx images and pushes to cloudstax docker hub. If you make some change and want to test on your own branch, you will need to create docker images with different names and push to your account on docker hub. You could simply create a docker hub account to host the public images, https://hub.docker.com/. The docker hub is free for hosting the public images.

To build your own images, you need to change the org name to your docker account. For example, your docker account name is "mydockeraccount". Do below changes.

1. Build your own ecs agent docker image.

Checkout cloudstax amazon-ecs-agent branch, git clone https://github.com/cloudstax/amazon-ecs-agent.git, and change the "org" in Makefile and agent/engine/firecamp_task_engine.go to "mydockeraccount/". Then simply 'make' to build and upload the docker image.

2. Build the docker images for FireCamp manage service, plugins and stateful services.

Update the "org" in Makefile and "OrgName" in api/common/types.go to "mydockeraccount/". Then 'make docker' to build and upload all docker images.

3. Create your own bucket for the CloudFormation templates.

Firstly, update the packaging/aws-cloudformation/init.sh to use your docker images. Update the "org" in init.sh to "mydockeraccount/". Modify ContainerDefinitions in firecamp-ecs.template to match your docker account name. Currently the swarm init binary is put in the cloudstax s3 bucket. Please change to your bucket as well.

Then you could upload the templates to your bucket. For example, your bucket name is "mybucket".
- Upload Linux Bastion template: upload https://github.com/aws-quickstart/quickstart-linux-bastion/scripts to mybucket/linux/bastion/latest/scripts, and https://github.com/aws-quickstart/quickstart-linux-bastion/templates to mybucket/linux/bastion/latest/templates.
- Upload AWS VPC templates: upload https://github.com/aws-quickstart/quickstart-aws-vpc/templates to mybucket/aws/vpc/latest/templates.
- Upload FireCamp templates, packaging/aws-cloudformation/firecamp-*.template, to mybucket/firecamp/releases/latest/templates/firecamp-*.template. Upload the init script, packaging/aws-cloudformation/init.sh, to mybucket/firecamp/releases/latest/scripts/init.sh. And upload $GOPATH/bin/firecamp-service-cli.tgz and $GOPATH/bin/firecamp-swarminit.tgz to mybucket/firecamp/releases/latest/packages/firecamp-*.tgz.

Then you could use mybucket/firecamp/releases/latest/templates/firecamp-master.template or firecamp.template to deploy the cluster and test the stateful service. You will need to set "QSS3BucketName" to "mybucket".
