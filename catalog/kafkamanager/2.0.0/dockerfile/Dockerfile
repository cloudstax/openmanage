FROM kafkamanager/kafka-manager:2.0.0.2

ENV KM_VERSION=2.0.0.2 \
  KM_CONFIGFILE="conf/application.conf"


WORKDIR /opt/kafka-manager-${KM_VERSION}

COPY logback.xml conf/
COPY firecamp-selectmember .
COPY docker-entrypoint.sh .

EXPOSE 9000
ENTRYPOINT ["./docker-entrypoint.sh"]
