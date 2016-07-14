#!/bin/sh
set -x
env

prometheus -config.file /conf/prometheus.yml -alertmanager.url http://localhost:9093 &
hugot-demo
