default: build generate

build: generate
	go build -v ./...

generate:
	cd tools; go generate ./...

# Run acceptance tests
.PHONY: test
test:
	TF_ACC=1 go test -v -cover ./internal/provider/ -count=1 -parallel=8
