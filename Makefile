
.PHONY: build-linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64  go build -o tk-linux -v -gcflags=all="-N -l"

.PHONY: build-win64
build-win64:
	CGO_ENABLED=0 GOOS=window GOARCH=amd64  go build -o tk-win.exe -v -gcflags=all="-N -l"

.PHONY: build-darwin
build-win64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64  go build -o tk-darwin -v -gcflags=all="-N -l"

.PHONY: docker
docker:
	docker run --rm -e "GOPROXY=https://goproxy.io" -e "GO111MODULE=auto" -v $(shell pwd):/srv -w /srv amd64/golang:1.19 go build -o tk -v -gcflags=all="-N -l"
