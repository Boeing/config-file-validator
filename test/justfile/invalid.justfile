# Invalid justfile with syntax errors
set shell := ["bash", "-c"]

# Default recipe
default
    @echo "Hello, World!"  # Missing colon after recipe name

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

# Invalid variable assignment
123invalid := "invalid"  # Invalid variable name

# Recipe with parameters
greet name:
    @echo "Hello, {{name}}!"

# Recipe with dependencies
install: build
    @echo "Installing version {{version}}..."
    cargo install --path .
