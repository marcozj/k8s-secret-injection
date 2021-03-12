#!/bin/bash

LOG="/var/log/injector-dmc.log"
counter=20
while [ $counter -gt 0 ]
do
    if /usr/bin/cinfo -A | grep -q 'connected'; then
        counter=0
        echo "cagent is in connected state" >> $LOG
        echo "Injecting credentials..." >> $LOG
        /usr/local/bin/centrify-secret-injector -auth dmc -url $VAULT_URL -scope $VAULT_SCOPE >> $LOG
    else
        echo "waiting $counter..." >> $LOG
        sleep 1
        counter=$(( $counter - 1 ))
    fi
done

