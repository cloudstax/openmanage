FROM debian:jessie

# Add CouchDB user account
RUN groupadd -g 5984 -r couchdb && useradd -u 5984 -d /opt/couchdb -g couchdb couchdb

RUN apt-get update -y && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    erlang-nox \
    erlang-reltool \
    libicu52 \
    libmozjs185-1.0 \
    openssl \
  && rm -rf /var/lib/apt/lists/*

# grab gosu for easy step-down from root and tini for signal handling
# see https://github.com/apache/couchdb-docker/pull/28#discussion_r141112407
ENV GOSU_VERSION 1.10
ENV TINI_VERSION 0.16.1
RUN set -ex; \
	\
	apt-get update; \
	apt-get install -y --no-install-recommends wget; \
	rm -rf /var/lib/apt/lists/*; \
	\
	dpkgArch="$(dpkg --print-architecture | awk -F- '{ print $NF }')"; \
	\
# install gosu
	wget -O /usr/local/bin/gosu "https://github.com/tianon/gosu/releases/download/${GOSU_VERSION}/gosu-$dpkgArch"; \
	wget -O /usr/local/bin/gosu.asc "https://github.com/tianon/gosu/releases/download/$GOSU_VERSION/gosu-$dpkgArch.asc"; \
	export GNUPGHOME="$(mktemp -d)"; \
	gpg --keyserver ha.pool.sks-keyservers.net --recv-keys B42F6819007F00F88E364FD4036A9C25BF357DD4; \
	gpg --batch --verify /usr/local/bin/gosu.asc /usr/local/bin/gosu; \
	rm -r "$GNUPGHOME" /usr/local/bin/gosu.asc; \
	chmod +x /usr/local/bin/gosu; \
	gosu nobody true; \
	\
# install tini
	wget -O /usr/local/bin/tini "https://github.com/krallin/tini/releases/download/v${TINI_VERSION}/tini-$dpkgArch"; \
	wget -O /usr/local/bin/tini.asc "https://github.com/krallin/tini/releases/download/v${TINI_VERSION}/tini-$dpkgArch.asc"; \
	export GNUPGHOME="$(mktemp -d)"; \
	gpg --keyserver ha.pool.sks-keyservers.net --recv-keys 595E85A6B1B4779EA4DAAEC70B588DFF0527A9B7; \
	gpg --batch --verify /usr/local/bin/tini.asc /usr/local/bin/tini; \
	rm -r "$GNUPGHOME" /usr/local/bin/tini.asc; \
	chmod +x /usr/local/bin/tini; \
	tini --version; \
	\
	apt-get purge -y --auto-remove wget

ENV COUCHDB_VERSION 2.3.1

# Download dev dependencies
RUN buildDeps=' \
    apt-transport-https \
    gcc \
    g++ \
    erlang-dev \
    libcurl4-openssl-dev \
    libicu-dev \
    libmozjs185-dev \
    make \
  ' \
 && apt-get update -y -qq && apt-get install -y --no-install-recommends $buildDeps \
 # Acquire CouchDB source code
 && cd /usr/src && mkdir couchdb \
 && curl -fSL https://dist.apache.org/repos/dist/release/couchdb/source/$COUCHDB_VERSION/apache-couchdb-$COUCHDB_VERSION.tar.gz -o couchdb.tar.gz \
 && curl -fSL https://dist.apache.org/repos/dist/release/couchdb/source/$COUCHDB_VERSION/apache-couchdb-$COUCHDB_VERSION.tar.gz.asc -o couchdb.tar.gz.asc \
 && export GPG_KEY=$(gpg --keyid-format=short --list-packets "couchdb.tar.gz.asc" | grep -oE 'keyid .*' | cut -f2 -d' ') \
 && gpg --keyserver ha.pool.sks-keyservers.net --recv-keys "$GPG_KEY" \
 && gpg --batch --verify couchdb.tar.gz.asc couchdb.tar.gz \
 && tar -xzf couchdb.tar.gz -C couchdb --strip-components=1 \
 && cd couchdb \
 # Build the release and install into /opt
 && ./configure --disable-docs \
 && make release \
 && mv /usr/src/couchdb/rel/couchdb /opt/ \
 # Cleanup build detritus
 && apt-get purge -y --auto-remove $buildDeps \
 && rm -rf /var/lib/apt/lists/* /usr/src/couchdb* \
 && mkdir /opt/couchdb/data \
 && chown -R couchdb:couchdb /opt/couchdb

# Setup directories and permissions
RUN chown -R couchdb:couchdb /opt/couchdb/

COPY docker-entrypoint.sh /

EXPOSE 5984 5986 6984 4369
EXPOSE 9100-9200

ENTRYPOINT ["tini", "--", "/docker-entrypoint.sh"]
CMD ["/opt/couchdb/bin/couchdb"]

