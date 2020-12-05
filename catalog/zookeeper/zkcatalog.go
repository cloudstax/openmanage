package zkcatalog

import (
	"fmt"

	"github.com/cloudstax/firecamp/api/catalog"
	"github.com/cloudstax/firecamp/api/common"
	"github.com/cloudstax/firecamp/api/manage"
	"github.com/cloudstax/firecamp/pkg/dns"
	"github.com/cloudstax/firecamp/pkg/utils"
)

const (
	defaultVersion = "3.5"
	// ContainerImage is the main ZooKeeper running container.
	ContainerImage = common.ContainerNamePrefix + "zookeeper:" + defaultVersion

	// ClientPort is the port at which the clients will connect
	ClientPort      = 2181
	peerConnectPort = 2888
	leaderElectPort = 3888
	jmxPort         = 2191

	// DefaultHeapMB is the default zookeeper java heap size
	DefaultHeapMB = 4096

	zooConfFileName  = "zoo.cfg"
	myidConfFileName = "myid"
	logConfFileName  = "log4j.properties"
)

// The default ZooKeeper catalog service. By default,
// 1) Distribute the node on the availability zones.
// 2) Listen on the standard ports, 2181 2888 3888.

// GenDefaultCreateServiceRequest returns the default service creation request.
func GenDefaultCreateServiceRequest(platform string, region string, azs []string,
	cluster string, service string, opts *catalog.CatalogZooKeeperOptions,
	res *common.Resources) (req *manage.CreateServiceRequest, jmxUser string, jmxPasswd string) {
	// check and set the jmx remote user and password
	if len(opts.JmxRemoteUser) == 0 {
		opts.JmxRemoteUser = catalog.JmxDefaultRemoteUser
	}
	if len(opts.JmxRemotePasswd) == 0 {
		opts.JmxRemotePasswd = utils.GenUUID()
	}

	serviceCfgs := genServiceConfigs(platform, cluster, service, azs, opts)

	// generate service ReplicaConfigs
	replicaCfgs := genReplicaConfigs(platform, region, cluster, service, azs, opts)

	portMappings := []common.PortMapping{
		{ContainerPort: ClientPort, HostPort: ClientPort, IsServicePort: true},
		{ContainerPort: peerConnectPort, HostPort: peerConnectPort},
		{ContainerPort: leaderElectPort, HostPort: leaderElectPort},
		{ContainerPort: jmxPort, HostPort: jmxPort},
	}

	reserveMemMB := res.ReserveMemMB
	if res.ReserveMemMB < opts.HeapSizeMB {
		reserveMemMB = opts.HeapSizeMB
	}

	req = &manage.CreateServiceRequest{
		Service: &manage.ServiceCommonRequest{
			Region:             region,
			Cluster:            cluster,
			ServiceName:        service,
			CatalogServiceType: common.CatalogService_ZooKeeper,
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
		Volume:         opts.Volume,
		ContainerPath:  common.DefaultContainerMountPath,
		PortMappings:   portMappings,

		RegisterDNS:    true,
		ServiceConfigs: serviceCfgs,
		ReplicaConfigs: replicaCfgs,
	}
	return req, opts.JmxRemoteUser, opts.JmxRemotePasswd
}

func genServiceConfigs(platform string, cluster string, service string, azs []string, opts *catalog.CatalogZooKeeperOptions) []*manage.ConfigFileContent {
	domain := dns.GenDefaultDomainName(cluster)
	serverList := genServerList(service, domain, opts.Replicas)

	// create the service.conf file
	content := fmt.Sprintf(servicefileContent, platform, opts.HeapSizeMB, opts.JmxRemoteUser, opts.JmxRemotePasswd, catalog.JmxReadOnlyAccess, jmxPort)
	serviceCfg := &manage.ConfigFileContent{
		FileName: catalog.SERVICE_FILE_NAME,
		FileMode: common.DefaultConfigFileMode,
		Content:  content,
	}

	// create the zoo.cfg file
	zooCfg := &manage.ConfigFileContent{
		FileName: zooConfFileName,
		FileMode: common.DefaultConfigFileMode,
		Content:  fmt.Sprintf(zooConfigs, serverList),
	}

	// create the log config file
	logCfg := &manage.ConfigFileContent{
		FileName: logConfFileName,
		FileMode: common.DefaultConfigFileMode,
		Content:  logConfContent,
	}

	return []*manage.ConfigFileContent{serviceCfg, zooCfg, logCfg}
}

// genReplicaConfigs generates the replica configs.
func genReplicaConfigs(platform string, region string, cluster string, service string, azs []string, opts *catalog.CatalogZooKeeperOptions) []*manage.ReplicaConfig {
	domain := dns.GenDefaultDomainName(cluster)

	replicaCfgs := make([]*manage.ReplicaConfig, opts.Replicas)
	for i := 0; i < int(opts.Replicas); i++ {
		member := utils.GenServiceMemberName(service, int64(i))
		memberHost := dns.GenDNSName(member, domain)

		index := i % len(azs)

		// create the member.conf file
		content := fmt.Sprintf(memberfileContent, azs[index], memberHost)
		memberCfg := &manage.ConfigFileContent{
			FileName: catalog.MEMBER_FILE_NAME,
			FileMode: common.DefaultConfigFileMode,
			Content:  content,
		}

		// create the myid file
		content = fmt.Sprintf(myidConfig, i+1)
		myidCfg := &manage.ConfigFileContent{
			FileName: myidConfFileName,
			FileMode: common.DefaultConfigFileMode,
			Content:  content,
		}

		configs := []*manage.ConfigFileContent{memberCfg, myidCfg}
		replicaCfg := &manage.ReplicaConfig{Zone: azs[index], MemberName: member, Configs: configs}
		replicaCfgs[i] = replicaCfg
	}
	return replicaCfgs
}

func genServerList(service string, domain string, replicas int64) string {
	serverList := ""
	for i := 0; i < int(replicas); i++ {
		member := utils.GenServiceMemberName(service, int64(i))
		dnsname := dns.GenDNSName(member, domain)
		server := fmt.Sprintf(serverLine, i+1, dnsname, peerConnectPort, leaderElectPort)
		serverList += server
	}
	return serverList
}

const (
	servicefileContent = `
PLATFORM=%s
HEAP_SIZE_MB=%d
JMX_REMOTE_USER=%s
JMX_REMOTE_PASSWD=%s
JMX_REMOTE_ACCESS=%s
JMX_PORT=%d
`

	memberfileContent = `
AVAILABILITY_ZONE=%s
SERVICE_MEMBER=%s
`

	serverLine = `
server.%d=%s:%d:%d`

	zooConfigs = `
# http://zookeeper.apache.org/doc/r3.4.10/zookeeperAdmin.html

# The number of milliseconds of each tick
tickTime=2000

# The number of ticks that the initial
# synchronization phase can take
initLimit=10

# The number of ticks that can pass between
# sending a request and getting an acknowledgement
syncLimit=5

# the directory where the snapshot is stored.
dataDir=/data

# the port at which the clients will connect
clientPort=2181

# Be sure to read the maintenance section of the
# administrator guide before turning on autopurge.
#
# The number of snapshots to retain in dataDir
#autopurge.snapRetainCount=3

# Purge task interval in hours
# Set to "0" to disable auto purge feature
#autopurge.purgeInterval=1

# all zookeeper members
%s
`

	myidConfig = `%d`

	logConfContent = `
zookeeper.root.logger=INFO, CONSOLE
zookeeper.console.threshold=INFO

# DEFAULT: console appender only
log4j.rootLogger=${zookeeper.root.logger}

#
# Log INFO level and above messages to the console
#
log4j.appender.CONSOLE=org.apache.log4j.ConsoleAppender
log4j.appender.CONSOLE.Threshold=${zookeeper.console.threshold}
log4j.appender.CONSOLE.layout=org.apache.log4j.PatternLayout
log4j.appender.CONSOLE.layout.ConversionPattern=%d{ISO8601} [myid:%X{myid}] - %-5p [%t:%C{1}@%L] - %m%n
`
)
