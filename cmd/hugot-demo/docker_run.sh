#!/bin/sh
set -x
env

ifconfig -a
ip link
ip addr

sed -i -e "s/PORT/${PORT}/" /conf/alertmanager.yml
AMDDIR=$(mktemp -d)
alertmanager -config.file /conf/alertmanager.yml -storage.path $AMDDIR &

DDIR=$(mktemp -d)
rm -rf ${DDIR}/*
prometheus -config.file /conf/prometheus.yml -alertmanager.url http://localhost:9093 -storage.local.path $DDIR &

hugot-demo


