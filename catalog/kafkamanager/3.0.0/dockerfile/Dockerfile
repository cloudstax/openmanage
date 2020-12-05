FROM kafkamanager/kafka-manager:3.0.0.4

ENV KM_VERSION=3.0.0.4 \
  KM_CONFIGFILE="conf/application.conf"


WORKDIR /opt/cmak-${KM_VERSION}

COPY logback.xml conf/
COPY firecamp-selectmember .
COPY docker-entrypoint.sh .

EXPOSE 9000
ENTRYPOINT ["./docker-entrypoint.sh"]
