#!/bin/sh
set -x
env

sed -i -e "s/PORT/${PORT}/" /conf/alertmanager.yml
alertmanager -config.file /conf/alertmanager.yml &

mkdir -f /tmp/data
rm -rf /tmp/data/*

prometheus -config.file /conf/prometheus.yml -alertmanager.url http://localhost:9093 -storage.local.path /tmp/data &
hugot-demo
