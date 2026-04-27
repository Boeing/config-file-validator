# This is a test justfile

set shell := ["bash", "-cu"]
set dotenv-load

version := "1.0.0"
export DATABASE_URL := "postgres://localhost/mydb"

# Build the project
[group: 'build']
build target='all':
    echo "Building {{target}} version {{version}}"

# Run tests
[private]
test *args: build
    cargo test {{args}}

# Deploy to production
[confirm]
deploy env='staging': build test
    echo "Deploying to {{env}}"

alias b := build
alias t := test

# Clean build artifacts
clean:
    rm -rf target/
    rm -rf dist/

greeting := if "hello" == "hello" { "match" } else { "no match" }

full_path := justfile_directory() / "src"
