FROM openjdk:8-jre-alpine

# Install required packages
RUN apk add --no-cache \
  bash \
  su-exec

ENV ZOO_USER=zookeeper

RUN set -x \
  && adduser -D "$ZOO_USER"

ARG DISTRO_FOLDER=zookeeper-3.5.8
ARG DISTRO_NAME=apache-$DISTRO_FOLDER-bin


# Download Apache Zookeeper, verify its PGP signature, untar and clean up
RUN set -ex; \
    apk add --no-cache --virtual .build-deps \
        ca-certificates \
        gnupg \
        libressl; \
    wget -q "https://www.apache.org/dist/zookeeper/$DISTRO_FOLDER/$DISTRO_NAME.tar.gz"; \
    wget -q "https://www.apache.org/dist/zookeeper/$DISTRO_FOLDER/$DISTRO_NAME.tar.gz.asc"; \
    export GNUPGHOME="$(mktemp -d)"; \
    export GPG_KEY=$(gpg --keyid-format=short --list-packets "$DISTRO_NAME.tar.gz.asc" | grep -oE 'keyid .*' | cut -f2 -d' '); \
    gpg --keyserver ha.pool.sks-keyservers.net --recv-key "$GPG_KEY" || \
    gpg --keyserver pgp.mit.edu --recv-keys "$GPG_KEY" || \
    gpg --keyserver keyserver.pgp.com --recv-keys "$GPG_KEY"; \
    gpg --batch --verify "$DISTRO_NAME.tar.gz.asc" "$DISTRO_NAME.tar.gz"; \
    tar -xzf "$DISTRO_NAME.tar.gz"; \
    mv "$DISTRO_NAME/conf/"* "$ZOO_CONF_DIR"; \
    rm -rf "$GNUPGHOME" "$DISTRO_NAME.tar.gz" "$DISTRO_NAME.tar.gz.asc"; \
    apk del .build-deps

ENV PATH=$PATH:/$DISTRO_NAME/bin

# set the JVM TTL.
RUN sed -i 's/#networkaddress.cache.ttl=-1/networkaddress.cache.ttl=10/g' $JAVA_HOME/lib/security/java.security

COPY docker-entrypoint.sh /
ENTRYPOINT ["/docker-entrypoint.sh", "zkServer.sh", "--config", "/etc/zk", "start-foreground"]

# expose client port, peer connect port, leader elect port
EXPOSE 2181 2888 3888
# expose jmx port
EXPOSE 2191
