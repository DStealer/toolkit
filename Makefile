
.PHONY: build
build:
	go build -o tk -v

.PHONY: docker
docker:
	docker run --rm -e "GOPROXY=https://goproxy.io" -e "GO111MODULE=auto" -v $(shell pwd):/srv -w /srv amd64/golang:1.16 go build -o tk -v -gcflags=all="-N -l"