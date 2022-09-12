##
# Golang rules to build the binaries, tidy dependencies,
# generate vendor directory, download dependencies and clean
# the generated binaries.
##

# Directory where the built binaries will be generated
GO_OUTPUT ?= $(PROJECT_DIR)/bin
ifeq (,$(shell ls -1d vendor 2>/dev/null))
MOD_VENDOR :=
else
MOD_VENDOR ?= -mod vendor
endif

.PHONY: build
build: $(patsubst cmd/%,$(GO_OUTPUT)/%,$(wildcard cmd/*)) ## Build binaries

# export CGO_ENABLED
# $(GO_OUTPUT)/%: CGO_ENABLED=0
$(GO_OUTPUT)/%: cmd/%/main.go
	@[ -e "$(GO_OUTPUT)" ] || mkdir -p "$(GO_OUTPUT)"
	go build $(MOD_VENDOR) -o "$@" "$<"

.PHONY: clean
clean: ## Clean binaries and testbin generated
	@[ ! -e "$(GO_OUTPUT)" ] || for item in cmd/*; do rm -vf "$(GO_OUTPUT)/$${item##cmd/}"; done
#	@[ ! -e testbin ] || rm -rf testbin

.PHONY: run
run: build ## Run the service locally
	"$(GO_OUTPUT)/content-sources"

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: get-deps
get-deps: ## Download golang dependencies
	go get -d ./...

.PHONY: vendor
vendor: ## Generate vendor/ directory populated with the dependencies
	go mod vendor

.PHONY: test
test: ## Run tests
	CONFIG_PATH="$(PROJECT_DIR)/configs/" go test $(MOD_VENDOR) ./...

.PHONY: test-ci
test-ci: ## Run tests for ci
	go test $(MOD_VENDOR) ./...
