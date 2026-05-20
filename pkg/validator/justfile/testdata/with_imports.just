import 'imported.just'

mod tools

build:
    echo "building"

check: build lint
    echo "all checks passed"
