#!/bin/bash
set -e

datadir=/data
confdir=/data/conf

# after release 0.9.5, sys.conf and java.env files are not created any more.
# instead service.conf and emember.conf are created for the common service configs
# and the service member configs.
syscfgfile=$confdir/sys.conf
javaenvfile=$confdir/java.env

zoocfgfile=$confdir/zoo.cfg
logcfgfile=$confdir/log4j.properties
myidfile=$confdir/myid
servicecfgfile=$confdir/service.conf
membercfgfile=$confdir/member.conf

export ZOOCFGDIR=/etc/zk
export ZOO_LOG_DIR=.

# sanity check to make sure the volume is mounted to /data.
if [ ! -d "$datadir" ]; then
  echo "error: $datadir not exist. Please make sure the volume is mounted to $datadir." >&2
  exit 1
fi
if [ ! -d "$confdir" ]; then
  echo "error: $confdir not exist." >&2
  exit 1
fi
# sanity check to make sure the service config file is created.
if [ ! -f "$zoocfgfile" ]; then
  echo "error: $zoocfgfile not exist." >&2
  exit 1
fi
if [ ! -d "$ZOOCFGDIR" ]; then
  mkdir $ZOOCFGDIR
fi

if [ "$(id -u)" = '0' ]; then
  if [ -f "$servicecfgfile" ]; then
    # after release 0.9.5
    . $servicecfgfile
    . $membercfgfile
  else
    # before release 0.9.6, load the sys config file
    . $syscfgfile
  fi

  if [ "$PLATFORM" = "swarm" ]; then
    # some platform, such as docker swarm, does not allow not using host network for service.
    # append the SERVICE_MEMBER to the last line of /etc/hosts, so the service could bind the
    # member name and serve correctly.
    echo "$(cat /etc/hosts) $SERVICE_MEMBER" > /etc/hosts
  fi
fi

# allow the container to be started with `--user`
if [ "$1" = 'zkServer.sh' -a "$(id -u)" = '0' ]; then
  cp $zoocfgfile $ZOOCFGDIR
  cp $logcfgfile $ZOOCFGDIR
  chown -R $ZOO_USER $ZOOCFGDIR

  datadiruser=$(stat -c "%U" $datadir)
  if [ "$datadiruser" != "$ZOO_USER" ]; then
    chown -R "$ZOO_USER" "$datadir"
  fi
  chown -R "$ZOO_USER" "$confdir"

  exec su-exec "$ZOO_USER" "$BASH_SOURCE" "$@"
fi

# copy myid file if it doesn't exist
if [ ! -f "$datadir/myid" ]; then
  cp $myidfile $datadir/myid
fi

if [ -f "$servicecfgfile" ]; then
  # after release 0.9.5
  . $servicecfgfile
  . $membercfgfile

  # create jmx password and access files
  echo "$JMX_REMOTE_USER $JMX_REMOTE_PASSWD" > $ZOOCFGDIR/jmxremote.password
  echo "$JMX_REMOTE_USER $JMX_REMOTE_ACCESS" > $ZOOCFGDIR/jmxremote.access
  chmod 0400 $ZOOCFGDIR/jmxremote.password
  chmod 0400 $ZOOCFGDIR/jmxremote.access

  # set java env
  export JVMFLAGS="-Xmx${HEAP_SIZE_MB}m -Djava.rmi.server.hostname=$SERVICE_MEMBER -Dcom.sun.management.jmxremote.rmi.port=$JMX_PORT -Dcom.sun.management.jmxremote.password.file=${ZOOCFGDIR}/jmxremote.password -Dcom.sun.management.jmxremote.access.file=${ZOOCFGDIR}/jmxremote.access"
  export JMXPORT=$JMX_PORT
  export JMXAUTH=true
  export JMXSSL=false
  export JMXLOG4J=true
else
  # before release 0.9.6, load the sys config file
  . $syscfgfile
  cp $javaenvfile $ZOOCFGDIR
fi

echo $SERVICE_MEMBER
echo ""

echo "$@"
exec "$@"
