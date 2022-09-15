##
# This file add rules to generate code from the
# event message specification
#
# This is based on the repository below:
# https://github.com/RedHatInsights/playbook-dispatcher
#
# https://pkg.go.dev/github.com/xeipuuv/gojsonschema
##

GOJSONSCHEMA := $(GO_OUTPUT)/gojsonschema
GOJSONSCHEMA_VERSION := latest

EVENT_SCHEMA_DIR := $(PROJECT_DIR)/pkg/event/schema
EVENT_MESSAGE_DIR := $(PROJECT_DIR)/pkg/event/message

SCHEMA_YAML_FILES := $(wildcard $(EVENT_SCHEMA_DIR)/*.yaml)
SCHEMA_JSON_FILES := $(patsubst $(EVENT_SCHEMA_DIR)/%.yaml,$(EVENT_SCHEMA_DIR)/%.json,$(wildcard $(EVENT_SCHEMA_DIR)/*.yaml))

.PHONY: gen-event-messages
gen-event-messages: $(GOJSONSCHEMA) $(SCHEMA_JSON_FILES)  ## Generate event messages from schemas
	@[ -e "$(EVENT_MESSAGE_DIR)" ] || mkdir -p "$(EVENT_MESSAGE_DIR)"

	@#rm -vf "$(EVENT_MESSAGE_DIR)/header.message.json"
	$(GOJSONSCHEMA) -p message "$(EVENT_SCHEMA_DIR)/header.message.json" -o "$(EVENT_MESSAGE_DIR)/header.types.gen.go"
	@# yaml2json "$(EVENT_SCHEMA_DIR)/header.message.yaml" "$(EVENT_SCHEMA_DIR)/header.message.json"

	@#rm -vf "$(EVENT_MESSAGE_DIR)/introspectRequest.message.json"
	$(GOJSONSCHEMA) -p message "$(EVENT_SCHEMA_DIR)/introspectRequest.message.json" -o "$(EVENT_MESSAGE_DIR)/introspect_request.types.gen.go"
	@# yaml2json "$(EVENT_SCHEMA_DIR)/introspectRequest.message.yaml" "$(EVENT_SCHEMA_DIR)/introspectRequest.message.json"


# .PHONY: gen-event-messages-json
# gen-event-messages-json: $(patsubst $(EVENT_SCHEMA_DIR)/%.yaml,$(EVENT_SCHEMA_DIR)/%.json,$(wildcard $(EVENT_SCHEMA_DIR)/*.yaml))

# $(SCHEMA_JSON_FILES): $(SCHEMA_YAML_FILES)

$(EVENT_SCHEMA_DIR)/%.json: $(EVENT_SCHEMA_DIR)/%.yaml
	@[ -e "$(EVENT_MESSAGE_DIR)" ] || mkdir -p "$(EVENT_MESSAGE_DIR)"
	yaml2json "$<" "$@"

.PHONY: install-gojsonschema
install-gojsonschema: $(GOJSONSCHEMA)

# go install github.com/atombender/go-jsonschema/cmd/gojsonschema
$(GOJSONSCHEMA):
	@{\
		export GOPATH="$(shell mktemp -d "$(PROJECT_DIR)/tmp.XXXXXXXX" 2>/dev/null)" ; \
		echo "Using GOPATH='$${GOPATH}'" ; \
		[ "$${GOPATH}" != "" ] || { echo "error:GOPATH is empty"; exit 1; } ; \
		export GOBIN="$(dir $(GOJSONSCHEMA))" ; \
		echo "Installing 'gojsonschema' at '$(GOJSONSCHEMA)'" ; \
		go install github.com/atombender/go-jsonschema/cmd/gojsonschema@$(GOJSONSCHEMA_VERSION) ; \
		find "$${GOPATH}" -type d -exec chmod u+w {} \; ; \
		rm -rf "$${GOPATH}" ; \
	}


