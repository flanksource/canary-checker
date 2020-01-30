
default: build
NAME:=canary-checker

ifeq ($(VERSION),)
VERSION := v$(shell git describe --tags --exclude "*-g*" ) built $(shell date)
endif

.PHONY: setup
setup:
	which github-release 2>&1 > /dev/null || go get github.com/aktau/github-release

.PHONY: linux
linux:
	GOOS=linux go build -o ./.bin/$(NAME) -ldflags "-X \"main.version=$(VERSION)\""  main.go

.PHONY: darwin
darwin:
	GOOS=darwin go build -o ./.bin/$(NAME)_osx -ldflags "-X \"main.version=$(VERSION)\""  main.go

.PHONY: compress
compress:
	which upx 2>&1 >  /dev/null  || (sudo apt-get update && sudo apt-get install -y upx-ucl)
	upx ./.bin/$(NAME) ./.bin/$(NAME)_osx

.PHONY: install
install:
	cp ./.bin/$(NAME) /usr/local/bin/

.PHONY: image
image:
	docker build -t $(NAME) --build-arg VERSION="$(VERSION)" -f Dockerfile .