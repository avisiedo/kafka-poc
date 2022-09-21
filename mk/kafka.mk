
KAFKA_IMAGE ?= localhost/kafka:latest
ZOOKEEPER_OPTS ?= -Dzookeeper.4lw.commands.whitelist=*
# Options passed to the jvm invokation for kafka container
KAFKA_OPTS ?= -Dzookeeper.4lw.commands.whitelist=*
# zookeepr client port; it is not publised but used inter containers
ZOOKEEPER_CLIENT_PORT ?= 2181
# The list of topics to be created; if more than one split them by a space
KAFKA_TOPICS ?= repos-introspect
KAFKA_CONFIG_DIR ?= $(PROJECT_DIR)/builds/kafka/config
KAFKA_DATA_DIR ?= $(PROJECT_DIR)/tests/kafka/data
KAFKA_GROUP_ID ?= content-sources

# https://kafka.apache.org/quickstart

.PHONY: kafka-up
kafka-up: DOCKER_IMAGE=$(KAFKA_IMAGE)
kafka-up:  ## Start local kafka containers
	[ -e "$(KAFKA_DATA_DIR)" ] || mkdir -p "$(KAFKA_DATA_DIR)"
	$(DOCKER) container inspect kafka &> /dev/null || $(DOCKER) --log-level $(DOCKER_LOG_LEVEL) run \
	  -d \
	  --rm \
	  --name zookeeper \
	  -e ZOOKEEPER_CLIENT_PORT=$(ZOOKEEPER_CLIENT_PORT) \
	  -e ZOOKEEPER_OPTS="$(ZOOKEEPER_OPTS)" \
	  -v "$(KAFKA_DATA_DIR):/tmp/zookeeper:z" \
	  -v "$(KAFKA_CONFIG_DIR):/tmp/config:z" \
	  -p 8778:8778 \
	  -p 9092:9092 \
	  --health-cmd /opt/kafka/scripts/zookeeper-healthcheck.sh \
	  --health-interval 5s \
	  --health-retries 10 \
	  --health-timeout 3s \
	  --health-start-period 3s \
	  "$(DOCKER_IMAGE)" \
	  /opt/kafka/scripts/zookeeper-entrypoint.sh
	$(DOCKER) container inspect kafka &> /dev/null || $(DOCKER) run \
	  -d \
	  --rm \
	  --name kafka \
	  --net container:zookeeper \
	  -e KAFKA_BROKER_ID=1 \
	  -e KAFKA_ZOOKEEPER_CONNECT="zookeeper:2181" \
	  -e KAFKA_ADVERTISED_LISTENERS="PLAINTEXT://kafka:9092" \
	  -e ZOOKEEPER_CLIENT_PORT=$(ZOOKEEPER_CLIENT_PORT) \
	  -e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=PLAINTEXT:PLAINTEXT \
	  -e KAFKA_INTER_BROKER_LISTENER_NAME=PLAINTEXT \
	  -e KAFKA_OPTS='-javaagent:/usr/jolokia/agents/jolokia-jvm.jar=host=0.0.0.0' \
	  -v "$(KAFKA_DATA_DIR):/tmp/zookeeper:z" \
	  -v "$(KAFKA_CONFIG_DIR):/tmp/config:z" \
	  "$(DOCKER_IMAGE)" \
	  /opt/kafka/scripts/kafka-entrypoint.sh

.PHONY: kafka-down
kafka-down: DOCKER_IMAGE=$(KAFKA_IMAGE)
kafka-down:  ## Stop local kafka infra
	! $(DOCKER) container inspect kafka &> /dev/null || $(DOCKER) container stop kafka
	! $(DOCKER) container inspect zookeeper &> /dev/null || $(DOCKER) container stop zookeeper
	$(DOCKER) container prune -f

.PHONY: kafka-clean
kafka-clean: kafka-down  ## Clean current local kafka infra
	export TMP="$(KAFKA_DATA_DIR)"; [ "$${TMP#$(PROJECT_DIR)/}" != "$${TMP}" ] \
	    || { echo "error:KAFKA_DATA_DIR should belong to $(PROJECT_DIR)"; exit 1; }
	rm -rf "$(KAFKA_DATA_DIR)"

.PHONY: kafka-cli
kafka-cli:  ## Open an interactive shell in kafka container
	! $(DOCKER) container inspect kafka &> /dev/null || $(DOCKER) exec -it --workdir /opt/kafka/bin kafka /bin/bash

.PHONY: kafka-build
kafka-build: DOCKER_IMAGE=$(KAFKA_IMAGE)
kafka-build: DOCKER_DOCKERFILE=$(PROJECT_DIR)/builds/kafka/Dockerfile
kafka-build:   ## Build local kafka container image
	$(DOCKER) build $(DOCKER_BUILD_OPTS) -t "$(DOCKER_IMAGE)" $(DOCKER_CONTEXT_DIR) -f "$(DOCKER_DOCKERFILE)"

.PHONY: kafka-topics-list
kafka-topics-list:  ## List the kafka topics from the kafka container
	$(DOCKER) container inspect kafka &> /dev/null || { echo "error:start kafka container by 'make kafka-up'"; exit 1; }
	$(DOCKER) exec kafka /opt/kafka/bin/kafka-topics.sh --list --bootstrap-server localhost:9092

.PHONY: kafka-topics-create
kafka-topics-create:  ## Create the kafka topics in KAFKA_TOPICS
	$(DOCKER) container inspect kafka &> /dev/null || { echo "error:start kafka container by 'make kafka-up'"; exit 1; }
	for topic in $(KAFKA_TOPICS); do \
	    $(DOCKER) exec kafka /opt/kafka/bin/kafka-topics.sh --create --topic $$topic --bootstrap-server localhost:9092; \
	done

.PHONY: kafka-topics-describe
kafka-topics-describe:  ## Execute kafka-topics.sh for KAFKA_TOPICS
	$(DOCKER) container inspect kafka &> /dev/null || { echo "error:start kafka container by 'make kafka-up'"; exit 1; }
	for topic in $(KAFKA_TOPICS); do \
	    $(DOCKER) exec kafka /opt/kafka/bin/kafka-topics.sh --describe --topic $$topic --bootstrap-server localhost:9092; \
	done

KAFKA_PROPERTIES ?= \
  --property print.key=true \
  --property print.partition=true \
  --property print.headers=true

.PHONY: kafka-topic-consume
kafka-topic-consume: KAFKA_TOPIC ?= $(firstword $(KAFKA_TOPICS))
kafka-topic-consume:
kafka-topic-consume:  ## Execute kafka-console-consume.sh inside the kafka container for KAFKA_TOPIC (singular)
	@[ "$(KAFKA_TOPIC)" != "" ] || { echo "error:KAFKA_TOPIC cannot be empty"; exit 1; }
	$(DOCKER) exec kafka \
	  /opt/kafka/bin/kafka-console-consumer.sh \
	  $(KAFKA_PROPERTIES) \
	  --topic $(KAFKA_TOPIC) \
	  --bootstrap-server localhost:9092

#	  --group $(KAFKA_GROUP_ID) \
