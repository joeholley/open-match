################################################################################
##                             Open Match Makefile                            ##
################################################################################

# Notice: There's 2 variables you need to make sure are set.
# GCP_PROJECT_ID if you're working against GCP.
# Or $REGISTRY if you want to use your own custom docker registry.

# Basic Deployment
# make create-gke-cluster OR make create-mini-cluster
# make push-helm
# make REGISTRY=gcr.io/$PROJECT_ID push-images -j$(nproc)
# make install-chart
#
# Generate Files
# make all-protos
#
# Building
# make all -j$(nproc)
#
# Access monitoring
# make proxy-prometheus
# make proxy-grafana
#
# Run those tools
# make run-backendclient
# make run-frontendclient
# make run-clientloadgen
#
# Teardown
# make delete-mini-cluster
# make delete-gke-cluster
#
## http://makefiletutorial.com/

BASE_VERSION = 0.4.0
VERSION_SUFFIX = $(shell git rev-parse --short=7 HEAD)
VERSION ?= $(BASE_VERSION)-$(VERSION_SUFFIX)

PROTOC_VERSION = 3.7.0
GOLANG_VERSION = 1.12
HELM_VERSION = 2.13.0
HUGO_VERSION = 0.54.0
KUBECTL_VERSION = 1.13.0
PROTOC_RELEASE_BASE = https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)
GO = go
GO_BIN := $(GOPATH)/bin
GO_BUILD_COMMAND = CGO_ENABLED=0 GOOS=linux $(GO) build -a -installsuffix cgo .
BUILD_DIR = $(CURDIR)/build
TOOLCHAIN_DIR = $(BUILD_DIR)/toolchain
TOOLCHAIN_BIN = $(TOOLCHAIN_DIR)/bin
PROTOC := $(TOOLCHAIN_BIN)/protoc
PROTOC_INCLUDES := $(TOOLCHAIN_DIR)/include/
GCP_PROJECT_ID =
GCP_PROJECT_FLAG = --project=$(GCP_PROJECT_ID)
OM_SITE_GCP_PROJECT_ID = open-match-site
OM_SITE_GCP_PROJECT_FLAG = --project=$(OM_SITE_GCP_PROJECT_ID)
REGISTRY = gcr.io/$(GCP_PROJECT_ID)
TAG := $(VERSION)
ALTERNATE_TAG := dev
GKE_CLUSTER_NAME = om-cluster
GCP_REGION = us-west1
GCP_ZONE = us-west1-a
EXE_EXTENSION =
LOCAL_CLOUD_BUILD_PUSH = # --push
GOPATH_PRIMARY = $(HOME)
KUBECTL_RUN_ENV = --env='REDIS_SERVICE_HOST=$$(OPEN_MATCH_REDIS_MASTER_SERVICE_HOST)' --env='REDIS_SERVICE_PORT=$$(OPEN_MATCH_REDIS_MASTER_SERVICE_PORT)'
GCP_LOCATION_FLAG = --zone $(GCP_ZONE)
GO111MODULE = on
PROMETHEUS_PORT = 9090
GRAFANA_PORT = 3000
SITE_PORT = 8080
HELM = $(TOOLCHAIN_BIN)/helm
TILLER = $(TOOLCHAIN_BIN)/tiller
MINIKUBE = $(TOOLCHAIN_BIN)/minikube
KUBECTL = $(TOOLCHAIN_BIN)/kubectl
SERVICE = default

## Make port forwards accessible outside of the proxy machine.
PORT_FORWARD_ADDRESS_FLAG = --address 0.0.0.0
DASHBOARD_PORT = 9092
export PATH := $(CURDIR)/node_modules/.bin/:$(TOOLCHAIN_BIN):$(TOOLCHAIN_DIR)/nodejs/bin:$(PATH)

ifneq (,$(wildcard $(TOOLCHAIN_GOLANG_DIR)/bin/go))
	export GO = $(CURDIR)/$(TOOLCHAIN_GOLANG_DIR)/bin/go
	export GOROOT = $(CURDIR)/$(TOOLCHAIN_GOLANG_DIR)
	export PATH := $(TOOLCHAIN_GOLANG_DIR):$(PATH)
endif

# Get the project from gcloud if it's not set.
ifeq ($(GCP_PROJECT_ID),)
	export GCP_PROJECT_ID = $(shell gcloud config list --format 'value(core.project)')
endif

ifeq ($(OS),Windows_NT)
	# TODO: Windows packages are here but things are broken since many paths are Linux based and zip vs tar.gz.
	HELM_PACKAGE = https://storage.googleapis.com/kubernetes-helm/helm-v$(HELM_VERSION)-windows-amd64.zip
	MINIKUBE_PACKAGE = https://storage.googleapis.com/minikube/releases/latest/minikube-windows-amd64.exe
	SKAFFOLD_PACKAGE = https://storage.googleapis.com/skaffold/releases/latest/skaffold-windows-amd64.exe
	EXE_EXTENSION = .exe
	PROTOC_PACKAGE = $(PROTOC_RELEASE_BASE)-win64.zip
	GO_PACKAGE = https://storage.googleapis.com/golang/go$(GOLANG_VERSION).windows-amd64.zip
	KUBECTL_PACKAGE = https://storage.googleapis.com/kubernetes-release/release/v$(KUBECTL_VERSION)/bin/windows/amd64/kubectl.exe
	HUGO_PACKAGE = https://github.com/gohugoio/hugo/releases/download/v$(HUGO_VERSION)/hugo_extended_$(HUGO_VERSION)_Windows-64bit.zip
	NODEJS_PACKAGE = https://nodejs.org/dist/v10.15.3/node-v10.15.3-win-x64.zip
	NODEJS_PACKAGE_NAME = nodejs.zip
else
	UNAME_S := $(shell uname -s)
	ifeq ($(UNAME_S),Linux)
		HELM_PACKAGE = https://storage.googleapis.com/kubernetes-helm/helm-v$(HELM_VERSION)-linux-amd64.tar.gz
		MINIKUBE_PACKAGE = https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
		SKAFFOLD_PACKAGE = https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64
		PROTOC_PACKAGE = $(PROTOC_RELEASE_BASE)-linux-x86_64.zip
		GO_PACKAGE = https://storage.googleapis.com/golang/go$(GOLANG_VERSION).linux-amd64.tar.gz
		KUBECTL_PACKAGE = https://storage.googleapis.com/kubernetes-release/release/v$(KUBECTL_VERSION)/bin/linux/amd64/kubectl
		HUGO_PACKAGE = https://github.com/gohugoio/hugo/releases/download/v$(HUGO_VERSION)/hugo_extended_$(HUGO_VERSION)_Linux-64bit.tar.gz
		NODEJS_PACKAGE = https://nodejs.org/dist/v10.15.3/node-v10.15.3-linux-x64.tar.xz
		NODEJS_PACKAGE_NAME = nodejs.tar.xz
	endif
	ifeq ($(UNAME_S),Darwin)
		HELM_PACKAGE = https://storage.googleapis.com/kubernetes-helm/helm-v$(HELM_VERSION)-darwin-amd64.tar.gz
		MINIKUBE_PACKAGE = https://storage.googleapis.com/minikube/releases/latest/minikube-darwin-amd64
		SKAFFOLD_PACKAGE = https://storage.googleapis.com/skaffold/releases/latest/skaffold-darwin-amd64
		PROTOC_PACKAGE = $(PROTOC_RELEASE_BASE)-osx-x86_64.zip
		GO_PACKAGE = https://storage.googleapis.com/golang/go$(GOLANG_VERSION).darwin-amd64.tar.gz
		KUBECTL_PACKAGE = https://storage.googleapis.com/kubernetes-release/release/v$(KUBECTL_VERSION)/bin/darwin/amd64/kubectl
		HUGO_PACKAGE = https://github.com/gohugoio/hugo/releases/download/v$(HUGO_VERSION)/hugo_extended_$(HUGO_VERSION)_macOS-64bit.tar.gz
		NODEJS_PACKAGE = https://nodejs.org/dist/v10.15.3/node-v10.15.3-darwin-x64.tar.gz
		NODEJS_PACKAGE_NAME = nodejs.tar.gz
	endif
endif

help:
	@cat Makefile | grep ^\# | grep -v ^\#\# | cut -c 3-

local-cloud-build:
	cloud-build-local --config=cloudbuild.yaml --dryrun=false $(LOCAL_CLOUD_BUILD_PUSH) -substitutions SHORT_SHA=$(VERSION_SUFFIX) .

push-images: push-service-images push-client-images push-mmf-example-images push-evaluator-example-images
push-service-images: push-frontendapi-image push-backendapi-image push-mmforc-image push-mmlogicapi-image
push-mmf-example-images: push-mmf-cs-mmlogic-simple-image push-mmf-go-mmlogic-simple-image push-mmf-php-mmlogic-simple-image push-mmf-py3-mmlogic-simple-image
push-client-images: push-backendclient-image push-clientloadgen-image push-frontendclient-image
push-evaluator-example-images: push-evaluator-simple-image

push-frontendapi-image: build-frontendapi-image
	docker push $(REGISTRY)/openmatch-frontendapi:$(TAG)
	docker push $(REGISTRY)/openmatch-frontendapi:$(ALTERNATE_TAG)

push-backendapi-image: build-backendapi-image
	docker push $(REGISTRY)/openmatch-backendapi:$(TAG)
	docker push $(REGISTRY)/openmatch-backendapi:$(ALTERNATE_TAG)

push-mmforc-image: build-mmforc-image
	docker push $(REGISTRY)/openmatch-mmforc:$(TAG)
	docker push $(REGISTRY)/openmatch-mmforc:$(ALTERNATE_TAG)

push-mmlogicapi-image: build-mmlogicapi-image
	docker push $(REGISTRY)/openmatch-mmlogicapi:$(TAG)
	docker push $(REGISTRY)/openmatch-mmlogicapi:$(ALTERNATE_TAG)

push-mmf-cs-mmlogic-simple-image: build-mmf-cs-mmlogic-simple-image
	docker push $(REGISTRY)/openmatch-mmf-cs-mmlogic-simple:$(TAG)
	docker push $(REGISTRY)/openmatch-mmf-cs-mmlogic-simple:$(ALTERNATE_TAG)

push-mmf-go-mmlogic-simple-image: build-mmf-go-mmlogic-simple-image
	docker push $(REGISTRY)/openmatch-mmf-go-mmlogic-simple:$(TAG)
	docker push $(REGISTRY)/openmatch-mmf-go-mmlogic-simple:$(ALTERNATE_TAG)

push-mmf-php-mmlogic-simple-image: build-mmf-php-mmlogic-simple-image
	docker push $(REGISTRY)/openmatch-mmf-php-mmlogic-simple:$(TAG)
	docker push $(REGISTRY)/openmatch-mmf-php-mmlogic-simple:$(ALTERNATE_TAG)

push-mmf-py3-mmlogic-simple-image: build-mmf-py3-mmlogic-simple-image
	docker push $(REGISTRY)/openmatch-mmf-py3-mmlogic-simple:$(TAG)
	docker push $(REGISTRY)/openmatch-mmf-py3-mmlogic-simple:$(ALTERNATE_TAG)

push-backendclient-image: build-backendclient-image
	docker push $(REGISTRY)/openmatch-backendclient:$(TAG)
	docker push $(REGISTRY)/openmatch-backendclient:$(ALTERNATE_TAG)

push-clientloadgen-image: build-clientloadgen-image
	docker push $(REGISTRY)/openmatch-clientloadgen:$(TAG)
	docker push $(REGISTRY)/openmatch-clientloadgen:$(ALTERNATE_TAG)

push-frontendclient-image: build-frontendclient-image
	docker push $(REGISTRY)/openmatch-frontendclient:$(TAG)
	docker push $(REGISTRY)/openmatch-frontendclient:$(ALTERNATE_TAG)

push-evaluator-simple-image: build-evaluator-simple-image
	docker push $(REGISTRY)/openmatch-evaluator-simple:$(TAG)
	docker push $(REGISTRY)/openmatch-evaluator-simple:$(ALTERNATE_TAG)

build-images: build-service-images build-client-images build-mmf-example-images build-evaluator-example-images
build-service-images: build-frontendapi-image build-backendapi-image build-mmforc-image build-mmlogicapi-image
build-client-images: build-backendclient-image build-clientloadgen-image build-frontendclient-image
build-mmf-example-images: build-mmf-cs-mmlogic-simple-image build-mmf-go-mmlogic-simple-image build-mmf-php-mmlogic-simple-image build-mmf-py3-mmlogic-simple-image
build-evaluator-example-images: build-evaluator-simple-image

build-frontendapi-image: cmd/frontendapi/frontendapi
	docker build -f cmd/frontendapi/Dockerfile -t $(REGISTRY)/openmatch-frontendapi:$(TAG) -t $(REGISTRY)/openmatch-frontendapi:$(ALTERNATE_TAG) .

build-backendapi-image: cmd/backendapi/backendapi
	docker build -f cmd/backendapi/Dockerfile -t $(REGISTRY)/openmatch-backendapi:$(TAG) -t $(REGISTRY)/openmatch-backendapi:$(ALTERNATE_TAG) .

build-mmforc-image: cmd/mmforc/mmforc
	docker build -f cmd/mmforc/Dockerfile -t $(REGISTRY)/openmatch-mmforc:$(TAG) -t $(REGISTRY)/openmatch-mmforc:$(ALTERNATE_TAG) .

build-mmlogicapi-image: cmd/mmlogicapi/mmlogicapi
	docker build -f cmd/mmlogicapi/Dockerfile -t $(REGISTRY)/openmatch-mmlogicapi:$(TAG) -t $(REGISTRY)/openmatch-mmlogicapi:$(ALTERNATE_TAG) .

build-mmf-cs-mmlogic-simple-image:
	cd examples/functions/csharp/simple/ && docker build -f Dockerfile -t $(REGISTRY)/openmatch-mmf-cs-mmlogic-simple:$(TAG) -t $(REGISTRY)/openmatch-mmf-cs-mmlogic-simple:$(ALTERNATE_TAG) .

build-mmf-go-mmlogic-simple-image:
	docker build -f examples/functions/golang/manual-simple/Dockerfile -t $(REGISTRY)/openmatch-mmf-go-mmlogic-simple:$(TAG) -t $(REGISTRY)/openmatch-mmf-go-mmlogic-simple:$(ALTERNATE_TAG) .

build-mmf-php-mmlogic-simple-image:
	docker build -f examples/functions/php/mmlogic-simple/Dockerfile -t $(REGISTRY)/openmatch-mmf-php-mmlogic-simple:$(TAG) -t $(REGISTRY)/openmatch-mmf-php-mmlogic-simple:$(ALTERNATE_TAG) .

build-mmf-py3-mmlogic-simple-image:
	docker build -f examples/functions/python3/mmlogic-simple/Dockerfile -t $(REGISTRY)/openmatch-mmf-py3-mmlogic-simple:$(TAG) -t $(REGISTRY)/openmatch-mmf-py3-mmlogic-simple:$(ALTERNATE_TAG) .

build-backendclient-image: examples/backendclient/backendclient
	docker build -f examples/backendclient/Dockerfile -t $(REGISTRY)/openmatch-backendclient:$(TAG) -t $(REGISTRY)/openmatch-backendclient:$(ALTERNATE_TAG) .

build-clientloadgen-image: test/cmd/clientloadgen/clientloadgen
	docker build -f test/cmd/clientloadgen/Dockerfile -t $(REGISTRY)/openmatch-clientloadgen:$(TAG) -t $(REGISTRY)/openmatch-clientloadgen:$(ALTERNATE_TAG) .

build-frontendclient-image: test/cmd/frontendclient/frontendclient
	docker build -f test/cmd/frontendclient/Dockerfile -t $(REGISTRY)/openmatch-frontendclient:$(TAG) -t $(REGISTRY)/openmatch-frontendclient:$(ALTERNATE_TAG) .

build-evaluator-simple-image: examples/evaluators/golang/simple/simple
	docker build -f examples/evaluators/golang/simple/Dockerfile -t $(REGISTRY)/openmatch-evaluator-simple:$(TAG) -t $(REGISTRY)/openmatch-evaluator-simple:$(ALTERNATE_TAG) .

clean-images:
	-docker rmi -f $(REGISTRY)/openmatch-frontendapi:$(TAG) $(REGISTRY)/openmatch-frontendapi:$(ALTERNATE_TAG)
	-docker rmi -f $(REGISTRY)/openmatch-backendapi:$(TAG) $(REGISTRY)/openmatch-backendapi:$(ALTERNATE_TAG)
	-docker rmi -f $(REGISTRY)/openmatch-mmforc:$(TAG) $(REGISTRY)/openmatch-mmforc:$(ALTERNATE_TAG)
	-docker rmi -f $(REGISTRY)/openmatch-mmlogicapi:$(TAG) $(REGISTRY)/openmatch-mmlogicapi:$(ALTERNATE_TAG)

	-docker rmi -f $(REGISTRY)/openmatch-mmf-cs-mmlogic-simple:$(TAG) $(REGISTRY)/openmatch-mmf-cs-mmlogic-simple:$(ALTERNATE_TAG)
	-docker rmi -f $(REGISTRY)/openmatch-mmf-go-mmlogic-simple:$(TAG) $(REGISTRY)/openmatch-mmf-go-mmlogic-simple:$(ALTERNATE_TAG)
	-docker rmi -f $(REGISTRY)/openmatch-mmf-php-mmlogic-simple:$(TAG) $(REGISTRY)/openmatch-mmf-php-mmlogic-simple:$(ALTERNATE_TAG)
	-docker rmi -f $(REGISTRY)/openmatch-mmf-py3-mmlogic-simple:$(TAG) $(REGISTRY)/openmatch-mmf-py3-mmlogic-simple:$(ALTERNATE_TAG)

	-docker rmi -f $(REGISTRY)/openmatch-backendclient:$(TAG) $(REGISTRY)/openmatch-backendclient:$(ALTERNATE_TAG)
	-docker rmi -f $(REGISTRY)/openmatch-clientloadgen:$(TAG) $(REGISTRY)/openmatch-clientloadgen:$(ALTERNATE_TAG)
	-docker rmi -f $(REGISTRY)/openmatch-frontendclient:$(TAG) $(REGISTRY)/openmatch-frontendclient:$(ALTERNATE_TAG)
	-docker rmi -f $(REGISTRY)/openmatch-evaluator-simple:$(TAG) $(REGISTRY)/openmatch-evaluator-simple:$(ALTERNATE_TAG)

install-redis: build/toolchain/bin/helm$(EXE_EXTENSION)
	$(HELM) upgrade --install --wait --debug redis stable/redis --namespace redis

chart-deps: build/toolchain/bin/helm$(EXE_EXTENSION)
	(cd install/helm/open-match; $(HELM) dependency update)

print-chart: build/toolchain/bin/helm$(EXE_EXTENSION)
	(cd install/helm; $(HELM) lint open-match; $(HELM) install --dry-run --debug open-match)

install-chart: build/toolchain/bin/helm$(EXE_EXTENSION)
	$(HELM) upgrade --install --wait --debug open-match install/helm/open-match \
	  --namespace=open-match \
	  --set openmatch.image.registry=$(REGISTRY) \
	  --set openmatch.image.tag=$(TAG)

install-example-chart: build/toolchain/bin/helm$(EXE_EXTENSION)
	$(HELM) upgrade --install --wait --debug open-match-example install/helm/open-match-example \
	  --namespace=open-match \
	  --set openmatch.image.registry=$(REGISTRY) \
	  --set openmatch.image.tag=$(TAG)

delete-example-chart: build/toolchain/bin/helm$(EXE_EXTENSION)
	-$(HELM) delete --purge open-match-example

dry-chart: build/toolchain/bin/helm$(EXE_EXTENSION)
	$(HELM) upgrade --install --wait --debug --dry-run open-match install/helm/open-match \
	  --namespace=open-match \
	  --set openmatch.image.registry=$(REGISTRY) \
	  --set openmatch.image.tag=$(TAG)

delete-chart: build/toolchain/bin/helm$(EXE_EXTENSION) build/toolchain/bin/kubectl$(EXE_EXTENSION)
	-$(HELM) delete --purge open-match
	-$(KUBECTL) delete crd prometheuses.monitoring.coreos.com
	-$(KUBECTL) delete crd servicemonitors.monitoring.coreos.com
	-$(KUBECTL) delete crd prometheusrules.monitoring.coreos.com

update-helm-deps:
	(cd install/helm/open-match; helm dependencies update)

install-toolchain: build/toolchain/bin/protoc$(EXE_EXTENSION) build/toolchain/bin/protoc-gen-go$(EXE_EXTENSION) build/toolchain/bin/kubectl$(EXE_EXTENSION) build/toolchain/bin/helm$(EXE_EXTENSION) build/toolchain/bin/minikube$(EXE_EXTENSION) build/toolchain/bin/skaffold$(EXE_EXTENSION)  build/toolchain/bin/hugo$(EXE_EXTENSION) build/toolchain/python/

build/toolchain/bin/helm$(EXE_EXTENSION):
	mkdir -p $(TOOLCHAIN_BIN)
	mkdir -p $(TOOLCHAIN_DIR)/temp-helm
	cd $(TOOLCHAIN_DIR)/temp-helm && curl -Lo helm.tar.gz $(HELM_PACKAGE) && tar xvzf helm.tar.gz --strip-components 1
	mv $(TOOLCHAIN_DIR)/temp-helm/helm$(EXE_EXTENSION) $(TOOLCHAIN_BIN)/helm$(EXE_EXTENSION)
	mv $(TOOLCHAIN_DIR)/temp-helm/tiller$(EXE_EXTENSION) $(TOOLCHAIN_BIN)/tiller$(EXE_EXTENSION)
	rm -rf $(TOOLCHAIN_DIR)/temp-helm/

build/toolchain/bin/hugo$(EXE_EXTENSION):
	mkdir -p $(TOOLCHAIN_BIN)
	mkdir -p $(TOOLCHAIN_DIR)/temp-hugo
	cd $(TOOLCHAIN_DIR)/temp-hugo && curl -Lo hugo.tar.gz $(HUGO_PACKAGE) && tar xvzf hugo.tar.gz
	mv $(TOOLCHAIN_DIR)/temp-hugo/hugo$(EXE_EXTENSION) $(TOOLCHAIN_BIN)/hugo$(EXE_EXTENSION)
	rm -rf $(TOOLCHAIN_DIR)/temp-hugo/

build/toolchain/bin/minikube$(EXE_EXTENSION):
	mkdir -p $(TOOLCHAIN_BIN)
	curl -Lo minikube$(EXE_EXTENSION) $(MINIKUBE_PACKAGE)
	chmod +x minikube$(EXE_EXTENSION)
	mv minikube$(EXE_EXTENSION) $(TOOLCHAIN_BIN)/minikube$(EXE_EXTENSION)

build/toolchain/bin/kubectl$(EXE_EXTENSION):
	mkdir -p $(TOOLCHAIN_BIN)
	curl -Lo kubectl$(EXE_EXTENSION) $(KUBECTL_PACKAGE)
	chmod +x kubectl$(EXE_EXTENSION)
	mv kubectl$(EXE_EXTENSION) $(TOOLCHAIN_BIN)/kubectl$(EXE_EXTENSION)

build/toolchain/bin/skaffold$(EXE_EXTENSION):
	mkdir -p $(TOOLCHAIN_BIN)
	curl -Lo skaffold$(EXE_EXTENSION) $(SKAFFOLD_PACKAGE)
	chmod +x skaffold$(EXE_EXTENSION)
	mv skaffold$(EXE_EXTENSION) $(TOOLCHAIN_BIN)/skaffold$(EXE_EXTENSION)

push-helm: build/toolchain/bin/helm$(EXE_EXTENSION) build/toolchain/bin/kubectl$(EXE_EXTENSION)
	$(KUBECTL) create serviceaccount --namespace kube-system tiller
	$(HELM) init --service-account tiller --force-upgrade
	$(KUBECTL) create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
ifneq ($(strip $($(KUBECTL) get clusterroles | grep -i rbac)),)
	$(KUBECTL) patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'
endif
	echo "Waiting for Tiller to become ready..."
	$(KUBECTL) wait deployment --timeout=60s --for condition=available -l app=helm,name=tiller --namespace kube-system

delete-helm: build/toolchain/bin/helm$(EXE_EXTENSION) build/toolchain/bin/kubectl$(EXE_EXTENSION)
	-$(HELM) reset
	-$(KUBECTL) delete serviceaccount --namespace kube-system tiller
	-$(KUBECTL) delete clusterrolebinding tiller-cluster-rule
ifneq ($(strip $($(KUBECTL) get clusterroles | grep -i rbac)),)
	-$(KUBECTL) delete deployment --namespace kube-system tiller-deploy
endif
	echo "Waiting for Tiller to go away..."
	-$(KUBECTL) wait deployment --timeout=60s --for delete -l app=helm,name=tiller --namespace kube-system

auth-docker:
	gcloud $(GCP_PROJECT_FLAG) auth configure-docker

auth-gke-cluster:
	gcloud $(GCP_PROJECT_FLAG) container clusters get-credentials $(GKE_CLUSTER_NAME) $(GCP_LOCATION_FLAG)

create-gke-cluster:
	gcloud $(GCP_PROJECT_FLAG) container clusters create $(GKE_CLUSTER_NAME) $(GCP_LOCATION_FLAG) --machine-type n1-standard-4 --tags open-match

delete-gke-cluster:
	gcloud $(GCP_PROJECT_FLAG) container clusters delete $(GKE_CLUSTER_NAME) $(GCP_LOCATION_FLAG)

create-mini-cluster: build/toolchain/bin/minikube$(EXE_EXTENSION)
	$(MINIKUBE) start --memory 6144 --cpus 4 --disk-size 50g

delete-mini-cluster: build/toolchain/bin/minikube$(EXE_EXTENSION)
	$(MINIKUBE) delete

build/toolchain/python/:
	mkdir -p build/toolchain/python/
	virtualenv --python=python3 build/toolchain/python/
	# Hack to workaround some crazy bug in pip that's chopping off python executable's name.
	cd build/toolchain/python/bin && ln -s python3 pytho
	cd build/toolchain/python/ && source bin/activate && pip install grpcio-tools && deactivate

build/toolchain/bin/protoc$(EXE_EXTENSION):
	mkdir -p $(TOOLCHAIN_BIN)
	curl -o $(TOOLCHAIN_DIR)/protoc-temp.zip -L $(PROTOC_PACKAGE)
	(cd $(TOOLCHAIN_DIR); unzip -o protoc-temp.zip)
	rm $(TOOLCHAIN_DIR)/protoc-temp.zip $(TOOLCHAIN_DIR)/readme.txt

build/toolchain/bin/protoc-gen-go$(EXE_EXTENSION):
	$(GO) get github.com/golang/protobuf/protoc-gen-go
	$(GO) install github.com/golang/protobuf/protoc-gen-go
	mv $(GOPATH)/bin/protoc-gen-go$(EXE_EXTENSION) build/toolchain/bin/protoc-gen-go$(EXE_EXTENSION)

all-protos: internal/pb/backend.pb.go internal/pb/frontend.pb.go internal/pb/function.pb.go internal/pb/messages.pb.go internal/pb/mmlogic.pb.go mmlogic-simple-protos
internal/pb/%.pb.go: api/protobuf-spec/%.proto build/toolchain/bin/protoc$(EXE_EXTENSION) build/toolchain/bin/protoc-gen-go$(EXE_EXTENSION)
	$(PROTOC) $< \
	-I $(CURDIR) -I $(PROTOC_INCLUDES) \
	--go_out=plugins=grpc:$(GOPATH)/src

## Include structure of the protos needs to be called out do the dependency chain is run through properly.
internal/pb/backend.pb.go: internal/pb/messages.pb.go
internal/pb/frontend.pb.go: internal/pb/messages.pb.go
internal/pb/mmlogic.pb.go: internal/pb/messages.pb.go
internal/pb/function.pb.go: internal/pb/messages.pb.go

mmlogic-simple-protos: examples/functions/python3/mmlogic-simple/api/protobuf_spec/messages_pb2.py examples/functions/python3/mmlogic-simple/api/protobuf_spec/mmlogic_pb2.py

examples/functions/python3/mmlogic-simple/api/protobuf_spec/%_pb2.py: api/protobuf-spec/%.proto build/toolchain/python/
	source build/toolchain/python/bin/activate && python3 -m grpc_tools.protoc -I $(CURDIR) -I $(PROTOC_INCLUDES) --python_out=examples/functions/python3/mmlogic-simple/ --grpc_python_out=examples/functions/python3/mmlogic-simple/ $< && deactivate

internal/pb/%_pb2.py: api/protobuf-spec/%.proto build/toolchain/python/
	source build/toolchain/python/bin/activate && python3 -m grpc_tools.protoc -I $(CURDIR) -I $(PROTOC_INCLUDES) --python_out=$(CURDIR) --grpc_python_out=$(CURDIR) $< && deactivate

build:
	$(GO) build ./...

test:
	$(GO) test ./... -race

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

cmd/backendapi/backendapi: internal/pb/backend.pb.go
	cd cmd/backendapi; $(GO_BUILD_COMMAND)

cmd/frontendapi/frontendapi: internal/pb/frontend.pb.go
	cd cmd/frontendapi; $(GO_BUILD_COMMAND)

cmd/mmforc/mmforc:
	cd cmd/mmforc; $(GO_BUILD_COMMAND)

cmd/mmlogicapi/mmlogicapi: internal/pb/mmlogic.pb.go
	cd cmd/mmlogicapi; $(GO_BUILD_COMMAND)

examples/backendclient/backendclient: internal/pb/backend.pb.go
	cd examples/backendclient; $(GO_BUILD_COMMAND)

examples/evaluators/golang/simple/simple: internal/pb/messages.pb.go
	cd examples/evaluators/golang/simple; $(GO_BUILD_COMMAND)

examples/functions/golang/manual-simple/manual-simple: internal/pb/messages.pb.go
	cd examples/functions/golang/manual-simple; $(GO_BUILD_COMMAND)

test/cmd/clientloadgen/clientloadgen:
	cd test/cmd/clientloadgen; $(GO_BUILD_COMMAND)

test/cmd/frontendclient/frontendclient: internal/pb/frontend.pb.go internal/pb/messages.pb.go
	cd test/cmd/frontendclient; $(GO_BUILD_COMMAND)

build/archives/${NODEJS_PACKAGE_NAME}:
	mkdir -p build/archives/
	cd build/archives/ && curl -L -o ${NODEJS_PACKAGE_NAME} ${NODEJS_PACKAGE}

build/toolchain/nodejs/: build/archives/${NODEJS_PACKAGE_NAME}
	mkdir -p build/toolchain/nodejs/
	cd build/toolchain/nodejs/ && tar xjf ../../archives/${NODEJS_PACKAGE_NAME} --strip-components 1

install-npm: build/toolchain/nodejs/
	echo "{}" > package.json
	$(TOOLCHAIN_DIR)/nodejs/bin/npm install postcss-cli autoprefixer

build/site/: build/archives/govanityurls.zip build/toolchain/bin/hugo$(EXE_EXTENSION)
	rm -rf build/site/
	mkdir -p build/site/
	cd site/ && ../build/toolchain/bin/hugo$(EXE_EXTENSION) --enableGitInfo --config=config.toml --source . --destination $(BUILD_DIR)/site/public/
	-cp -f site/* $(BUILD_DIR)/site
	#cd $(BUILD_DIR)/site && "SERVICE=$(SERVICE) envsubst < app.yaml > .app.yaml"
	cp $(BUILD_DIR)/site/app.yaml $(BUILD_DIR)/site/.app.yaml

browse-site: build/site/
	cd $(BUILD_DIR)/site && dev_appserver.py .app.yaml

deploy-dev-site: build/site/
	cd $(BUILD_DIR)/site && gcloud $(OM_SITE_GCP_PROJECT_FLAG) app deploy .app.yaml --promote --version=$(VERSION_SUFFIX) --quiet

run-site: build/toolchain/bin/hugo$(EXE_EXTENSION)
	cd site/ && ../build/toolchain/bin/hugo$(EXE_EXTENSION) server --debug --watch --enableGitInfo . --bind 0.0.0.0 --port $(SITE_PORT) --disableFastRender

all: service-binaries client-binaries example-binaries
service-binaries: cmd/backendapi/backendapi cmd/frontendapi/frontendapi cmd/mmforc/mmforc cmd/mmlogicapi/mmlogicapi
client-binaries: examples/backendclient/backendclient test/cmd/clientloadgen/clientloadgen test/cmd/frontendclient/frontendclient
example-binaries: examples/evaluators/golang/simple/simple examples/functions/golang/manual-simple
presubmit: fmt vet build test

clean-site:
	rm -rf build/site/

clean-protos:
	rm -rf internal/pb/
	rm -rf api/protobuf_spec/

clean-binaries:
	rm -rf cmd/backendapi/backendapi
	rm -rf cmd/frontendapi/frontendapi
	rm -rf cmd/mmforc/mmforc
	rm -rf cmd/mmlogicapi/mmlogicapi
	rm -rf examples/backendclient/backendclient
	rm -rf examples/evaluators/golang/simple/simple
	rm -rf examples/functions/golang/manual-simple/manual-simple
	rm -rf test/cmd/clientloadgen/clientloadgen
	rm -rf test/cmd/frontendclient/frontendclient

clean-toolchain:
	rm -rf build/toolchain/

clean-nodejs:
	rm -rf build/toolchain/nodejs/
	rm -rf node_modules/
	rm -rf package.json

clean: clean-images clean-binaries clean-site clean-toolchain clean-protos clean-nodejs

run-backendclient: build/toolchain/bin/kubectl$(EXE_EXTENSION)
	$(KUBECTL) run om-backendclient --rm --restart=Never --image-pull-policy=Always -i --tty --image=$(REGISTRY)/openmatch-backendclient:$(TAG) --namespace=open-match $(KUBECTL_RUN_ENV)

run-frontendclient: build/toolchain/bin/kubectl$(EXE_EXTENSION)
	$(KUBECTL) run om-frontendclient --rm --restart=Never --image-pull-policy=Always -i --tty --image=$(REGISTRY)/openmatch-frontendclient:$(TAG) --namespace=open-match $(KUBECTL_RUN_ENV)

run-clientloadgen: build/toolchain/bin/kubectl$(EXE_EXTENSION)
	$(KUBECTL) run om-clientloadgen --rm --restart=Never --image-pull-policy=Always -i --tty --image=$(REGISTRY)/openmatch-clientloadgen:$(TAG) --namespace=open-match $(KUBECTL_RUN_ENV)

proxy-grafana: build/toolchain/bin/kubectl$(EXE_EXTENSION)
	echo "User: admin"
	echo "Password: openmatch"
	$(KUBECTL) port-forward --namespace open-match $(shell $(KUBECTL) get pod --namespace open-match --selector="app=grafana,release=open-match" --output jsonpath='{.items[0].metadata.name}') $(GRAFANA_PORT):3000 $(PORT_FORWARD_ADDRESS_FLAG)

proxy-prometheus: build/toolchain/bin/kubectl$(EXE_EXTENSION)
	$(KUBECTL) port-forward --namespace open-match $(shell $(KUBECTL) get pod --namespace open-match --selector="app=prometheus,component=server,release=open-match" --output jsonpath='{.items[0].metadata.name}') $(PROMETHEUS_PORT):9090 $(PORT_FORWARD_ADDRESS_FLAG)

proxy-dashboard: build/toolchain/bin/kubectl$(EXE_EXTENSION)
	$(KUBECTL) port-forward --namespace kube-system $(shell $(KUBECTL) get pod --namespace kube-system --selector="app=kubernetes-dashboard" --output jsonpath='{.items[0].metadata.name}') $(DASHBOARD_PORT):9090 $(PORT_FORWARD_ADDRESS_FLAG)

.PHONY: proxy-dashboard proxy-prometheus proxy-grafana clean clean-toolchain clean-binaries clean-protos presubmit test vet

