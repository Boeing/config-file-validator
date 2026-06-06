set unstable

# unexport
unexport HOME

# eager assignment
eager eagerly := "now"

# export eager combo (just doesn't support this syntax)
# export eager both := "exported-eager"

# Various attributes
[doc('Build the project')]
[group: 'ci']
[no-cd]
build:
    echo "building"

[script('bash')]
scripted:
    echo "hello"
    echo "world"

[confirm('Really delete?')]
danger:
    echo "danger"

[confirm]
also-danger:
    echo "also danger"

# Multiple attributes on one line
[private, group: 'internal']
multi-attr:
    echo "multi"

[positional-arguments]
pos-args arg:
    echo $1

# Dependency with arguments
deploy env:
    echo "deploying to {{env}}"

push: (deploy "production")
    echo "pushed"

# Dependency with expression argument
release version='latest': (deploy version)
    echo "releasing"

# Expression operators (unstable)
truthy := "hello" && "goodbye"
falsy := "" || "fallback"
chained := "a" && "b" || "c"

# f-string (format string)
name := "world"

# _private variable
_secret := "hidden"

# Nested modules would need subdirectories, tested in with_imports.just
