# Patterns from the just README
# https://github.com/casey/just/blob/master/README.md

set shell := ["bash", "-cu"]
set unstable
set allow-duplicate-recipes
set allow-duplicate-variables
set dotenv-load
set export
set positional-arguments

# String operators
foo := '' && 'goodbye'
bar := 'hello' && 'goodbye'
baz := '' || 'goodbye'
qux := 'hello' || 'goodbye'

# Shell-expanded strings (just evaluates these at dump time, so use
# a variable that's likely to exist for conformance testing)
foobar := x'echo hello'

# f-string (just evaluates interpolations at dump time, so this is
# tested for parsing but not dump conformance)
# message := f'Hello, {{name}}!'
name := "world"

# Backtick
foos := `echo "foo"`

# Conditional
os := if os() == "macos" { "mac" } else if os() == "linux" { "linux" } else { "other" }

# Concatenation and path join
full := "prefix-" + name
path := justfile_directory() / "src" / "main"

# Multi-line raw string
long := '
  this is a
  multi-line raw string
'

# Various recipe forms
recipe-name:
  echo 'This is a recipe!'

another-recipe:
  @echo 'This is another recipe.'

default: lint build test

build project='main':
  echo "Building {{project}}"

test: build
  echo 'Testing!'

lint:
  echo 'Linting!'

# Quiet recipe
@quiet-recipe:
  pwd

# No-cd attribute
[no-cd]
@bar:
  pwd

# Working directory attribute
[working-directory: 'bar']
@wd-recipe:
  pwd

# Alias
alias b := build
alias t := test

# Positional arguments attribute
[positional-arguments]
@pos-args arg:
  echo $0
  echo $1

# Variadic
@variadic-star *args='':
  bash -c 'while (( "$#" )); do echo - $1; shift; done' -- "$@"

@variadic-plus +args:
  echo {{args}}

# Export parameter
run $ENV_VAR:
  echo $ENV_VAR

# Dependencies with arguments
push: (deploy "production")
  echo 'Pushed!'

deploy env:
  echo "Deploying to {{env}}"

# Subsequent dependencies
all: build && test lint
  echo 'All done!'

# Confirm attribute
[confirm]
dangerous:
  echo 'doing something dangerous'

[confirm('Are you sure?')]
also-dangerous:
  echo 'also dangerous'

# Group attribute
[group: 'ci']
ci-build:
  echo 'CI build'

[group('deploy')]
ci-deploy:
  echo 'CI deploy'

# Private attribute
[private]
helper:
  echo 'helper'

# Multiple attributes on one line
[private, group: 'internal']
internal-helper:
  echo 'internal'

# Script attribute
[script('bash')]
script-recipe:
  echo "this is bash"
  echo "still bash"

# Doc attribute
[doc('Build the project for release')]
release:
  echo 'releasing'

# Recipe with shebang
polyglot:
  #!/usr/bin/env python3
  print('Hello from python!')

# Conditional in recipe body
greet name:
  echo {{ if name == "world" { "Hello, world!" } else { "Hello, " + name + "!" } }}

# Function calls
home := env_var("HOME")
with_default := env_var_or_default("MISSING", "fallback")
arch_val := arch()
os_val := os()
invocation := invocation_directory()
just_dir := justfile_directory()
just_file := justfile()

# Nested function calls
replaced := replace(replace("a.b.c", ".", "-"), "-", "_")

# Complex expression
complex := "v" + replace(trim("  1.0.0  "), ".", "-")
