# Go API

The config-file-validator can be called programmatically from within a Go program through the `cli` package.

[![Go Reference](https://pkg.go.dev/badge/github.com/Boeing/config-file-validator/v2.svg)](https://pkg.go.dev/github.com/Boeing/config-file-validator/v2)

> **Note:** The `cli` package imports all validators, which pulls in dependencies for every supported file type. If you only need specific validators, import `pkg/validator` directly and use individual validators like `validator.JSONValidator{}` or `validator.YAMLValidator{}` without the `cli` package.

## Default configuration

The default configuration will perform the following actions:

* Search for all supported configuration file types in the current directory and its subdirectories 
* Uses the default reporter which will output validation results to console (stdout)

```go
package main

import (
	"os"
	"log"

	"github.com/Boeing/config-file-validator/v2/pkg/cli"
)

func main() {

	// Initialize the CLI
	cfv := cli.Init()
	
	// Run the config file validation
	exitStatus, err := cfv.Run()
	if err != nil {
	  log.Printf("Errors occurred during execution: %v", err)
	}
	
	os.Exit(exitStatus)
}
```

## Custom Search Paths

The below example will search the provided search paths.

```go
package main

import (
	"os"
	"log"

	"github.com/Boeing/config-file-validator/v2/pkg/cli"
	"github.com/Boeing/config-file-validator/v2/pkg/finder"
)

func main() {

	// Initialize a file system finder
	fileSystemFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("/path/to/search", "/another/path/to/search"),
	)

	// Initialize the CLI
	cfv := cli.Init(
		cli.WithFinder(fileSystemFinder),
	)
	
	// Run the config file validation
	exitStatus, err := cfv.Run()
	if err != nil {
	  log.Printf("Errors occurred during execution: %v", err)
	}
	
	os.Exit(exitStatus)
}
```

## Custom Reporter

Will output JSON to stdout. To output to a file, pass a value to the `reporter.NewJSONReporter` constructor.

```go
package main

import (
	"os"
	"log"

	"github.com/Boeing/config-file-validator/v2/pkg/cli"
	"github.com/Boeing/config-file-validator/v2/pkg/reporter"
)

func main() {
	// Initialize reporter type
	var outputDir string
	jsonReporter := reporter.NewJSONReporter(outputDir)

	// Initialize the CLI
	cfv := cli.Init(
		cli.WithFinder(fileSystemFinder),
		cli.WithReporters(jsonReporter),
	)
	
	// Run the config file validation
	exitStatus, err := cfv.Run()
	if err != nil {
	  log.Printf("Errors occurred during execution: %v", err)
	}
	
	os.Exit(exitStatus)
}
```

## Additional Configuration Options

### Exclude Directories

```go
excludeDirs := []string{"test", "subdir"}
fileSystemFinder := finder.FileSystemFinderInit(
	finder.WithExcludeDirs(excludeDirs),
)
cfv := cli.Init(
      cli.WithFinder(fileSystemFinder),
)
```

### Exclude File Types

```go
excludeFileTypes := []string{"yaml", "json"}
fileSystemFinder := finder.FileSystemFinderInit(
      finder.WithExcludeFileTypes(excludeFileTypes),
)
cfv := cli.Init(
      cli.WithFinder(fileSystemFinder),
)
```

### Set Recursion Depth

```go
fileSystemFinder := finder.FileSystemFinderInit(
      finder.WithDepth(0)
)
cfv := cli.Init(
      cli.WithFinder(fileSystemFinder),
)
```

## Suppress Output

```go
cfv := cli.Init(
      cli.WithQuiet(true),
)
```

## Group Output

```go
groupOutput := []string{"pass-fail"} 
cfv := cli.Init(
      cli.WithGroupOutput(groupOutput),
)
```

## Schema Validation Options

### Require Schema

Fail validation for files that support schema validation but don't declare one.

```go
cfv := cli.Init(
      cli.WithRequireSchema(true),
)
```

### Disable Schema Validation

Skip all schema validation. Only syntax is checked.

```go
cfv := cli.Init(
      cli.WithNoSchema(true),
)
```

### Schema Map

Map file patterns to schema files. Use JSON Schema (`.json`) for JSON, YAML, TOML, and TOON files. Use XSD (`.xsd`) for XML files.

```go
schemaMap := map[string]string{
      "**/package.json": "schemas/package.schema.json",
      "**/config.xml":   "schemas/config.xsd",
}
cfv := cli.Init(
      cli.WithSchemaMap(schemaMap),
)
```

### SchemaStore

Enable automatic schema lookup using the embedded [SchemaStore](https://github.com/SchemaStore/schemastore) catalog with remote fetching:

```go
import "github.com/Boeing/config-file-validator/v2/pkg/schemastore"

// Use embedded catalog (schemas fetched remotely and cached)
store, err := schemastore.OpenEmbedded()
if err != nil {
      log.Fatal(err)
}
cfv := cli.Init(
      cli.WithSchemaStore(store),
)
```

For air-gapped environments, use a local clone:

```go
store, err := schemastore.Open("/path/to/schemastore")
if err != nil {
      log.Fatal(err)
}
cfv := cli.Init(
      cli.WithSchemaStore(store),
)
```
