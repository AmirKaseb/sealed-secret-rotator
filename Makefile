# Makefile for SealedSecret Rotator

# Define the binary name
BINARY_NAME = sealedsecret-rotator

# Default target is to build the project
all: build

# Build the binary
build:
	@echo "Building the project..."
	go build -o $(BINARY_NAME) ./cmd/sealed-secrets-rotator.go

# Clean the compiled binary
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)

# Install the binary to /usr/local/bin
install: build
	@echo "Installing the binary..."
	sudo mv $(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)

# Show help
help:
	@echo "Usage:"
	@echo "  make build     - Build the project"
	@echo "  make clean     - Clean up the project"
	@echo "  make install   - Build and install the binary"
	@echo "  make help      - Show this help message"
