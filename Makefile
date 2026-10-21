default: build

# Terraform finds the provider binary by its name, terraform-provider-<type>
BINARY = terraform-provider-s4communication$(shell go env GOEXE)

# Build the provider
build:
	go build -o $(BINARY)

# Install the provider into $GOPATH/bin, for use with a dev_overrides block
# in ~/.terraformrc (see DEVELOPER.md). Plain "go install" would name the
# binary after the Go module, which Terraform does not find.
install:
	go build -o $(shell go env GOPATH)/bin/$(BINARY)

# Generate the documentation in docs/
generate:
	go generate ./...

# Export the system configured in S4_HOST, S4_USERNAME and S4_PASSWORD as
# Terraform configuration in ./exported
export:
	go run ./cmd/s4export -out exported

# Run all tests. The resource tests run Terraform against an in-memory SAP
# mock; they need the terraform CLI on PATH (or TF_ACC_TERRAFORM_PATH).
test:
	go test -v ./...

# Format Go code
fmt:
	go fmt ./...

# Lint the code
lint:
	go vet ./...

# Clean build artifacts
clean:
	rm -f $(BINARY)

.PHONY: build install generate export test fmt lint clean
