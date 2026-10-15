BINARY := kube-node-doctor
IMAGE  := ashsharon/kube-node-doctor

.PHONY: build test vet fmt lint docker-build run clean

build:
	go build -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	go test ./... -cover

vet:
	go vet ./...

fmt:
	gofmt -l .

lint: fmt vet test

docker-build:
	docker build -t $(IMAGE):local .

run: build
	./bin/$(BINARY) $(ARGS)

clean:
	rm -rf bin
