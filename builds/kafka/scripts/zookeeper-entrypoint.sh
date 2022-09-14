#!/bin/bash -xv

# Environment variables:
#
#   ZOOKEEPER_CLIENT_PORT
#   ZOOKEEPER_OPTS
#
export EXTRA_ARGS="-Dzookeeper.4lw.commands.whitelist=*"
ls -la /tmp
exec "${KAFKA_HOME}/bin/zookeeper-server-start.sh" /tmp/config/zookeeper.properties
