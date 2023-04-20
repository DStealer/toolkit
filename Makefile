
.PHONY: build-linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64  go build -o dist/tk-linux -v -gcflags=all="-N -l"

.PHONY: build-win
build-win:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build -o dist/tk-win.exe -v -gcflags=all="-N -l"

.PHONY: build-darwin
build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64  go build -o dist/tk-darwin -v -gcflags=all="-N -l"

.PHONY: docker
docker:
	docker run --rm -e "GOPROXY=https://goproxy.io" -e "GO111MODULE=auto" -v $(shell pwd):/srv -w /srv amd64/golang:1.19 go build -o tk -v -gcflags=all="-N -l"
