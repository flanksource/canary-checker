
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= ""
NAME=canary-checker
OS   = $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH = $(shell uname -m | sed 's/x86_64/amd64/')

ifeq ($(VERSION),)
  VERSION_TAG=$(shell git describe --abbrev=0 --tags --exact-match 2>/dev/null || echo latest)
else
  VERSION_TAG=$(VERSION)
endif

# Image URL to use all building/pushing image targets
IMG ?= docker.io/flanksource/canary-checker:${VERSION_TAG}

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/canary-checker main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install-crd: manifests
	kubectl apply -f config/deploy/crd.yaml


kind-install: docker-build
	kind load docker-image --name=kind-kind ${IMG}

# Uninstall CRDs from a cluster
uninstall: manifests
	kubectl delete -f config/deploy/crd.yaml

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: kustomize manifests
	cd config && $(KUSTOMIZE) edit set image controller=${IMG}
	kubectl $(KUSTOMIZE) config | kubectl apply -f -

static: kustomize manifests generate .bin/yq
	$(KUSTOMIZE) build ./config | $(YQ) eval -P '' - > config/deploy/manifests.yaml
	$(KUSTOMIZE) build ./config/base | $(YQ) eval -P '' - > config/deploy/base.yaml

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen .bin/yq
	$(CONTROLLER_GEN) crd:trivialVersions=false paths="./..." output:stdout > config/deploy/crd.yaml
	$(YQ) eval -i '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.junit.items.properties.spec.properties.containers.items.properties.ports.items.required=["containerPort", "protocol"]' config/deploy/crd.yaml
	$(YQ) eval -i '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.junit.items.properties.spec.properties.initContainers.items.properties.ports.items.required=["containerPort", "protocol"]' config/deploy/crd.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	# go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker:
	docker build . -t ${IMG} --build-arg=GITHUB_TOKEN=$(GITHUB_TOKEN)

# Build the docker image
docker-dev: linux
	docker build ./ -f ./Dockerfile.dev -t ${IMG}


docker-push-%:
	docker build ./ -f ./Dockerfile.dev -t ${IMG}
	docker tag $(IMG) $*/$(IMG)
	docker push  $*/$(IMG)
	kubectl set image deployment/$(NAME) $(NAME)=$*/$(IMG)

# Push the docker image
docker-push:
	docker push ${IMG}

.PHONY: compress
compress:
	# upx 3.95 has issues compressing darwin binaries - https://github.com/upx/upx/issues/301
	which upx 2>&1 >  /dev/null  || (sudo apt-get update && sudo apt-get install -y xz-utils && wget -nv -O upx.tar.xz https://github.com/upx/upx/releases/download/v3.96/upx-3.96-amd64_linux.tar.xz; tar xf upx.tar.xz; mv upx-3.96-amd64_linux/upx /usr/bin )
	upx -5 ./.bin/$(NAME)-amd64 ./.bin/$(NAME)_osx-amd64 ./.bin/$(NAME).exe


.PHONY: linux
linux: ui
	GOOS=linux GOARCH=amd64 go build -o ./.bin/$(NAME)-amd64 -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go

.PHONY: darwin-amd64
darwin-amd64: ui
	GOOS=darwin GOARCH=amd64 go build -o ./.bin/$(NAME)_osx-amd64 -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go

.PHONY: darwin-arm64
darwin-arm64: ui
	GOOS=darwin GOARCH=arm64 go build -o ./.bin/$(NAME)_osx-arm64 -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go

.PHONY: windows
windows: ui
	GOOS=windows GOARCH=amd64 go build -o ./.bin/$(NAME).exe -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go

.PHONY: release
release: ui kustomize linux darwin-amd64 darwin-arm64 windows compress
	cd config/base && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/ > ./.bin/release.yaml

.PHONY: lint
lint:
	golangci-lint run

.PHONY: serve-docs
serve-docs:
	docker run --rm -it -p 8000:8000 -v $(PWD):/docs -w /docs squidfunk/mkdocs-material

.PHONY: build-api-docs
build-api-docs:
	go run main.go docs api  api/v1/checks.go  > docs/reference.md
	mkdir -p docs/cli
	go run main.go docs cli "docs/cli"

.PHONY: build-docs
build-docs:
	which mkdocs 2>&1 > /dev/null || pip install mkdocs mkdocs-material
	mkdocs build -d build/docs

.PHONY: deploy-docs
deploy-docs:
	which netlify 2>&1 > /dev/null || sudo npm install -g netlify-cli
	netlify deploy --site cfe8c6b7-79b7-4a88-9e13-ff792126717f --prod --dir build/docs

.PHONY: ui
ui:
	cd ui && npm ci && npm run build

.PHONY: build
build:
	go build -o ./.bin/$(NAME) -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go

.PHONY: install
install: build
	cp ./.bin/$(NAME) /usr/local/bin/

.PHONY: test-e2e
test-e2e: bin
	./test/e2e.sh

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif


# find or download kustomize if necessary
kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v4@v4.0.3 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

.bin/go-junit-report:
	mkdir -p .bin
	# set -e ;\
	_TMP_DIR=$$(mktemp -d) ;\
	cd $$_TMP_DIR ;\
	go mod init tmp   ;\
	go get github.com/jstemmer/go-junit-report   ;\
	rm -rf $$_TMP_DIR
	cp $(GOBIN)/go-junit-report .bin/go-junit-report ;\

.bin/jmeter:
	curl -L https://mirrors.estointernet.in/apache//jmeter/binaries/apache-jmeter-5.4.1.tgz -o apache-jmeter-5.4.1.tgz && \
    sudo tar xf apache-jmeter-5.4.1.tgz -C .bin/ && \
    rm apache-jmeter-5.4.1.tgz && \
		ln -s apache-jmeter-5.4.1/bin/jmeter .bin/jmeter

.bin/restic:
	wget -nv  https://github.com/restic/restic/releases/download/v0.12.0/restic_0.12.0_$(OS)_$(ARCH).bz2 -O .bin/restic.bz2 && \
    bunzip2  .bin/restic.bz2 && \
    chmod +x .bin/restic

.bin/wait4x:
	wget -nv https://github.com/atkrad/wait4x/releases/download/v0.3.0/wait4x-$(OS)-$(ARCH) -O .bin/wait4x && \
  chmod +x .bin/wait4x

.bin/karina:
	wget -q https://github.com/flanksource/karina/releases/download/v0.50.0/karina_$(OS)-$(ARCH) -O .bin/karina && \
	chmod +x .bin/karina

.bin/yq:
	curl -sSLo .bin/yq https://github.com/mikefarah/yq/releases/download/v4.9.6/yq_$(OS)_$(ARCH) && chmod +x .bin/yq
YQ = $(realpath ./.bin/yq)

.bin:
	mkdir -p .bin

bin: .bin .bin/wait4x .bin/yq .bin/karina .bin/go-junit-report .bin/restic .bin/jmeter

# Generate all the resources and formats your code, i.e: CRDs, controller-gen, static
.PHONY: resources
resources: fmt static manifests
