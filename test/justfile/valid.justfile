# Sample justfile for testing
set shell := ["bash", "-c"]

# Default recipe
default:
    @echo "Hello, World!"

# Build recipe
build:
    @echo "Building project..."
    cargo build --release

# Test recipe
test:
    @echo "Running tests..."
    cargo test

# Clean recipe
clean:
    @echo "Cleaning up..."
    cargo clean

# Variables
version := "1.0.0"
author := "Test Author"

# Recipe with parameters
greet name:
    @echo "Hello, {{name}}!"

# Recipe with dependencies
install: build
    @echo "Installing version {{version}}..."
    cargo install --path .
