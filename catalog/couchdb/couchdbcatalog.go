package couchdbcatalog

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"

	"github.com/cloudstax/firecamp/api/catalog"
	"github.com/cloudstax/firecamp/api/common"
	"github.com/cloudstax/firecamp/api/manage"
	"github.com/cloudstax/firecamp/pkg/dns"
	"github.com/cloudstax/firecamp/pkg/utils"
)

const (
	defaultVersion = "2.3"
	// ContainerImage is the main running container.
	ContainerImage = common.ContainerNamePrefix + "couchdb:" + defaultVersion
	// InitContainerImage initializes the couchdb cluster.
	InitContainerImage = common.ContainerNamePrefix + "couchdb-init:" + defaultVersion

	// the application access port
	httpPort = 5984
	sslPort  = 6984

	// cluster internal ports
	localPort                  = 5986
	erlangClusterPort          = 4369
	erlangClusterListenPortMin = 9100
	// couchdb cluster setup suggests tcp range 9100-9200. It would not actually use 100 ports.
	//
	// AWS ECS allows up to 100 ports at a time for one container. If you map more than 100 ports,
	// ECS will not be able to place the task.
	// http://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html
	//
	// 9200 is used by ElasticSearch, and 9160 is used by Cassandra.
	// Set the max port as 9159. 59 ports would be enough for couchdb. If not, we could find another range.
	erlangClusterListenPortMax = 9159

	defaultShards      = 8
	defaultReadCopies  = 2
	defaultWriteCopies = 2

	passwdSha1OutputLen = 20
	passwdIter          = 10

	vmArgsConfFileName     = "vm.args"
	localIniConfFileName   = "local.ini"
	privateKeyConfFileName = "privkey.pem"
	certConfFileName       = "couchdb.pem"
	caCertConfFileName     = "ca-certificates.crt"
)

// ValidateRequest checks if the request is valid
func ValidateRequest(r *catalog.CatalogCreateCouchDBRequest) error {
	if r.Options.Replicas == 2 {
		return errors.New("invalid replicas, please create 1, 3 or more replicas")
	}
	if r.Options.EnableCors {
		if r.Options.Credentials && r.Options.Origins == "*" {
			return errors.New("can't set origins: * and credentials = true at the same time")
		}
		if len(r.Options.Origins) == 0 || len(r.Options.Headers) == 0 || len(r.Options.Methods) == 0 {
			return errors.New("cors is enabled, please set the origins, headers and methods")
		}
	}
	if r.Options.EnableSSL {
		if len(r.Options.CertFileContent) == 0 || len(r.Options.KeyFileContent) == 0 {
			return errors.New("ssl is enabled, please set the valid cert and key files")
		}
	}
	return nil
}

// GenDefaultCreateServiceRequest returns the default service creation request.
func GenDefaultCreateServiceRequest(platform string, region string, azs []string, cluster string,
	service string, res *common.Resources, opts *catalog.CatalogCouchDBOptions) *manage.CreateServiceRequest {
	// generate service configs
	serviceCfgs := genServiceConfigs(platform, cluster, service, azs, opts)

	// generate service ReplicaConfigs
	replicaCfgs := genReplicaConfigs(platform, cluster, service, azs, opts)

	portMappings := []common.PortMapping{
		{ContainerPort: httpPort, HostPort: httpPort, IsServicePort: true},
		{ContainerPort: sslPort, HostPort: sslPort, IsServicePort: true},
	}
	if opts.Replicas != int64(1) {
		m := common.PortMapping{ContainerPort: localPort, HostPort: localPort}
		portMappings = append(portMappings, m)
		m = common.PortMapping{ContainerPort: erlangClusterPort, HostPort: erlangClusterPort}
		portMappings = append(portMappings, m)

		for i := int64(erlangClusterListenPortMin); i <= int64(erlangClusterListenPortMax); i++ {
			m = common.PortMapping{ContainerPort: i, HostPort: i}
			portMappings = append(portMappings, m)
		}
	}

	req := &manage.CreateServiceRequest{
		Service: &manage.ServiceCommonRequest{
			Region:             region,
			Cluster:            cluster,
			ServiceName:        service,
			CatalogServiceType: common.CatalogService_CouchDB,
		},

		Resource: res,

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
	return req
}

// genServiceConfigs generates the service configs.
func genServiceConfigs(platform string, cluster string, service string, azs []string, opts *catalog.CatalogCouchDBOptions) []*manage.ConfigFileContent {
	uuid := utils.GenUUID()

	// encrypt password
	encryptedPasswd := encryptPasswd(opts.AdminPasswd)

	hasCert := false
	if len(opts.CACertFileContent) != 0 {
		hasCert = true
	}

	// create the service.conf file
	content := fmt.Sprintf(servicefileContent, platform, uuid, opts.Admin, opts.AdminPasswd, encryptedPasswd,
		strconv.FormatBool(opts.EnableCors), strconv.FormatBool(opts.Credentials), opts.Origins, opts.Headers,
		opts.Methods, strconv.FormatBool(opts.EnableSSL), strconv.FormatBool(hasCert))
	serviceCfg := &manage.ConfigFileContent{
		FileName: catalog.SERVICE_FILE_NAME,
		FileMode: common.DefaultConfigFileMode,
		Content:  content,
	}

	// create vm.args file
	content = fmt.Sprintf(vmArgsConfigs, erlangClusterListenPortMax)
	vmArgsCfg := &manage.ConfigFileContent{
		FileName: vmArgsConfFileName,
		FileMode: common.DefaultConfigFileMode,
		Content:  content,
	}

	// create the local.ini file
	couchContent := couchConfigs

	// set cluster configs
	if opts.Replicas >= int64(3) {
		placement := ""
		if len(azs) == 1 {
			placement = fmt.Sprintf(zonePlacementFmt, azs[0], 3)
		} else if len(azs) == 2 {
			placement = fmt.Sprintf(zonePlacementFmt, azs[0], 2) + zonePlaceSep + fmt.Sprintf(zonePlacementFmt, azs[1], 1)
		} else {
			// len(azs) >= 3
			placement = fmt.Sprintf(zonePlacementFmt, azs[0], 1) + zonePlaceSep + fmt.Sprintf(zonePlacementFmt, azs[1], 1) +
				zonePlaceSep + fmt.Sprintf(zonePlacementFmt, azs[2], 1)
		}

		clusterContent := fmt.Sprintf(clusterConfigs, opts.Replicas, defaultReadCopies, defaultWriteCopies, placement)
		couchContent += clusterContent
	}

	couchCfg := &manage.ConfigFileContent{
		FileName: localIniConfFileName,
		FileMode: common.DefaultConfigFileMode,
		Content:  couchContent,
	}

	configs := []*manage.ConfigFileContent{serviceCfg, vmArgsCfg, couchCfg}

	// add ssl config files
	if opts.EnableSSL {
		certCfg := &manage.ConfigFileContent{
			FileName: certConfFileName,
			FileMode: common.ReadOnlyFileMode,
			Content:  opts.CertFileContent,
		}
		keyCfg := &manage.ConfigFileContent{
			FileName: privateKeyConfFileName,
			FileMode: common.ReadOnlyFileMode,
			Content:  opts.KeyFileContent,
		}
		configs = append(configs, certCfg, keyCfg)

		if len(opts.CACertFileContent) != 0 {
			caCfg := &manage.ConfigFileContent{
				FileName: caCertConfFileName,
				FileMode: common.ReadOnlyFileMode,
				Content:  opts.CACertFileContent,
			}
			configs = append(configs, caCfg)
		}
	}
	return configs
}

// genReplicaConfigs generates the replica configs.
func genReplicaConfigs(platform string, cluster string, service string, azs []string, opts *catalog.CatalogCouchDBOptions) []*manage.ReplicaConfig {
	domain := dns.GenDefaultDomainName(cluster)

	replicaCfgs := make([]*manage.ReplicaConfig, opts.Replicas)
	for i := int64(0); i < opts.Replicas; i++ {
		member := utils.GenServiceMemberName(service, i)
		memberHost := dns.GenDNSName(member, domain)

		// The CouchDB init task follows the same way to assign the az to each node.
		// If the az selection algo is changed here, please also update the init task.
		azIndex := int(i) % len(azs)
		az := azs[azIndex]

		// create the member.conf file
		content := fmt.Sprintf(memberfileContent, az, member, memberHost)
		memberCfg := &manage.ConfigFileContent{
			FileName: catalog.MEMBER_FILE_NAME,
			FileMode: common.DefaultConfigFileMode,
			Content:  content,
		}

		configs := []*manage.ConfigFileContent{memberCfg}
		replicaCfg := &manage.ReplicaConfig{Zone: az, MemberName: member, Configs: configs}
		replicaCfgs[i] = replicaCfg
	}

	return replicaCfgs
}

// couchdb encrypts password using pbkdf2, and store in local.ini with format -pbkdf2-EncryptedPasswd(160bits),entryptionSalt(128bits),Iteration(10)
// example: -pbkdf2-bd261f6d2d30500145a19c502220b02e250a77db,c995aadb9f35d8fc1e0fe9571a07e36b,10
// http://docs.couchdb.org/en/2.1.0/config/auth.html#admins
func encryptPasswd(passwd string) string {
	salt := utils.GenUUID()
	kd := pbkdf2.Key([]byte(passwd), []byte(salt), passwdIter, passwdSha1OutputLen, sha1.New)
	return fmt.Sprintf(pbkdf2Fmt, hex.EncodeToString(kd), salt, passwdIter)
}

// GenDefaultInitTaskRequest returns the default service init task request.
func GenDefaultInitTaskRequest(req *manage.ServiceCommonRequest, azs []string,
	replicas int64, manageurl string, admin string, adminPass string) *manage.RunTaskRequest {

	envkvs := GenInitTaskEnvKVPairs(req.Region, req.Cluster, manageurl, req.ServiceName, azs, replicas, admin, adminPass)

	return &manage.RunTaskRequest{
		Service: req,
		Resource: &common.Resources{
			MaxCPUUnits:     common.DefaultMaxCPUUnits,
			ReserveCPUUnits: common.DefaultReserveCPUUnits,
			MaxMemMB:        common.DefaultMaxMemoryMB,
			ReserveMemMB:    common.DefaultReserveMemoryMB,
		},
		ContainerImage: InitContainerImage,
		TaskType:       common.TaskTypeInit,
		Envkvs:         envkvs,
	}
}

// GenInitTaskEnvKVPairs generates the environment key-values for the init task.
func GenInitTaskEnvKVPairs(region string, cluster string, manageurl string, service string,
	azs []string, replicas int64, admin string, adminPass string) []*common.EnvKeyValuePair {

	kvregion := &common.EnvKeyValuePair{Name: common.ENV_REGION, Value: region}
	kvcluster := &common.EnvKeyValuePair{Name: common.ENV_CLUSTER, Value: cluster}
	kvmgtserver := &common.EnvKeyValuePair{Name: common.ENV_MANAGE_SERVER_URL, Value: manageurl}
	kvservice := &common.EnvKeyValuePair{Name: common.ENV_SERVICE_NAME, Value: service}
	kvsvctype := &common.EnvKeyValuePair{Name: common.ENV_SERVICE_TYPE, Value: common.CatalogService_CouchDB}
	kvport := &common.EnvKeyValuePair{Name: common.ENV_SERVICE_PORT, Value: strconv.Itoa(httpPort)}
	kvop := &common.EnvKeyValuePair{Name: common.ENV_OP, Value: catalog.CatalogSetServiceInitOp}

	zones := ""
	for i, zone := range azs {
		if i == 0 {
			zones = zone
		} else {
			zones = zones + common.ENV_VALUE_SEPARATOR + zone
		}
	}
	kvazs := &common.EnvKeyValuePair{Name: common.ENV_AVAILABILITY_ZONES, Value: zones}

	kvadmin := &common.EnvKeyValuePair{Name: common.ENV_ADMIN, Value: admin}
	// TODO simply pass the password as env variable. The init task should fetch from the manage server.
	kvadminpass := &common.EnvKeyValuePair{Name: common.ENV_ADMIN_PASSWORD, Value: adminPass}

	domain := dns.GenDefaultDomainName(cluster)
	members := dns.GenDNSName(utils.GenServiceMemberName(service, 0), domain)
	for i := int64(1); i < replicas; i++ {
		members = members + common.ENV_VALUE_SEPARATOR + dns.GenDNSName(utils.GenServiceMemberName(service, i), domain)
	}
	kvmembers := &common.EnvKeyValuePair{Name: common.ENV_SERVICE_MEMBERS, Value: members}

	envkvs := []*common.EnvKeyValuePair{kvregion, kvcluster, kvmgtserver, kvservice, kvsvctype, kvport, kvop, kvazs, kvadmin, kvadminpass, kvmembers}
	return envkvs
}

// GetAdminFromServiceConfigs gets the admin config from the service configs
func GetAdminFromServiceConfigs(content string) (admin string, adminPass string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		fields := strings.Split(line, "=")
		if fields[0] == "ADMIN" {
			admin = fields[1]
		} else if fields[0] == "ADMIN_PASSWD" {
			adminPass = fields[1]
		}
	}
	return admin, adminPass
}

const (
	servicefileContent = `
PLATFORM=%s
UUID=%s
ADMIN=%s
ADMIN_PASSWD=%s
ENCRYPTED_ADMIN_PASSWD=%s
ENABLE_CORS=%s
CREDENTIALS=%s
ORIGINS=%s
HEADERS=%s
METHODS=%s
ENABLE_SSL=%s
HAS_CA_CERT_FILE=%s
`

	memberfileContent = `
AVAILABILITY_ZONE=%s
MEMBER_NAME=%s
SERVICE_MEMBER=%s
`

	pbkdf2Fmt        = "-pbkdf2-%s,%s,%d"
	zonePlacementFmt = "%s:%d"
	zonePlaceSep     = ","

	// couchdb@localhost will be replaced at docker entrypoint
	vmArgsConfigs = `
-name couchdb@localhost

# All nodes must share the same magic cookie for distributed Erlang to work.
# Comment out this line if you synchronized the cookies by other means (using
# the ~/.erlang.cookie file, for example).
-setcookie monster

# Tell kernel and SASL not to log anything
-kernel error_logger silent
-sasl sasl_error_logger false

# Erlang kernel listen port
-kernel inet_dist_listen_min 9100
-kernel inet_dist_listen_max %d

# Use kernel poll functionality if supported by emulator
+K true

# Start a pool of asynchronous IO threads
+A 16

# Comment this line out to enable the interactive Erlang shell on startup
+Bd -noinput
`

	couchConfigs = `
[couchdb]
uuid = uuid-member
database_dir = /data/couchdb
view_index_dir = /data/couchdb

[couch_peruser]
; peruser db is broken in 2.1.0, https://github.com/apache/couchdb/issues/749
;enable = true
;delete_dbs = true

[chttpd]
port = 5984
; has to listen on 0.0.0.0, couchdb could not bind dnsname.
bind_address = 0.0.0.0

require_valid_user = true

[httpd]
port = 5986
bind_address = 0.0.0.0
enable_cors = false

WWW-Authenticate = Basic realm="administrator"

[couch_httpd_auth]
require_valid_user = true

; 'username = password'
[admins]
admin = encryptePasswd
`

	clusterConfigs = `
#http://docs.couchdb.org/en/2.1.0/cluster/theory.html
[cluster]
q=%d
r=%d
w=%d
n=3
placement = %s
; placement = metro-dc-a:2,metro-dc-b:1
`
)
