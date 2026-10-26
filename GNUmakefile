default: build generate

gql_dest = graphql/generated.go
gql_src = graphql/schema.graphql graphql/genqlient.graphql graphql/genqlient.yaml

build: $(gql_dest)
	go build -v ./...

$(gql_dest): $(gql_src)
	go generate

generate: $(gql_dest)
	cd tools; go generate ./...

.PHONY: clean
clean:
	rm -rf docs $(gql_dest)

# Run acceptance tests
.PHONY: test
test:
	TF_ACC=1 go test -v -cover ./internal/provider/ -count=1 -parallel=8
