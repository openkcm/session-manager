SERVICE_NAME = session-manager

.PHONY: build
build: clean
	go build -o $(SERVICE_NAME) ./cmd/$(SERVICE_NAME)
	sha256sum $(SERVICE_NAME)

.PHONY: clean
clean:
	rm -f cover.out cover.html $(SERVICE_NAME)
	rm -rf cover/

.PHONY: lint
lint:
	golangci-lint run -v --fix ./...

.PHONY: reuse-lint
reuse-lint:
	docker run --rm --volume $(PWD):/data fsfe/reuse lint
