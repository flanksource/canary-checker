NAME=canary-checker
LD_FLAGS=-ldflags "-w -s -X \"main.version=$(VERSION_TAG)\""

ifeq ($(VERSION),)
  VERSION_TAG=$(shell git describe --abbrev=0 --tags || echo latest)
else
  VERSION_TAG=$(VERSION)
endif

.PHONY: build
build:
	go test  ./...  -o ../.bin/$(NAME).test -c $(LD_FLAGS)  main.go

