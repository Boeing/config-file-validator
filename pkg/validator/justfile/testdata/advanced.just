#!/usr/bin/env just --justfile

set shell := ["bash", "-cu"]
set dotenv-load
set export
set positional-arguments

# Variables
version := "1.0.0"
arch := arch()
os_name := os()
build_dir := justfile_directory() / "build"
export DATABASE_URL := "postgres://localhost/mydb"

# Conditional assignment
greeting := if version == "1.0.0" { "stable" } else { "dev" }
nested := if arch == "x86_64" { "64bit" } else if arch == "aarch64" { "arm64" } else { "other" }

# String types
raw := 'hello\nworld'
escaped := "hello\nworld"
backtick_val := `echo hello`

# Recipes with various parameter types
build target='all' mode="debug":
    echo "Building {{target}} in {{mode}}"
    echo "Version: {{version}}"

# Variadic parameters
test +args:
    cargo test {{args}}

test-optional *args:
    cargo test {{args}}

# Export parameter
run-with-env $ENV_VAR:
    echo $ENV_VAR

# Dependencies
deploy env='staging': build (test "integration")
    echo "Deploying to {{env}}"

# Subsequent dependencies
all: build && deploy
    echo "All done"

# Quiet recipe
@quiet-recipe:
    echo "this is quiet"

# Recipe with error suppression in body
error-suppressed:
    -might-fail

# Recipe line prefixes
mixed-prefixes:
    normal line
    @quiet line
    -error line
    @-both line
    -@also-both line

# Attributes
[private]
[group: 'ci']
ci: build deploy

[confirm]
[no-cd]
dangerous:
    rm -rf /

[linux]
[macos]
unix-only:
    uname -a

# Aliases
alias b := build
alias t := test
alias d := deploy

# Settings
# Unexport a non-exported variable
unexport SOME_VAR

# Function calls in expressions
home := env_var("HOME")
with_default := env_var_or_default("MISSING", "fallback")
replaced := replace(version, ".", "-")

# Complex expressions
full := "prefix-" + version + "-" + arch
path := build_dir / "output" / arch
