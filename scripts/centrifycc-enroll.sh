#!/bin/bash

if [ "$VAULT_URL" = "" ] ; then
  echo No tenant URL specified.
fi

if [ "$VAULT_ENROLLMENTCODE" = "" ] ; then
  echo No enrollment code specified.
fi

# set up command line parameters
CMDPARAM=()

if [ "$VAULT_SCOPE" != "" ] ; then
  CMDPARAM=("${CMDPARAM[@]}" "-d" "$VAULT_SCOPE:.*")
fi

if [ "$PORT" != "" ] ; then
  CMDPARAM=("${CMDPARAM[@]}" "-S" "Port:$PORT")
fi

if [ "$NAME" != "" ] ; then
  CMDPARAM=("${CMDPARAM[@]}" "--name" "$NAME")
fi

if [ "$ADDRESS" != "" ] ; then
  CMDPARAM=("${CMDPARAM[@]}" "--address" "$ADDRESS")
fi

if [ "$CONNECTOR" != "" ] ; then
  CMDPARAM=("${CMDPARAM[@]}" "-S" "\"Connectors:$CONNECTOR\"")
fi

# grant permission for each role that is authorized
IFS=","
for role in $LOGIN_ROLE 
do
  CMDPARAM=("${CMDPARAM[@]}" "--resource-permission" "role:$role:View")
done

# Create env file for injector binary to use.
# injector binary will be executed by systemd so it can't see shell env variables
# source environment variables from the file instead
ENV_FILE="/usr/local/bin/centrify-secret-injector.env"
echo "VAULT_URL=$VAULT_URL" >> $ENV_FILE
echo "VAULT_APPID=$VAULT_APPID" >> $ENV_FILE
echo "VAULT_SCOPE=$VAULT_SCOPE" >> $ENV_FILE
echo "VAULT_AUTHTYPE=$VAULT_AUTHTYPE" >> $ENV_FILE
env | grep "vault://" >> $ENV_FILE

/usr/sbin/cenroll -t $VAULT_URL -F dmc --code $VAULT_ENROLLMENTCODE "${CMDPARAM[@]}" -f &

exec /usr/sbin/init