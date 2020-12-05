# vim:set ft=dockerfile:
FROM debian:stretch-slim

# explicitly set user/group IDs
RUN groupadd -r cassandra --gid=999 && useradd -r -g cassandra --uid=999 cassandra

RUN set -ex; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
# solves warning: "jemalloc shared library could not be preloaded to speed up memory allocations"
		libjemalloc1 \
# free is used by cassandra-env.sh
		procps \
	; \
	if ! command -v gpg > /dev/null; then \
		apt-get install -y --no-install-recommends \
			dirmngr \
			gnupg \
		; \
	fi; \
	rm -rf /var/lib/apt/lists/*

# grab gosu for easy step-down from root
ENV GOSU_VERSION 1.10
RUN set -x \
	&& apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && rm -rf /var/lib/apt/lists/* \
	&& wget -O /usr/local/bin/gosu "https://github.com/tianon/gosu/releases/download/$GOSU_VERSION/gosu-$(dpkg --print-architecture)" \
	&& wget -O /usr/local/bin/gosu.asc "https://github.com/tianon/gosu/releases/download/$GOSU_VERSION/gosu-$(dpkg --print-architecture).asc" \
	&& export GNUPGHOME="$(mktemp -d)" \
	&& gpg --keyserver ha.pool.sks-keyservers.net --recv-keys B42F6819007F00F88E364FD4036A9C25BF357DD4 \
	&& gpg --batch --verify /usr/local/bin/gosu.asc /usr/local/bin/gosu \
	&& rm -rf "$GNUPGHOME" /usr/local/bin/gosu.asc \
	&& chmod +x /usr/local/bin/gosu \
	&& gosu nobody true \
	&& apt-get purge -y --auto-remove ca-certificates wget

# https://cwiki.apache.org/confluence/display/CASSANDRA2/DebianPackaging
#ENV GPG_KEYS \
## gpg: key 0353B12C: public key "T Jake Luciani <jake@apache.org>" imported
#	514A2AD631A57A16DD0047EC749D6EEC0353B12C \
## gpg: key FE4B2BDA: public key "Michael Shuler <michael@pbandjelly.org>" imported
#	A26E528B271F19B9E5D8E19EA278B781FE4B2BDA
#RUN set -ex; \
#	export GNUPGHOME="$(mktemp -d)"; \
#	for key in $GPG_KEYS; do \
#		gpg --keyserver ha.pool.sks-keyservers.net --recv-keys "$key"; \
#	done; \
#	gpg --export $GPG_KEYS > /etc/apt/trusted.gpg.d/cassandra.gpg; \
#	rm -rf "$GNUPGHOME"; \
#	apt-key list
RUN set -ex \
	&& apt-get update && apt-get install -y --no-install-recommends apt-transport-https ca-certificates wget && rm -rf /var/lib/apt/lists/* \
	&& wget -O- https://www.apache.org/dist/cassandra/KEYS | apt-key add - \
	&& apt-key list \
	&& apt-get purge -y --auto-remove wget

ENV CASSANDRA_VERSION 3.11.8

RUN set -ex; \
  mkdir -p /usr/share/man/man1/; \
  echo 'deb http://www.apache.org/dist/cassandra/debian 311x main' >> /etc/apt/sources.list.d/cassandra.list; \
  apt-get update; \
	apt-get install -y \
    cassandra="$CASSANDRA_VERSION" \
    cassandra-tools="$CASSANDRA_VERSION"; \
	rm -rf /var/lib/apt/lists/*

# https://issues.apache.org/jira/browse/CASSANDRA-11661
RUN sed -ri 's/^(JVM_PATCH_VERSION)=.*/\1=25/' /etc/cassandra/cassandra-env.sh

# set the JVM TTL.
RUN sed -i 's/#networkaddress.cache.ttl=-1/networkaddress.cache.ttl=10/g' /usr/lib/jvm/java-8-openjdk-amd64/jre/lib/security/java.security

# get jolokia jvm agent
RUN set -ex; \
  mkdir -p /opt/jolokia-agent/; \
	apt-get update; \
	apt-get install -y wget; \
  wget http://search.maven.org/remotecontent?filepath=org/jolokia/jolokia-jvm/1.5.0/jolokia-jvm-1.5.0-agent.jar -O /opt/jolokia-agent/jolokia-jvm-1.5.0-agent.jar; \
	rm -rf /var/lib/apt/lists/*

COPY docker-entrypoint.sh /
ENTRYPOINT ["/docker-entrypoint.sh", "cassandra", "-f"]

# 7000: intra-node communication
# 7001: TLS intra-node communication
# 7199: JMX
# 9042: CQL
# 9160: thrift service
EXPOSE 7000 7001 7199 9042 9160

# jolokia default port
EXPOSE 8778
