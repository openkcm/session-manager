IMG ?= kms20/session-manager:latest
KUBECTL_CONFIG=${HOME}/.config/k3d/kubeconfig-$(CLUSTER_NAME).yaml
CLUSTER_NAME=session-manager
NAMESPACE=session-manager
PSQL_RELEASE_NAME := postgresql
VALKEY_RELEASE_NAME := valkey
DOCKERFILE_DIR := .
DOCKERFILE_NAME := Dockerfile.dev
CONTEXT_DIR := .
CHART_NAME := session-manager
CHART_DIR := ./charts/session-manager
DB_USERNAME := postgres
DB_PASS := secret
DB_NAME := session_manager
DB_ADMIN_PASS_KEY := secretKey
VALKEY_PASS := $(DB_PASS)

all: clean lint build test image

.PHONY: start
start: start-k3d psql-helm-install valkey-helm-install ensure-deps service-helm-install

.PHONY: start-k3d
start-k3d: delete-cluster clean-k3d
	@echo "Starting k3d."
	@if docker version | grep -q 'colima'; then \
	   K3D_FIX_DNS=0 k3d cluster create $(CLUSTER_NAME) -p "30083:30083@server:0" --api-port 127.0.0.1:6443; \
	else \
	   k3d cluster create $(CLUSTER_NAME) -p "30083:30083@server:0" --api-port 127.0.0.1:6443; \
	fi
	kubectl create namespace $(NAMESPACE)
	kubectl config set-context --current --namespace=$(NAMESPACE)

.PHONY: delete-cluster
delete-cluster:
	@echo "Deleting k3d cluster '$(CLUSTER_NAME)'."
	@if k3d cluster list | grep -q '$(CLUSTER_NAME)'; then \
	   k3d cluster delete $(CLUSTER_NAME); \
	else \
	   echo "k3d cluster '$(CLUSTER_NAME)' does not exist."; \
	fi

.PHONY: clean-k3d
clean-k3d:
	@echo "Cleaning everything in the session-manager namespace in k3d."
	@if kubectl --kubeconfig=${KUBECTL_CONFIG} get namespace $(NAMESPACE) > /dev/null 2>&1; then \
	   kubectl --kubeconfig=${KUBECTL_CONFIG} delete namespace $(NAMESPACE) --ignore-not-found=true; \
	else \
	   echo "Namespace $(NAMESPACE) does not exist."; \
	fi

.PHONY: psql-helm-install
psql-helm-install:
	@helm upgrade --install '$(PSQL_RELEASE_NAME)' oci://registry-1.docker.io/bitnamicharts/postgresql \
	  --set global.postgresql.auth.username=$(DB_USERNAME) \
	  --set global.postgresql.auth.password=$(DB_PASS) \
	  --set global.postgresql.auth.database=$(DB_NAME) \
	  --set global.postgresql.auth.secretKeys.adminPasswordKey=$(DB_ADMIN_PASS_KEY) \
	  --set image.repository=bitnamilegacy/postgresql \
	  --set volumePermissions.image.repository=bitnamilegacy/os-shell \
	  --set metrics.image.repository=bitnamilegacy/postgres-exporter \
	  --set global.security.allowInsecureImages=true \
	  --namespace $(NAMESPACE)

.PHONY: valkey-helm-install
valkey-helm-install:
	@helm upgrade --install '$(VALKEY_RELEASE_NAME)' oci://registry-1.docker.io/cloudpirates/valkey \
	  --set auth.enabled=true \
	  --set auth.password=$(VALKEY_PASS) \
	  --namespace $(NAMESPACE)

.PHONY: ensure-deps
ensure-deps:
	@echo "Waiting for PostgreSQL and ValKey to be available"
	kubectl wait --for=create pod/valkey-0
	kubectl wait pod \
	  --all \
	  --for=condition=Ready \
	  -l 'app.kubernetes.io/name in ($(PSQL_RELEASE_NAME), $(VALKEY_RELEASE_NAME))' \
	  --timeout 5m \
	  -n $(NAMESPACE)

.PHONY: service-helm-install
service-helm-install: k3d-build-image
	@echo "Installing the service via helm"
	@helm dependency build $(CHART_DIR)
	helm upgrade --install $(CHART_NAME) $(CHART_DIR) --namespace $(NAMESPACE) \
		-f $(CHART_DIR)/values-dev.yaml

.PHONY: service-render-helm
service-render-helm:
	helm template --debug $(CHART_NAME) $(CHART_DIR) --namespace $(NAMESPACE) \
		-f $(CHART_DIR)/values-dev.yaml

.PHONY: k3d-build-image
k3d-build-image: docker-dev-build
	@echo "Importing docker image into k3d"
	k3d image import $(IMG) -c $(CLUSTER_NAME)

.PHONY: docker-dev-build
docker-dev-build:
	docker build -f $(DOCKERFILE_DIR)/$(DOCKERFILE_NAME) -t $(IMG) $(CONTEXT_DIR)

.PHONY: codegen
codegen:
	go generate ./...

.PHONY: clean
clean:
	rm -f cover.out cover.html session-manager
	rm -rf cover/

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: build
build:
	go build ./cmd/session-manager

.PHONY: test
test: clean install-gotestsum
	@mkdir -p cover/integration cover/unit
	@go clean -testcache

	gotestsum --junitfile="${CURDIR}/junit-unit.xml" --format=testname -- -count=1 -race -cover ./... -args -test.gocoverdir="${CURDIR}/cover/unit"
	GOCOVERDIR="${CURDIR}/cover/integration" gotestsum --junitfile="${CURDIR}/junit-integration.xml" --format=testname -- -v -count=1 -race --tags=integration ./integration

	@go tool covdata textfmt -i=./cover/unit,./cover/integration -o cover.out
	@grep -v 'github.com/openkcm/session-manager/internal/openapi/'      cover.out > cover.tmp && mv cover.tmp cover.out
	@grep -v 'github.com/openkcm/session-manager/internal/dbtest/'       cover.out > cover.tmp && mv cover.tmp cover.out
	@grep -v 'github.com/openkcm/session-manager/internal/oidc/mock/'    cover.out > cover.tmp && mv cover.tmp cover.out
	@grep -v 'github.com/openkcm/session-manager/internal/session/mock/' cover.out > cover.tmp && mv cover.tmp cover.out
	@go tool cover -func=cover.out

	@echo "On a Mac, you can use the following command to open the coverage report in the browser\ngo tool cover -html=cover.out -o cover.html && open cover.html"

.PHONY: install-gotestsum
install-gotestsum:
	(cd /tmp && go install gotest.tools/gotestsum@latest)

.PHONY: image
image:
	docker build -t ${IMG} .
