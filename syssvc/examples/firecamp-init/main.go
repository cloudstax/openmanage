package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/cloudstax/firecamp/common"
	"github.com/cloudstax/firecamp/containersvc"
	"github.com/cloudstax/firecamp/containersvc/awsecs"
	"github.com/cloudstax/firecamp/log"
	"github.com/cloudstax/firecamp/log/awscloudwatch"
	"github.com/cloudstax/firecamp/server/awsec2"
)

var (
	cluster = flag.String("cluster", "default", "The ECS cluster")
	region  = flag.String("region", "", "The AWS region")
	dbtype  = flag.String("dbtype", common.DBTypeCloudDB, "The db type, such as the AWS DynamoDB or the embedded controldb")
)

func main() {
	flag.Parse()

	var err error

	// check region and az
	awsRegion := *region
	if awsRegion == "" {
		awsRegion, err = awsec2.GetLocalEc2Region()
		if err != nil {
			fmt.Println("please input the correct AWS region")
			os.Exit(-1)
		}
	}

	cfg := aws.NewConfig().WithRegion(awsRegion)
	sess, err := session.NewSession(cfg)
	if err != nil {
		glog.Fatalln("failed to create session, error", err)
	}

	ecsIns := awsecs.NewAWSEcs(sess)
	logIns := awscloudwatch.NewLog(sess, awsRegion, common.ContainerPlatformECS)

	// create context
	ctx := context.Background()

	// cluster should exist
	exist, err := ecsIns.IsClusterExist(ctx, *cluster)
	if err != nil {
		glog.Fatalln("IsClusterExist error", err, *cluster)
	}
	if !exist {
		glog.Fatalln("cluster not exist", *cluster)
	}

	// check if the manage service exists
	exist, err = ecsIns.IsServiceExist(ctx, *cluster, common.ManageServiceName)
	if err != nil {
		glog.Fatalln("check the manage http service exist error", err, common.ManageServiceName)
	}
	if exist {
		fmt.Println("The manage service is already created")
		return
	}

	// create the manage service
	logConfig := logIns.CreateLogConfigForStream(ctx, *cluster, common.ManageServiceName, "", common.ManageServiceName)
	createOpts := genCreateOptions(awsRegion, *cluster, *dbtype, logConfig)
	err = ecsIns.CreateService(ctx, createOpts)
	if err != nil {
		glog.Fatalln("create the manage service error", err, common.ManageServiceName)
	}
	fmt.Println("created the manage service")

	// wait the manage service ready
	maxWaitSeconds := int64(120)
	err = ecsIns.WaitServiceRunning(ctx, *cluster, common.ManageServiceName, 1, maxWaitSeconds)
	if err != nil {
		glog.Fatalln("wait the manage container running error", err, common.ManageServiceName)
	}
	fmt.Println("The manage service is ready")
}

func genCreateOptions(region string, cluster string, dbtype string, logConfig *cloudlog.LogConfig) *containersvc.CreateServiceOptions {
	// create the env variables for the manage service container
	kv1 := &common.EnvKeyValuePair{
		Name:  common.ENV_CONTAINER_PLATFORM,
		Value: common.ContainerPlatformECS,
	}
	kv2 := &common.EnvKeyValuePair{
		Name:  common.ENV_DB_TYPE,
		Value: dbtype,
	}
	envkvs := []*common.EnvKeyValuePair{kv1, kv2}

	// create the management http server service
	commonOpts := &containersvc.CommonOptions{
		Cluster:        cluster,
		ServiceName:    common.ManageServiceName,
		ServiceUUID:    "",
		ContainerImage: common.ManageContainerImage,
		Resource: &common.Resources{
			MaxCPUUnits:     common.DefaultMaxCPUUnits,
			ReserveCPUUnits: common.ManageReserveCPUUnits,
			MaxMemMB:        common.ManageMaxMemMB,
			ReserveMemMB:    common.ManageReserveMemMB,
		},
		LogConfig: logConfig,
	}

	p := common.PortMapping{
		ContainerPort: common.ManageHTTPServerPort,
		HostPort:      common.ManageHTTPServerPort,
	}

	return &containersvc.CreateServiceOptions{
		Common:        commonOpts,
		ContainerPath: "",
		PortMappings:  []common.PortMapping{p},
		Replicas:      int64(1),
		Envkvs:        envkvs,
	}
}