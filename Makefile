GOPATH ?= $(shell go env GOPATH)
GO111MODULE:=auto
export GO111MODULE
ifeq "$(GOPATH)" ""
  $(error Please set the environment variable GOPATH before running `make`)
endif
PATH := ${GOPATH}/bin:$(PATH)
GCFLAGS=-gcflags "all=-trimpath=${GOPATH} -N -l"

VERSION_TAG := $(shell git describe --tags --always)
VERSION_VERSION := $(VERSION_TAG) $(shell git log --date=iso --pretty=format:"%cd" -1)
VERSION_COMPILE := $(shell date +"%F %T %z") by $(shell go version)
VERSION_BRANCH  := $(shell git rev-parse --abbrev-ref HEAD)
VERSION_GIT_DIRTY := $(shell git diff --no-ext-diff 2>/dev/null | wc -l | awk '{print $1}')

LDFLAGS=-ldflags="-s -w -X 'github.com/dstealer/devops/cmd.Version=$(VERSION_VERSION)' -X 'github.com/dstealer/devops/cmd.Compile=$(VERSION_COMPILE)' -X 'github.com/dstealer/devops/cmd.Branch=$(VERSION_BRANCH)' -X 'github.com/dstealer/devops/cmd.GitDirty=$(VERSION_GIT_DIRTY)'"

.PHONY: build-linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64  go build -v ${LDFLAGS} ${GCFLAGS} -o dist/tk-linux

.PHONY: build-win
build-win:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build -v ${LDFLAGS} ${GCFLAGS} -o dist/tk-win.exe

.PHONY: build-darwin
build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64  go build -v ${LDFLAGS} ${GCFLAGS} -o dist/tk-darwin

.PHONY: docker
docker:
	docker run --rm -e "GOPROXY=https://goproxy.io" -e "GO111MODULE=auto" -v $(shell pwd):/srv -w /srv amd64/golang:1.19 go build -v ${LDFLAGS} ${GCFLAGS} -o tk
