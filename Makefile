.PHONY: build-server build-lambda package-lambda clean test

# Build variables
BINARY_SERVER = dns-api-go
BINARY_LAMBDA = bootstrap
LAMBDA_ZIP = function.zip

# Build the ECS server binary
build-server:
	go build -o $(BINARY_SERVER) ./cmd/server/

# Build the Lambda binary (Linux amd64 for AWS Lambda)
build-lambda:
	GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o $(BINARY_LAMBDA) ./cmd/lambda/

# Package Lambda deployment zip with bootstrap binary and config files
package-lambda: build-lambda
	zip $(LAMBDA_ZIP) $(BINARY_LAMBDA) config/cidr_*.json

# Run all tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f $(BINARY_SERVER) $(BINARY_LAMBDA) $(LAMBDA_ZIP)

