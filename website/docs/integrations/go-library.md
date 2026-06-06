---
---

# Go Library

The validator is available as a Go package. Embed validation in your own tools using the `cli` and `finder` packages.

[![Go Reference](https://pkg.go.dev/badge/github.com/Boeing/config-file-validator/v2.svg)](https://pkg.go.dev/github.com/Boeing/config-file-validator/v2)

## Install

```shell
go get github.com/Boeing/config-file-validator/v2
```

:::note
The `cli` package imports all validators, which pulls in dependencies for every supported file type. If you only need specific validators, import `pkg/validator` directly and use individual validators like `validator.JSONValidator{}` or `validator.YAMLValidator{}`.
:::

## Default configuration

Validates all supported config files in the current directory and prints results to stdout:

```go
package main

import (
	"os"
	"log"

	"github.com/Boeing/config-file-validator/v2/pkg/cli"
)

func main() {
	cfv := cli.Init()
	exitStatus, err := cfv.Run()
	if err != nil {
		log.Printf("Errors occurred during execution: %v", err)
	}
	os.Exit(exitStatus)
}
```

## Custom search paths

```go
fileSystemFinder := finder.FileSystemFinderInit(
	finder.WithPathRoots("/path/to/search", "/another/path"),
)

cfv := cli.Init(
	cli.WithFinder(fileSystemFinder),
)
```

## Custom reporter

Output JSON to stdout:

```go
jsonReporter := reporter.NewJSONReporter("")

cfv := cli.Init(
	cli.WithFinder(fileSystemFinder),
	cli.WithReporters(jsonReporter),
)
```

Pass a directory path to `NewJSONReporter` to write to a file instead.

## Finder options

```go
fileSystemFinder := finder.FileSystemFinderInit(
	finder.WithPathRoots("."),
	finder.WithExcludeDirs([]string{"node_modules", "vendor"}),
	finder.WithExcludeFileTypes([]string{"csv"}),
	finder.WithDepth(3),
)
```

## CLI options

```go
cfv := cli.Init(
	cli.WithFinder(fileSystemFinder),
	cli.WithReporters(jsonReporter),
	cli.WithQuiet(true),
	cli.WithGroupOutput([]string{"pass-fail"}),
	cli.WithRequireSchema(true),
	cli.WithNoSchema(false),
	cli.WithSchemaMap(map[string]string{
		"**/package.json": "schemas/package.schema.json",
	}),
)
```

## SchemaStore

Enable automatic schema lookup:

```go
import "github.com/Boeing/config-file-validator/v2/pkg/schemastore"

store, err := schemastore.OpenEmbedded()
if err != nil {
	log.Fatal(err)
}

cfv := cli.Init(
	cli.WithSchemaStore(store),
)
```

For offline or restricted environments, use a local SchemaStore clone:

```go
store, err := schemastore.Open("/path/to/schemastore")
if err != nil {
	log.Fatal(err)
}

cfv := cli.Init(
	cli.WithSchemaStore(store),
)
```

## Full API reference

See the [Go package documentation](https://pkg.go.dev/github.com/Boeing/config-file-validator/v2) for all exported types and functions.
