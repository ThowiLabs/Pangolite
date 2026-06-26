.PHONY: tidy test build run

tidy:
	go mod tidy

test:
	go test ./...

build:
	CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w' -o bin/pangolite ./cmd/pangolite
	CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w' -o bin/pangolite-client ./cmd/pangolite-client

run:
	PANGOLITE_ADDR=127.0.0.1:2424 PANGOLITE_DATA=./data/pangolite.db go run ./cmd/pangolite serve
