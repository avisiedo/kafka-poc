
# .PHONY: ephemeral-setup
# ephemeral-setup: ## Configure bonfire to run locally
# 	bonfire config write-default > $(PROJECT_DIR)/config/bonfire-config.yaml

APP ?= $(APP_NAME)
NAMESPACE ?= $(shell oc project -q 2>/dev/null)
export NAMESPACE
export APP


# https://consoledot.pages.redhat.com/docs/dev/getting-started/ephemeral/onboarding.html
.PHONY: ephemeral-login
ephemeral-login: .old-ephemeral-login ## Help in login to the ephemeral cluster
	@#if [ "$(GH_SESSION_COOKIE)" != "" ]; then python3 $(GO_OUTPUT)/get-token.py; else $(MAKE) .old-ephemeral-login; fi

.PHONY: .old-ephemeral-login
.old-ephemeral-login:
	xdg-open "https://oauth-openshift.apps.c-rh-c-eph.8p0c.p1.openshiftapps.com/oauth/token/request"
	@echo "- Login with github"
	@echo "- Do click on 'Display Token'"
	@echo "- Copy 'Log in with this token' command"
	@echo "- Paste the command in your terminal"
	@echo ""
	@echo "Now you should have access to the cluster, remember to use bonfire to manage namespace lifecycle:"
	@echo '# make ephemeral-namespace-reserve'
	@echo ""
	@echo "Check the namespaces reserved to you by:"
	@echo '# make ephemeral-namespace-list'
	@echo ""
	@echo "If you need to extend 1hour the time for the namespace reservation"
	@echo '# make ephemeral-namespace-extend-1h'
	@echo ""
	@echo "Finally if you don't need the reserved namespace or just you want to cleanup and restart with a fresh namespace you run:"
	@echo '# make ephemeral-namespace-release-all'

# Download https://gitlab.cee.redhat.com/klape/get-token/-/blob/main/get-token.py
$(GO_OUTPUT/get-token.py):
	curl -Ls -o "$(GO_OUTPUT/get-token.py)" "https://gitlab.cee.redhat.com/klape/get-token/-/raw/main/get-token.py"

# Changes to config/bonfire-local.yaml could impact to this rule
.PHONY: ephemeral-deploy
ephemeral-deploy:  ## Deploy application using 'config/bonfire-local.yaml' file
	@# $(MAKE) -C external/hmsidm-poc-golang tidy vendor
	@# $(MAKE) -C external/hmsidm-poc-golang DOCKER_IMAGE_BASE=quay.io/$(QUAY_USER)/$(APP)-backend docker-build docker-push
	@# $(MAKE) docker-build DOCKER_IMAGE_BASE=quay.io/$(QUAY_USER)/$(APP)-backend docker-build docker-push
	@# $(MAKE) docker-build docker-build docker-push DOCKER_IMAGE_BASE=quay.io/$(QUAY_USER)/$(APP)-backend
	oc get secrets/content-sources-certs &>/dev/null || oc create secret generic content-sources-certs --from-file=cdn.redhat.com=$(CERT_PATH)
	$(MAKE) docker-build docker-push
	source .venv/bin/activate && \
	bonfire deploy \
	    --source local \
		--local-config-path configs/bonfire-local.yaml \
		--namespace "$(NAMESPACE)" \
		--set-parameter "$(APP_COMPONENT)/IMAGE=$(DOCKER_IMAGE_BASE)" \
		--set-parameter "$(APP_COMPONENT)/IMAGE_TAG=$(DOCKER_IMAGE_TAG)" \
		$(APP)

.PHONY: ephemeral-undeploy
ephemeral-undeploy: ## Undeploy application from the current namespace
	source .venv/bin/activate && \
	bonfire process \
	    --source local \
		--local-config-path configs/bonfire-local.yaml \
		--namespace "$(NAMESPACE)" \
		--set-parameter "$(APP_COMPONENT)/IMAGE=$(DOCKER_IMAGE_BASE)" \
		--set-parameter "$(APP_COMPONENT)/IMAGE_TAG=$(DOCKER_IMAGE_TAG)" \
		$(APP) 2>/dev/null | json2yaml | oc delete -f -
	! oc get secrets/content-sources-certs &>/dev/null || oc delete secrets/content-sources-certs

# TODO Add command to specify to bonfire the clowdenv template to be used
.PHONY: ephemeral-namespace-reserve
ephemeral-namespace-reserve:  ## Reserve a namespace (requires ephemeral environment)
	command -v bonfire 1>/dev/null 2>/dev/null || { echo "error:bonfire is not available in this environment"; exit 1; }
#oc project "$(shell bonfire namespace reserve --pool managed-kafka 2>/dev/null)"
	oc project "$(shell source $(PROJECT_DIR)/.venv/bin/activate && bonfire namespace reserve 2>/dev/null)"

.PHONY: ephemeral-namespace-release-all
ephemeral-namespace-release-all: ## Release all namespace reserved by us (requires ephemeral environment)
	source .venv/bin/activate && \
	for item in $$( bonfire namespace list --mine --output json | jq -r '. | to_entries | map(select(.key | match("ephemeral-*";"i"))) | map(.key) | .[]' ); do \
	  bonfire namespace release --force $$item ; \
	done

.PHONY: ephemeral-namespace-list
ephemeral-namespace-list: ## List all the namespaces reserved to the current user (requires ephemeral environment)
	source .venv/bin/activate && \
	bonfire namespace list --mine

.PHONY: ephemeral-namespace-extend-1h
ephemeral-namespace-extend-1h: ## Extend for 1 hour the usage of the current ephemeral environment
	source .venv/bin/activate && \
	bonfire namespace extend --duration 1h "$(NAMESPACE)"

# DOCKER_IMAGE_BASE should be a public image
# Tested by 'make ephemeral-build-deploy DOCKER_IMAGE_BASE=quay.io/avisied0/content-sources-backend'
.PHONY: ephemeral-build-deploy
ephemeral-build-deploy:  ## Build and deploy image using 'build_deploy.sh' scripts; It requires to pass DOCKER_IMAGE_BASE
	IMAGE="$(DOCKER_IMAGE_BASE)" IMAGE_TAG="$(DOCKER_IMAGE_TAG)" ./build_deploy.sh

.PHONY: ephemeral-pr-checks
ephemeral-pr-checks:
	IMAGE="$(DOCKER_IMAGE_BASE)" bash ./pr_checks.sh
