
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= ""
NAME=canary-checker
OS   = $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH = $(shell uname -m | sed 's/x86_64/amd64/')
KUSTOMIZE=$(PWD)/.bin/kustomize

ifeq ($(VERSION),)
  VERSION_TAG=$(shell git describe --abbrev=0 --tags || echo latest)
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
deploy: .bin/kustomize manifests
	cd config && .bin/kustomize edit set image controller=${IMG}
	kubectl $(KUSTOMIZE) config | kubectl apply -f -

static: .bin/kustomize generate manifests .bin/yq
	.bin/kustomize build ./config | $(YQ) ea -P '[.] | sort_by(.metadata.name) | .[] | splitDoc' - > config/deploy/manifests.yaml
	.bin/kustomize build ./config/base | $(YQ) ea -P '[.] | sort_by(.metadata.name) | .[] | splitDoc' - > config/deploy/base.yaml

# Generate manifests e.g. CRD, RBAC etc.
manifests: .bin/controller-gen .bin/yq
	schemaPath=.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties
	.bin/controller-gen crd paths="./api/..." output:stdout | $(YQ) ea -P '[.] | sort_by(.metadata.name) | .[] | splitDoc' - > config/deploy/crd.yaml
	cd hack/generate-schemas && go run ./main.go
	cd config/deploy && $(YQ) ea  'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.checks.items.properties)' crd.yaml | $(YQ) ea  'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.forEach.properties)' /dev/stdin  | $(YQ) ea  'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.lookup.properties)'  /dev/stdin | $(YQ) ea  'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.properties.items.properties.lookup.properties)' /dev/stdin | $(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.forEach.properties)' /dev/stdin |  $(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.lookup.properties)' /dev/stdin | $(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.checks.items.properties.inline.properties)' /dev/stdin | $(YQ) ea 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.components.items.properties.properties.items.properties.lookup.properties)' /dev/stdin > crd.slim.yaml
	cd config/deploy && mv crd.slim.yaml crd.yaml

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
docker:
	docker build . -t ${IMG}

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
compress: .bin/upx
	upx -5 ./.bin/$(NAME)_linux_amd64 ./.bin/$(NAME)_linux_arm64 ./.bin/$(NAME)_darwin_amd64 ./.bin/$(NAME)_darwin_arm64 ./.bin/$(NAME).exe

.PHONY: linux
linux:
	GOOS=linux GOARCH=amd64 go build  -o ./.bin/$(NAME)_linux_amd64 -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go
	GOOS=linux GOARCH=arm64 go build  -o ./.bin/$(NAME)_linux_arm64 -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go

.PHONY: darwin
darwin:
	GOOS=darwin GOARCH=amd64 go build -o ./.bin/$(NAME)_darwin_amd64 -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go
	GOOS=darwin GOARCH=arm64 go build -o ./.bin/$(NAME)_darwin_arm64 -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go

.PHONY: windows
windows:
	GOOS=windows GOARCH=amd64 go build -o ./.bin/$(NAME).exe -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go

.PHONY: binaries
binaries: linux darwin windows compress

.PHONY: release
release: .bin/kustomize binaries
	mkdir -p .release
	cd config/base && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/ > .release/release.yaml
	cp .bin/canary-checker* .release/

.PHONY: lint
lint:
	golangci-lint run

.PHONY: serve-docs
serve-docs:
	docker run --rm -it -p 8000:8000 -v $(PWD):/docs -w /docs squidfunk/mkdocs-material

.PHONY: build-api-docs
build-api-docs:
	go run main.go docs  api/v1/*.go --output-file docs/api.md

.PHONY: build-docs
build-docs:
	pip3 install $(MKDOCS_INSIDERS)
	mkdocs build -d build/docs

.PHONY: deploy-docs
deploy-docs:
	which netlify 2>&1 > /dev/null || sudo npm install -g netlify-cli
	netlify deploy --site cfe8c6b7-79b7-4a88-9e13-ff792126717f --prod --dir build/docs

.PHONY: dev
dev:
	go build -o ./.bin/$(NAME) -gcflags="all=-N -l" -v main.go

.PHONY: build
build:
	go build -o ./.bin/$(NAME) -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go

.PHONY: fast-build
fast-build:
	go build --tags fast -o ./.bin/$(NAME) -ldflags "-X \"main.version=$(VERSION_TAG)\""  main.go

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

.bin/controller-gen: .bin
		GOBIN=$(PWD)/.bin go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.11.1
		CONTROLLER_GEN=$(GOBIN)/controller-gen

.bin/kustomize: .bin
	curl -L https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv4.3.0/kustomize_v4.3.0_$(OS)_$(ARCH).tar.gz -o kustomize.tar.gz && \
	tar xf kustomize.tar.gz -C .bin/ && \
	rm kustomize.tar.gz

.bin/go-junit-report: .bin
	GOBIN=$(PWD)/.bin GOFLAGS="-mod=mod"  go install github.com/jstemmer/go-junit-report

.bin/jmeter:
	curl -L https://mirrors.estointernet.in/apache//jmeter/binaries/apache-jmeter-5.4.3.tgz -o apache-jmeter-5.4.3.tgz && \
    tar xf apache-jmeter-5.4.3.tgz -C .bin/ && \
    rm apache-jmeter-5.4.3.tgz && \
		ln -s apache-jmeter-5.4.3/bin/jmeter .bin/jmeter

.bin/restic:
	wget -nv  https://github.com/restic/restic/releases/download/v0.12.1/restic_0.12.1_$(OS)_$(ARCH).bz2 -O .bin/restic.bz2 && \
    bunzip2  .bin/restic.bz2 && \
    chmod +x .bin/restic

.bin/wait4x:
	wget -nv https://github.com/atkrad/wait4x/releases/download/v0.3.0/wait4x-$(OS)-$(ARCH) -O .bin/wait4x && \
  chmod +x .bin/wait4x

.bin/karina:
	wget -q https://github.com/flanksource/karina/releases/download/v0.50.0/karina_$(OS)-$(ARCH) -O .bin/karina && \
	chmod +x .bin/karina

.bin/yq: .bin
	curl -sSLo .bin/yq https://github.com/mikefarah/yq/releases/download/v4.16.1/yq_$(OS)_$(ARCH) && chmod +x .bin/yq
YQ = $(realpath ./.bin/yq)

.PHONY: telepresence
telepresence:
ifeq (, $(shell which telepresence))
ifeq ($(OS), darwin)
	brew install --cask macfuse
	brew install datawire/blackbird/telepresence-legacy
else
	sudo curl -fL https://app.getambassador.io/download/tel2/linux/amd64/latest/telepresence -o /usr/local/bin/telepresence
	sudo chmod a+x /usr/local/bin/telepresence
endif
endif

.bin:
	mkdir -p .bin

.bin/octopilot:
	curl -sSLo .bin/octopilot https://github.com/dailymotion-oss/octopilot/releases/download/v1.0.7/octopilot_1.0.7_$(OS)_$(ARCH) && \
	chmod +x .bin/octopilot

bin: .bin .bin/wait4x .bin/yq .bin/karina .bin/go-junit-report .bin/restic .bin/jmeter telepresence .bin/octopilot .bin/kustomize


# Generate all the resources and formats your code, i.e: CRDs, controller-gen, static
.PHONY: resources
resources: fmt static manifests

.PHONY: chart
chart:
	helm dependency update ./chart
	helm package ./chart
