package kmcatalog

import (
	"errors"
	"fmt"

	"github.com/cloudstax/firecamp/api/catalog"
	"github.com/cloudstax/firecamp/api/common"
	"github.com/cloudstax/firecamp/api/manage"
)

const (
	defaultVersion = "3.0.0"
	// ContainerImage is the main running container.
	ContainerImage        = common.ContainerNamePrefix + "kafka-manager:" + defaultVersion
	ReleaseContainerImage = ContainerImage + common.NameSeparator + common.Version

	listenPort = 9000

	// DefaultHeapMB is the default kafka manager java heap size.
	// TODO what is the best heap size for kafka manager?
	DefaultHeapMB = 4096

	ENV_ZKHOSTS     = "ZK_HOSTS"
	ENV_JAVA_OPTS   = "JAVA_OPTS"
	ENV_KM_USERNAME = "KM_USERNAME"
	ENV_KM_PASSWORD = "KM_PASSWORD"
)

// The Yahoo Kafka Manager catalog service, https://github.com/yahoo/kafka-manager
// Kafka Manager only needs 1 instance running.
// Kafka Manager will store the data in ZooKeeper. So Kafka Manager itself is stateless.

// ValidateRequest checks if the request is valid
func ValidateRequest(opts *catalog.CatalogKafkaManagerOptions) error {
	if len(opts.User) == 0 || len(opts.Password) == 0 {
		return errors.New("Please specify the user and password")
	}
	if opts.HeapSizeMB <= 0 {
		return errors.New("The heap size should be larger than 0")
	}
	if len(opts.ZkServiceName) == 0 {
		return errors.New("The zookeeper service name could not be empty")
	}

	return nil
}

// GenDefaultCreateServiceRequest returns the default service creation request.
func GenDefaultCreateServiceRequest(platform string, region string, cluster string, service string,
	zkServers string, opts *catalog.CatalogKafkaManagerOptions, res *common.Resources) *manage.CreateServiceRequest {

	envkvs := []*common.EnvKeyValuePair{
		&common.EnvKeyValuePair{Name: common.ENV_CLUSTER, Value: cluster},
		&common.EnvKeyValuePair{Name: common.ENV_SERVICE_NAME, Value: service},
		&common.EnvKeyValuePair{Name: common.ENV_CONTAINER_PLATFORM, Value: platform},
		&common.EnvKeyValuePair{Name: ENV_ZKHOSTS, Value: zkServers},
		&common.EnvKeyValuePair{Name: ENV_JAVA_OPTS, Value: fmt.Sprintf("-Xms%dm -Xmx%dm", opts.HeapSizeMB, opts.HeapSizeMB)},
		// TODO it is not best to put user & password directly in container environment.
		// consider to add a record to DB and fetch from DB?
		&common.EnvKeyValuePair{Name: ENV_KM_USERNAME, Value: opts.User},
		&common.EnvKeyValuePair{Name: ENV_KM_PASSWORD, Value: opts.Password},
	}

	portMappings := []common.PortMapping{
		{ContainerPort: listenPort, HostPort: listenPort, IsServicePort: true},
	}

	reserveMemMB := res.ReserveMemMB
	if res.ReserveMemMB < opts.HeapSizeMB {
		reserveMemMB = opts.HeapSizeMB
	}

	serviceCfgs := genServiceConfigs(platform, opts)

	replicaCfgs := catalog.GenStatelessServiceReplicaConfigs(cluster, service, 1)

	// kafkamanager docker-entrypoint script includes the firecamp-selectmember tool to select
	// the member and update dns. the firecamp-selectmember tool is different for different releases.
	// Every release has to use its own docker image.
	// TODO this is not good. we should consider to drop Docker Swarm. For AWS ECS, could use the
	// 			init container to do the work.
	// Note: scripts/builddocker.sh also builds the kafka manager image with the release version,
	// 			 the release upgrade needs to update the version as well.
	containerImage := ContainerImage
	if platform != common.ContainerPlatformK8s {
		containerImage = ReleaseContainerImage
	}

	req := &manage.CreateServiceRequest{
		Service: &manage.ServiceCommonRequest{
			Region:             region,
			Cluster:            cluster,
			ServiceName:        service,
			CatalogServiceType: common.CatalogService_KafkaManager,
		},

		Resource: &common.Resources{
			MaxCPUUnits:     res.MaxCPUUnits,
			ReserveCPUUnits: res.ReserveCPUUnits,
			MaxMemMB:        res.MaxMemMB,
			ReserveMemMB:    reserveMemMB,
		},

		ServiceType: common.ServiceTypeStateless,

		ContainerImage: containerImage,
		// Kafka Manager only needs 1 container.
		Replicas:       1,
		PortMappings:   portMappings,
		Envkvs:         envkvs,
		RegisterDNS:    true,
		ServiceConfigs: serviceCfgs,
		ReplicaConfigs: replicaCfgs,
	}
	return req
}

func genServiceConfigs(platform string, opts *catalog.CatalogKafkaManagerOptions) []*manage.ConfigFileContent {
	content := fmt.Sprintf(servicefileContent, platform, opts.HeapSizeMB, opts.User, opts.Password, opts.ZkServiceName)
	serviceCfg := &manage.ConfigFileContent{
		FileName: catalog.SERVICE_FILE_NAME,
		FileMode: common.DefaultConfigFileMode,
		Content:  content,
	}
	return []*manage.ConfigFileContent{serviceCfg}
}

const (
	servicefileContent = `
PLATFORM=%s
HEAP_SIZE_MB=%d
KM_USER=%s
KM_PASSWD=%s
ZK_SERVICE_NAME=%s
`
)
