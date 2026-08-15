VERSION ?= 0.27
VERSION_CODE ?= 27
LDFLAGS := -s -w -X github.com/thowilabs/pangolite/internal/app.Version=$(VERSION) -X github.com/thowilabs/pangolite/internal/app.VersionCode=$(VERSION_CODE)

.PHONY: tidy test build run

tidy:
	go mod tidy

test:
	go test -timeout 2m ./...

build:
	CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="$(LDFLAGS)" -o bin/pangolite ./cmd/pangolite
	CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="$(LDFLAGS)" -o bin/pangolite-client ./cmd/pangolite-client

run:
	PANGOLITE_ADDR=127.0.0.1:2424 PANGOLITE_DATA=./data/pangolite.db go run ./cmd/pangolite serve
