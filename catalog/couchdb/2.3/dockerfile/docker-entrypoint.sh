#!/bin/bash
set -e

rootdir=/data
datadir=/data/couchdb
confdir=/data/conf

# after release 0.9.5, these files are not created any more.
syscfgfile=$confdir/sys.conf


localinifile=$confdir/local.ini
vmargsfile=$confdir/vm.args
servicecfgfile=$confdir/service.conf
membercfgfile=$confdir/member.conf

couchdir=/opt/couchdb/etc

USER=couchdb

# sanity check to make sure the volume is mounted to /data.
if [ ! -d "$rootdir" ]; then
  echo "error: $rootdir not exist. Please make sure the volume is mounted to $rootdir." >&2
  exit 1
fi
if [ ! -d "$datadir" ]; then
  mkdir "$datadir"
fi
if [ ! -d "$confdir" ]; then
  echo "error: $confdir not exist." >&2
  exit 1
fi
# sanity check to make sure the config files are created.
if [ ! -f "$localinifile" ]; then
  echo "error: $localinifile not exist." >&2
  exit 1
fi
if [ ! -f "$vmargsfile" ]; then
  echo "error: $vmargsfile not exist." >&2
  exit 1
fi

# allow the container to be started with `--user`
if [ "$1" = '/opt/couchdb/bin/couchdb' -a "$(id -u)" = '0' ]; then
  rootdiruser=$(stat -c "%U" $rootdir)
  if [ "$rootdiruser" != "$USER" ]; then
    echo "chown -R $USER $rootdir"
    chown -R "$USER" "$rootdir"
  fi
  diruser=$(stat -c "%U" $datadir)
  if [ "$diruser" != "$USER" ]; then
    chown -R "$USER" "$datadir"
  fi
  chown -R "$USER" "$confdir"

  exec gosu "$USER" "$BASH_SOURCE" "$@"
fi

# copy the configuration files
cp $localinifile $couchdir
cp $vmargsfile $couchdir

if [ -f $servicecfgfile ]; then
  # after release 0.9.5, load service and member configs
  . $servicecfgfile
  . $membercfgfile

  # update vm.args file
  sed -i 's/couchdb@localhost/couchdb@'$SERVICE_MEMBER'/g' $couchdir/vm.args

  # update local.ini file
  sed -i 's/uuid = uuid-member/uuid = '$UUID'-'$MEMBER_NAME'/g' $couchdir/local.ini
  sed -i 's/enable_cors = false/enable_cors = '$ENABLE_CORS'/g' $couchdir/local.ini
  sed -i 's/admin = encryptePasswd/'$ADMIN' = '$ENCRYPTED_ADMIN_PASSWD'/g' $couchdir/local.ini

  if [ "$ENABLE_CORS" = "true" ]; then
    echo "[cors]" >> $couchdir/local.ini
    echo "credentials = $CREDENTIALS" >> $couchdir/local.ini
    echo "; List of origins separated by a comma, * means accept all" >> $couchdir/local.ini
    echo "; Origins must include the scheme: http://example.com" >> $couchdir/local.ini
    echo "; You can't set origins: * and credentials = true at the same time." >> $couchdir/local.ini
    echo "origins = $ORIGINS" >> $couchdir/local.ini
    echo "; List of accepted headers separated by a comma" >> $couchdir/local.ini
    echo "headers = $HEADERS" >> $couchdir/local.ini
    echo "; List of accepted methods" >> $couchdir/local.ini
    echo "methods = $METHODS" >> $couchdir/local.ini
  fi

  if [ "$ENABLE_SSL" = "true" ]; then
    echo "[daemons]" >> $couchdir/local.ini
    echo "httpsd = {couch_httpd, start_link, [https]}" >> $couchdir/local.ini
    echo "[ssl]" >> $couchdir/local.ini
    echo "cert_file = /data/conf/couchdb.pem" >> $couchdir/local.ini
    echo "key_file = /data/conf/privkey.pem" >> $couchdir/local.ini

    if [ "$HAS_CA_CERT_FILE" = "true" ]; then
      echo "cacert_file = /data/conf/ca-certificates.crt" >> $couchdir/local.ini
    fi
  fi

else
  # load the sys config file
  . $syscfgfile
fi

echo $SERVICE_MEMBER

echo "$@"
exec "$@"
