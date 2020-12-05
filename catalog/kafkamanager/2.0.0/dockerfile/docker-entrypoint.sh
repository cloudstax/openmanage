#!/bin/bash

if [ -z "$ZK_HOSTS" -o -z "$CLUSTER" -o -z "$SERVICE_NAME" ]; then
  echo "error: please specify the ZK_HOSTS $ZK_HOSTS, CLUSTER $CLUSTER and SERVICE_NAME $SERVICE_NAME"
  exit 1
fi

if [[ $KM_USERNAME != ''  && $KM_PASSWORD != '' ]]; then
  sed -i.bak '/^basicAuthentication/d' ${KM_CONFIGFILE}
  echo 'basicAuthentication.enabled=true' >> ${KM_CONFIGFILE}
  echo "basicAuthentication.username=${KM_USERNAME}" >> ${KM_CONFIGFILE}
  echo "basicAuthentication.password=${KM_PASSWORD}" >> ${KM_CONFIGFILE}
  echo 'basicAuthentication.realm="Kafka-Manager"' >> ${KM_CONFIGFILE}
fi

# for now, simply pass all configs in env variables. later, we may put all configs in service.conf
if [ "$CONTAINER_PLATFORM" = "ecs" -o "$CONTAINER_PLATFORM" = "swarm" ]; then
  # select one service member, update member dns and write the member dns name to /etc/firecamp-member.
  ./firecamp-selectmember -cluster=$CLUSTER -service-name=$SERVICE_NAME -container-platform=$CONTAINER_PLATFORM

  # load member configs
  . /etc/firecamp-member
else
  # for K8s
  # if member is only accessible from within k8s cluster
  #   SERVICE_MEMBER=podname.internaldomain
  # else
  #   the member's dns name the init container will update member dns, or call a tool here.
  #   SERVICE_MEMBER=podname.externaldomain
  # SERVICE_MEMBER=${POD_NAME}.${DOMAIN}
  echo "error: not support k8s yet"
  exit 2
fi

exec ./bin/kafka-manager -Dconfig.file=${KM_CONFIGFILE} "${KM_ARGS}" "${@}"
