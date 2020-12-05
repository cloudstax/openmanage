package kafkacatalog

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudstax/firecamp/api/catalog"
	"github.com/cloudstax/firecamp/api/common"
	"github.com/cloudstax/firecamp/api/manage"
	"github.com/cloudstax/firecamp/pkg/dns"
	"github.com/cloudstax/firecamp/pkg/utils"
)

const (
	defaultVersion = "2.6"
	// ContainerImage is the main running container.
	ContainerImage = common.ContainerNamePrefix + "kafka:" + defaultVersion

	// ListenPort is the default kafka listening port
	ListenPort = 9092
	jmxPort    = 9093

	// follow http://docs.confluent.io/current/kafka/deployment.html
	// The KAFKA_JVM_PERFORMANCE_OPTS is combined of the default opts in kafka-run-class.sh and
	// http://docs.confluent.io/current/kafka/deployment.html#jvm
	defaultReplFactor     = 3
	defaultInsyncReplicas = 2
	defaultMaxPartitions  = 8

	// DefaultHeapMB is the default kafka java heap size
	DefaultHeapMB = 6144
	// DefaultRetentionHours 7 days
	DefaultRetentionHours = 168

	serverPropConfFileName = "server.properties"
	javaEnvConfFileName    = "java.env"
	logConfFileName        = "log4j.properties"
)

// The default Kafka catalog service. By default,
// 1) Distribute the nodes on the availability zones.
// 2) Listen on the standard ports, 9092.

// KafkaOptions defines the configurable kafka options
type KafkaOptions struct {
	HeapSizeMB     int64 // 0 == no change
	AllowTopicDel  *bool // nil == no change
	RetentionHours int64 // 0 == no change
	// empty JmxRemoteUser means no change, jmx user could not be disabled.
	// JmxRemotePasswd could not be empty if JmxRemoteUser is set.
	JmxRemoteUser   string
	JmxRemotePasswd string
}

// ValidateUpdateOptions checks if the update options are valid
func ValidateUpdateOptions(opts *KafkaOptions) error {
	if opts.HeapSizeMB < 0 || opts.RetentionHours < 0 {
		return errors.New("heap size and retention hours should not be less than 0")
	}
	if len(opts.JmxRemoteUser) != 0 && len(opts.JmxRemotePasswd) == 0 {
		return errors.New("please set the new jmx remote password")
	}
	return nil
}

// GenDefaultCreateServiceRequest returns the default service creation request.
func GenDefaultCreateServiceRequest(platform string, region string, azs []string,
	cluster string, service string, opts *catalog.CatalogKafkaOptions, res *common.Resources,
	zkServers string) (crReq *manage.CreateServiceRequest, jmxUser string, jmxPasswd string) {
	// check and set the jmx remote user and password
	if len(opts.JmxRemoteUser) == 0 {
		opts.JmxRemoteUser = catalog.JmxDefaultRemoteUser
	}
	if len(opts.JmxRemotePasswd) == 0 {
		opts.JmxRemotePasswd = utils.GenUUID()
	}

	// generate service configs
	serviceCfgs := genServiceConfigs(platform, opts, zkServers)

	// generate member ReplicaConfigs
	replicaCfgs := genMemberConfigs(platform, cluster, service, azs, opts)

	portMappings := []common.PortMapping{
		{ContainerPort: ListenPort, HostPort: ListenPort, IsServicePort: true},
		{ContainerPort: jmxPort, HostPort: jmxPort},
	}

	reserveMemMB := res.ReserveMemMB
	if res.ReserveMemMB < opts.HeapSizeMB {
		reserveMemMB = opts.HeapSizeMB
	}

	req := &manage.CreateServiceRequest{
		Service: &manage.ServiceCommonRequest{
			Region:             region,
			Cluster:            cluster,
			ServiceName:        service,
			CatalogServiceType: common.CatalogService_Kafka,
		},

		Resource: &common.Resources{
			MaxCPUUnits:     res.MaxCPUUnits,
			ReserveCPUUnits: res.ReserveCPUUnits,
			MaxMemMB:        res.MaxMemMB,
			ReserveMemMB:    reserveMemMB,
		},

		ServiceType: common.ServiceTypeStateful,

		ContainerImage: ContainerImage,
		Replicas:       opts.Replicas,
		PortMappings:   portMappings,
		RegisterDNS:    true,

		ServiceConfigs: serviceCfgs,

		Volume:        opts.Volume,
		ContainerPath: common.DefaultContainerMountPath,

		ReplicaConfigs: replicaCfgs,
	}
	return req, opts.JmxRemoteUser, opts.JmxRemotePasswd
}

// genServiceConfigs generates the service configs.
func genServiceConfigs(platform string, opts *catalog.CatalogKafkaOptions, zkServers string) []*manage.ConfigFileContent {
	// adjust the default configs by the number of members(replicas)
	replFactor := defaultReplFactor
	if int(opts.Replicas) < defaultReplFactor {
		replFactor = int(opts.Replicas)
	}
	minInsyncReplica := defaultInsyncReplicas
	if int(opts.Replicas) < defaultInsyncReplicas {
		minInsyncReplica = int(opts.Replicas)
	}
	numPartitions := defaultMaxPartitions
	if int(opts.Replicas) < defaultMaxPartitions {
		numPartitions = int(opts.Replicas)
	}

	// create the service.conf file
	topicDelStr := strconv.FormatBool(opts.AllowTopicDel)
	content := fmt.Sprintf(servicefileContent, platform, opts.HeapSizeMB, topicDelStr,
		numPartitions, opts.RetentionHours, opts.ZkServiceName, replFactor, minInsyncReplica,
		opts.JmxRemoteUser, opts.JmxRemotePasswd, catalog.JmxReadOnlyAccess, jmxPort)
	serviceCfg := &manage.ConfigFileContent{
		FileName: catalog.SERVICE_FILE_NAME,
		FileMode: common.DefaultConfigFileMode,
		Content:  content,
	}

	// create the default server.properties file
	content = fmt.Sprintf(serverPropConfig, zkServers)
	serverCfg := &manage.ConfigFileContent{
		FileName: serverPropConfFileName,
		FileMode: common.DefaultConfigFileMode,
		Content:  content,
	}

	// create the log config file
	logCfg := &manage.ConfigFileContent{
		FileName: logConfFileName,
		FileMode: common.DefaultConfigFileMode,
		Content:  logConfConfig,
	}

	return []*manage.ConfigFileContent{serviceCfg, serverCfg, logCfg}
}

// genMemberConfigs generates the configs for each member.
func genMemberConfigs(platform string, cluster string, service string, azs []string, opts *catalog.CatalogKafkaOptions) []*manage.ReplicaConfig {
	domain := dns.GenDefaultDomainName(cluster)

	replicaCfgs := make([]*manage.ReplicaConfig, opts.Replicas)
	for i := 0; i < int(opts.Replicas); i++ {
		member := utils.GenServiceMemberName(service, int64(i))
		memberHost := dns.GenDNSName(member, domain)

		bindip := memberHost
		if platform == common.ContainerPlatformSwarm {
			bindip = catalog.BindAllIP
		}

		index := i % len(azs)
		content := fmt.Sprintf(memberfileContent, azs[index], memberHost, i, bindip)

		// create the member config file
		memberCfg := &manage.ConfigFileContent{
			FileName: catalog.MEMBER_FILE_NAME,
			FileMode: common.DefaultConfigFileMode,
			Content:  content,
		}

		configs := []*manage.ConfigFileContent{memberCfg}
		replicaCfg := &manage.ReplicaConfig{Zone: azs[index], MemberName: member, Configs: configs}
		replicaCfgs[i] = replicaCfg
	}
	return replicaCfgs
}

// UpdateServiceConfigs update the service configs
func UpdateServiceConfigs(oldContent string, opts *KafkaOptions) string {
	content := oldContent
	lines := strings.Split(oldContent, "\n")
	for _, line := range lines {
		if opts.HeapSizeMB > 0 && strings.HasPrefix(line, "HEAP_SIZE_MB") {
			newHeap := fmt.Sprintf("HEAP_SIZE_MB=%d", opts.HeapSizeMB)
			content = strings.Replace(content, line, newHeap, 1)
		}
		if opts.AllowTopicDel != nil && strings.HasPrefix(line, "ALLOW_TOPIC_DEL") {
			newTopicDel := fmt.Sprintf("ALLOW_TOPIC_DEL=%s", strconv.FormatBool(*opts.AllowTopicDel))
			content = strings.Replace(content, line, newTopicDel, 1)
		}
		if opts.RetentionHours != 0 && strings.HasPrefix(line, "RETENTION_HOURS") {
			newHours := fmt.Sprintf("RETENTION_HOURS=%d", opts.RetentionHours)
			content = strings.Replace(content, line, newHours, 1)
		}
		if len(opts.JmxRemoteUser) != 0 && len(opts.JmxRemotePasswd) != 0 {
			if strings.HasPrefix(line, "JMX_REMOTE_USER") {
				newUser := fmt.Sprintf("JMX_REMOTE_USER=%s", opts.JmxRemoteUser)
				content = strings.Replace(content, line, newUser, 1)
			} else if strings.HasPrefix(line, "JMX_REMOTE_PASSWD") {
				newPasswd := fmt.Sprintf("JMX_REMOTE_PASSWD=%s", opts.JmxRemotePasswd)
				content = strings.Replace(content, line, newPasswd, 1)
			}
		}
	}
	return content
}

const (
	servicefileContent = `
PLATFORM=%s
HEAP_SIZE_MB=%d
ALLOW_TOPIC_DEL=%s
NUMBER_PARTITIONS=%d
RETENTION_HOURS=%d
ZK_SERVICE_NAME=%s
REPLICATION_FACTOR=%d
MIN_INSYNC_REPLICAS=%d
JMX_REMOTE_USER=%s
JMX_REMOTE_PASSWD=%s
JMX_REMOTE_ACCESS=%s
JMX_PORT=%d
`

	memberfileContent = `
AVAILABILITY_ZONE=%s
SERVICE_MEMBER=%s
SERVICE_MEMBER_INDEX=%d
BIND_IP=%s
`

	zkServerSep = ","

	serverPropConfig = `
# The id of the broker. This must be set to a unique integer for each broker.
broker.id=0
broker.rack=rack

delete.topic.enable=true
auto.create.topics.enable=true
num.partitions=8

listeners=PLAINTEXT://bindip:9092
advertised.listeners=PLAINTEXT://advertisedip:9092

log.dirs=/data/kafka

offsets.topic.replication.factor=3
transaction.state.log.replication.factor=3
transaction.state.log.min.isr=2
default.replication.factor=3
min.insync.replicas=2

unclean.leader.election.enable=false

log.retention.hours=168

# e.g. "127.0.0.1:3000,127.0.0.1:3001,127.0.0.1:3002"
zookeeper.connect=%s

group.initial.rebalance.delay.ms=3000

`

	logConfConfig = `
log4j.rootLogger=INFO, stdout

log4j.appender.stdout=org.apache.log4j.ConsoleAppender
log4j.appender.stdout.layout=org.apache.log4j.PatternLayout
log4j.appender.stdout.layout.ConversionPattern=[%d] %p %m (%c)%n

# Change the two lines below to adjust ZK client logging
log4j.logger.org.I0Itec.zkclient.ZkClient=INFO
log4j.logger.org.apache.zookeeper=INFO

# Change the two lines below to adjust the general broker logging level (output to server.log and stdout)
log4j.logger.kafka=INFO
log4j.logger.org.apache.kafka=INFO

`
)
