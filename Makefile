
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= ""
NAME=canary-checker
YQ=$(shell readlink -f .bin/yq)
OS   ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/')
LD_FLAGS=-ldflags "-w -s -X \"main.version=$(VERSION_TAG)\""
ifeq ($(VERSION),)
  VERSION_TAG=$(shell git describe --abbrev=0 --tags || echo latest)
else
  VERSION_TAG=$(VERSION)
endif

# Image URL to use all building/pushing image targets
IMG_F ?= docker.io/flanksource/canary-checker-full:${VERSION_TAG}
IMG ?= docker.io/flanksource/canary-checker:${VERSION_TAG}

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif


all: manager


.PHONY: test
test: manifests generate fmt ginkgo
	ginkgo -vv -r  --cover  --keep-going --junit-report junit-report.xml --

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
deploy: manifests
	cd config && kustomize edit set image controller=${IMG}
	kustomize build config | kubectl apply -f -

static:  generate manifests .bin/yq
	kustomize build ./config | $(YQ) ea -P '[.] | sort_by(.metadata.name) | .[] | splitDoc' - > config/deploy/manifests.yaml
	kustomize build ./config/base | $(YQ) ea -P '[.] | sort_by(.metadata.name) | .[] | splitDoc' - > config/deploy/base.yaml

# Generate OpenAPI schema
.PHONY: gen-schemas
gen-schemas:
	cd hack/generate-schemas &&  go mod tidy && go run ./main.go

# Generate manifests e.g. CRD, RBAC etc.
manifests: .bin/controller-gen .bin/yq
	# For debugging
	$(YQ) -V

	schemaPath=.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties

	# Generate CRDS
	.bin/controller-gen crd paths="./api/..." output:stdout | $(YQ) ea -P '[.] | sort_by(.metadata.name) | .[] | splitDoc' - > config/deploy/crd.yaml

	$(MAKE) gen-schemas

	# Remove various nested properties within the CRD structure, such as checks, forEach, lookup, and properties
	# to reduce the CRD size.
	cd config/deploy && \
		$(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.checks.items.properties)' crd.yaml | \
		$(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.forEach.properties)' /dev/stdin | \
		$(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.lookup.properties)' /dev/stdin | \
		$(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.properties.items.properties.lookup.properties)' /dev/stdin | \
		$(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.forEach.properties)' /dev/stdin | \
		$(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.lookup.properties)' /dev/stdin | \
		$(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.checks.items.properties.inline.properties)' /dev/stdin | \
		$(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.properties.items.properties.lookup.properties)' /dev/stdin > crd.slim.yaml
	
	cd config/deploy && mv crd.slim.yaml crd.yaml

tidy:
	go mod tidy
	cd hack/generate-schemas && go mod tidy


# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	# go vet ./...

# Generate code
generate: .bin/controller-gen
	.bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

# Build the docker image
docker: docker-minimal docker-full

docker-full:
	docker build . -f build/full/Dockerfile -t ${IMG}

docker-minimal:
	docker build . -f build/minimal/Dockerfile -t ${IMG}

# Build the docker image
docker-dev: linux
	docker build ./ -f build/dev/Dockerfile -t ${IMG}

docker-push-%:
	docker build . -f build/full/Dockerfile -t ${IMG_F}
	docker build . -f build/minimal/Dockerfile -t ${IMG}
	docker tag $(IMG_F) $*/$(IMG_F)
	docker tag $(IMG) $*/$(IMG)
	docker push  $*/$(IMG_F)
	docker push  $*/$(IMG)
	kubectl set image deployment/$(NAME) $(NAME)=$*/$(IMG_F)

# Push the docker image
docker-push:
	docker push ${IMG_F}
	docker push ${IMG}

.PHONY: compress
compress: .bin/upx
	upx -5 ./.bin/$(NAME)_linux_amd64 ./.bin/$(NAME)_linux_arm64

.PHONY: compress-build
compress-build: .bin/upx
	upx -5 ./.bin/$(NAME) ./.bin/$(NAME).test

.PHONY: linux
linux:
	GOOS=linux GOARCH=amd64 go build  -o ./.bin/$(NAME)_linux_amd64 $(LD_FLAGS)  main.go
	GOOS=linux GOARCH=arm64 go build  -o ./.bin/$(NAME)_linux_arm64 $(LD_FLAGS)  main.go

.PHONY: darwin
darwin:
	GOOS=darwin GOARCH=amd64 go build -o ./.bin/$(NAME)_darwin_amd64 $(LD_FLAGS)  main.go
	GOOS=darwin GOARCH=arm64 go build -o ./.bin/$(NAME)_darwin_arm64 $(LD_FLAGS)  main.go

.PHONY: windows
windows:
	GOOS=windows GOARCH=amd64 go build -o ./.bin/$(NAME).exe $(LD_FLAGS)  main.go

.PHONY: binaries
binaries: linux darwin windows compress

.PHONY: release
release: binaries
	mkdir -p .release
	cd config/base && kustomize edit set image controller=${IMG}
	kustomize build config/ > .release/release.yaml
	cp .bin/canary-checker* .release/

.PHONY: lint
lint:
	golangci-lint run -v ./...

.PHONY: build-api-docs
build-api-docs:
	go run main.go docs  api/v1/*.go --output-file docs/api.md

.PHONY: dev
dev:
	go build -o ./.bin/$(NAME) -gcflags="all=-N -l" -v main.go

.PHONY: build
build:
	GOOS=$(OS) GOARCH=$(ARCH) go build -o ./.bin/$(NAME) $(LD_FLAGS)  main.go

.PHONY: test-build
test-build:
	go test  test/...  -o ./.bin/$(NAME).test $(LD_FLAGS)  main.go


.PHONY: fast-build
fast-build:
	go build --tags fast -o ./.bin/$(NAME) $(LD_FLAGS)  main.go

.PHONY: install
install:
	cp ./.bin/$(NAME) /usr/local/bin/

.PHONY: test-e2e
test-e2e: bin
	./test/e2e.sh


.bin/upx:
ifeq (, $(shell which upx))
ifeq ($(OS), darwin)
	brew install upx
	UPX=upx
else
	wget -nv -O upx.tar.xz https://github.com/upx/upx/releases/download/v3.96/upx-3.96-$(OS)_$(ARCH).xz
	tar xf upx.tar.xz
	mv upx-3.96-$(OS)_$(ARCH)/upx .bin
	rm -rf upx-3.96-$(OS)_$(ARCH)
	UPX=.bin/upx
endif
else
	UPX=$(shell which upx)
endif


.PHONY: ginkgo
ginkgo:
	go install github.com/onsi/ginkgo/v2/ginkgo

.bin/controller-gen: .bin
		GOBIN=$(PWD)/.bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0
		CONTROLLER_GEN=$(GOBIN)/controller-gen

.bin/yq: .bin
	curl -sSLo $(YQ) https://github.com/mikefarah/yq/releases/download/v4.40.5/yq_$(OS)_$(ARCH) && \
	chmod +x $(YQ)


.bin/go-junit-report: .bin
	GOBIN=$(PWD)/.bin GOFLAGS="-mod=mod"  go install github.com/jstemmer/go-junit-report

.bin/jmeter:
	curl -L https://mirrors.estointernet.in/apache//jmeter/binaries/apache-jmeter-5.4.3.tgz -o apache-jmeter-5.4.3.tgz && \
    tar xf apache-jmeter-5.4.3.tgz -C .bin/ && \
    rm apache-jmeter-5.4.3.tgz && \
		ln -s apache-jmeter-5.4.3/bin/jmeter .bin/jmeter

.bin/wait4x:
	wget -nv https://github.com/atkrad/wait4x/releases/download/v0.3.0/wait4x-$(OS)-$(ARCH) -O .bin/wait4x && \
  chmod +x .bin/wait4x

.bin/karina:
	curl -sSLo .bin/karina https://github.com/flanksource/karina/releases/download/v0.50.0/karina_$(OS)-$(ARCH) && \
	chmod +x .bin/karina

.bin:
	mkdir -p .bin

bin: .bin .bin/wait4x .bin/karina

# Generate all the resources and formats your code, i.e: CRDs, controller-gen, static
.PHONY: resources
resources: fmt static

.PHONY: chart
chart:
	helm dependency update ./chart
	helm package ./chart
