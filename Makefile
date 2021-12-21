DOCKER := docker
DOCKER_BUILD := $(DOCKER) build
MINIKUBE_DOCKER_ENV := minikube docker-env

DOCKER_IMAGE_REPO := groupcache-example
DOCKER_IMAGE_TAG := local
DOCKER_IMAGE := $(DOCKER_IMAGE_REPO):$(DOCKER_IMAGE_TAG)
DOCKER_BUILD_CONTEXT := .

KUBECTL_CONTEXT := minikube
KUBECTL_NAMESPACE := groupcache-example
KUBECTL := kubectl --context $(KUBECTL_CONTEXT)
KUBECTL_APPLY := $(KUBECTL) apply
KUBECTL_APPLY_NS := $(KUBECTL_APPLY) -n $(KUBECTL_NAMESPACE)

KUBECTL_GROUPCACHE_YAML_BASE := ./k8s
KUBECTL_GROUPCACHE_YAML_NS := $(KUBECTL_GROUPCACHE_YAML_BASE)/namespace.yaml
KUBECTL_GROUPCACHE_YAML_RESOURCES := $(KUBECTL_GROUPCACHE_YAML_BASE)/resources

GROUPCACHE_SERVICE_NAME := groupcache-svc
GROUPCACHE_SELECTOR := app=groupcache

GO := go
GO_BIN := bin

MOD_MODE := -mod=vendor
GO_TEST := $(GO) test $(MOD_MODE)
GO_BUILD := $(GO) build $(MOD_MODE)

all: build-proto test build

build: vendor build-go

test: vendor
	$(GO_TEST) ./...

build-go: $(GO_BIN)/groupcache-example

# naive build target
$(GO_BIN)/groupcache-example: *.go
	$(GO_BUILD) -o $@

vendor: vendor/modules.txt

vendor/modules.txt:
	go mod vendor

.PHONY: docker-build
docker-build:
	$(DOCKER_BUILD) -t $(DOCKER_IMAGE) $(DOCKER_BUILD_CONTEXT)

.PHONY: minikube-docker-build
minikube-docker-build:
	eval $$($(MINIKUBE_DOCKER_ENV)) && \
		$(DOCKER_BUILD) -t $(DOCKER_IMAGE) $(DOCKER_BUILD_CONTEXT)

.PHONY: minikube-apply-ns
minikube-apply-ns:
	$(KUBECTL_APPLY) -f $(KUBECTL_GROUPCACHE_YAML_NS)

.PHONY: minikube-apply-resources
minikube-apply-resources:
	$(KUBECTL_APPLY_NS) -f $(KUBECTL_GROUPCACHE_YAML_RESOURCES)

.PHONY: minikube-apply
minikube-apply: minikube-docker-build minikube-apply-ns minikube-apply-resources

.PHONY: minikube-delete
minikube-delete:
	-$(KUBECTL) delete -f $(KUBECTL_GROUPCACHE_YAML_RESOURCES) 2> /dev/null || true
	-$(KUBECTL) delete -f $(KUBECTL_GROUPCACHE_YAML_NS) 2> /dev/null || true
	-@eval $$($(MINIKUBE_DOCKER_ENV)) && \
		$(DOCKER) image rm $(DOCKER_IMAGE) 2> /dev/null || true

.PHONY: minikube-logs
minikube-logs:
	$(KUBECTL) logs -n $(KUBECTL_NAMESPACE) -f -l $(GROUPCACHE_SELECTOR)

.PHONY: minikube-tunnel
minikube-tunnel:
	minikube service -n $(KUBECTL_NAMESPACE) --url $(GROUPCACHE_SERVICE_NAME) || true

.PHONY: run
run: minikube-apply minikube-tunnel

.PHONY: stop
stop: clean

.PHONY: .go-clean
.go-clean:
	-@rm -r bin/ vendor/ 2> /dev/null || true

.PHONY: clean
clean: minikube-delete .go-clean

include proto/proto.mk