<div align="center">
  <img src="./img/logo.png" width="200" height="200"/>
  <h1>Config File Validator</h1>
  <p>Single cross-platform CLI tool to validate different configuration file types</p>
</div>

<p align="center">
<img id="cov" src="https://img.shields.io/badge/Coverage-93.8%25-brightgreen" alt="Code Coverage">

  <a href="https://scorecard.dev/viewer/?uri=github.com/Boeing/config-file-validator">
    <img src="https://api.scorecard.dev/projects/github.com/Boeing/config-file-validator/badge" alt="OpenSSF Scorecard">
  </a>

  <a href="https://www.bestpractices.dev/projects/9027">
    <img src="https://www.bestpractices.dev/projects/9027/badge">
  </a>

  <a href="https://opensource.org/licenses/Apache-2.0">
  <img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="Apache 2 License">
  </a>

  <a href="https://github.com/avelino/awesome-go">
  <img src="https://awesome.re/mentioned-badge.svg" alt="Awesome Go">
  </a>  

  <a href="https://pkg.go.dev/github.com/Boeing/config-file-validator">
  <img src="https://pkg.go.dev/badge/github.com/Boeing/config-file-validator.svg" alt="Go Reference">
  </a>

  <a href="https://goreportcard.com/report/github.com/Boeing/config-file-validator">
  <img src="https://goreportcard.com/badge/github.com/Boeing/config-file-validator" alt="Go Report Card">
  </a>

  <a href="https://github.com/boeing/config-file-validator/actions/workflows/go.yml">
  <img src="https://github.com/boeing/config-file-validator/actions/workflows/go.yml/badge.svg" alt="Pipeline Status">
  </a>
</p>

## Supported config files formats:

* Apple PList XML
* CSV
* EDITORCONFIG
* ENV
* HCL
* HOCON
* INI
* JSON
* Properties
* TOML
* XML
* YAML

## Demo

<img src="./img/demo.gif" alt="demo" />

## Installation

There are several options to install the config file validator tool.

### Binary Releases

Download and unpack from https://github.com/Boeing/config-file-validator/releases


### Package Managers

#### [Homebrew](https://brew.sh/)

```shell
brew install config-file-validator
```

#### [MacPorts](https://ports.macports.org)

```shell
sudo port install config-file-validator
```

#### [Aqua](https://aquaproj.github.io/)

```shell
aqua g -i Boeing/config-file-validator
```

#### [Winget](https://learn.microsoft.com/en-us/windows/package-manager/winget/)

```shell
winget install Boeing.config-file-validator
```

#### [Scoop](https://scoop.sh/)

```shell
scoop install config-file-validator
```

### Arch Linux

We maintain and release an [AUR package](https://aur.archlinux.org/packages/config-file-validator) for the config-file-validator

```shell
git clone https://aur.archlinux.org/config-file-validator.git
cd config-file-validator
makepkg -si
```

### `go install`

If you have a go environment on your desktop you can use [go install](https://go.dev/doc/go-get-install-deprecation) to install the validator executable. The validator executable will be installed to the directory named by the GOBIN environment variable, which defaults to $GOPATH/bin or $HOME/go/bin if the GOPATH environment variable is not set.

```shell
go install github.com/Boeing/config-file-validator/cmd/validator@v1.8.1
```

## Usage

```
Usage: validator [OPTIONS] [<search_path>...]

positional arguments:
    search_path: The search path on the filesystem for configuration files. Defaults to the current working directory if no search_path provided

optional flags:
  -depth int
        Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal
  -exclude-dirs string
        Subdirectories to exclude when searching for configuration files
  -exclude-file-types string
        A comma separated list of file types to ignore
  -globbing
        If globbing flag is set, check for glob patterns in the arguments.
  -groupby string
        Group output by filetype, directory, pass-fail. Supported for Standard and JSON reports
  -quiet
        If quiet flag is set. It doesn't print any output to stdout.
  -reporter value
        A string representing report format and optional output file path separated by colon if present.
        Usage: --reporter <format>:<optional_file_path>
        Multiple reporters can be specified: --reporter json:file_path.json --reporter junit:another_file_path.xml
        Omit the file path to output to stdout: --reporter json or explicitly specify stdout using "-": --reporter json:-
        Supported formats: standard, json, junit, and sarif (default: "standard")
  -version
        Version prints the release version of validator
```

### Environment Variables

The config-file-validator supports setting options via environment variables. If both command-line flags and environment variables are set, the command-line flags will take precedence. The supported environment variables are as follows:

| Environment Variable | Equivalent Flag |
|----------------------|-----------------|
| `CFV_DEPTH`          | `-depth`        |
| `CFV_EXCLUDE_DIRS`   | `-exclude-dirs` |
| `CFV_EXCLUDE_FILE_TYPES` | `-exclude-file-types` |
| `CFV_REPORTER`       | `-reporter`     |
| `CFV_GROUPBY`        | `-groupby`      |
| `CFV_QUIET`          | `-quiet`        |
| `CFV_GLOBBING`          | `-globbing`  |

### Examples

#### Standard Run

If the search path is omitted it will search the current directory

```shell
validator /path/to/search
```

![Standard Run](./img/standard_run.gif)

#### Multiple search paths

Multiple search paths are supported, and the results will be merged into a single report

```shell
validator /path/to/search /another/path/to/search
```

![Multiple Search Paths Run](./img/multiple_paths.gif)

#### Exclude directories

Exclude subdirectories in the search path

```shell
validator --exclude-dirs=/path/to/search/tests /path/to/search
```

![Exclude Dirs Run](./img/exclude_dirs.gif)

#### Exclude file types

Exclude file types in the search path. Available file types are `csv`, `env`, `hcl`, `hocon`, `ini`, `json`, `plist`, `properties`, `toml`, `xml`, `yaml`, and `yml`

```shell
validator --exclude-file-types=json /path/to/search
```

![Exclude File Types Run](./img/exclude_file_types.gif)

#### Customize recursion depth

By default there is no recursion limit. If desired, the recursion depth can be set to an integer value. If depth is set to `0` recursion will be disabled and only the files in the search path will be validated.

```shell
validator --depth=0 /path/to/search
```

![Custom Recursion Run](./img/custom_recursion.gif)

#### Customize report output

You can customize the report output and save the results to a file (default name is result.{extension}). The available report types are `standard`, `junit`, `json`, and `sarif`. You can specify multiple report types by chaining the `--reporter` flags.

You can specify a path to an output file for any reporter by appending `:<path>` the the name of the reporter. Providing an output file is optional and the results will be printed to stdout by default. To explicitly direct the output to stdout, use `:-` as the file path.

```shell
validator --reporter=json:- /path/to/search
validator --reporter=json:output.json --reporter=standard /path/to/search
```

![Exclude File Types Run](./img/custom_reporter.gif)

### Group report output

Group the report output by file type, directory, or pass-fail. Supports one or more groupings.

```shell
validator -groupby filetype
```

![Groupby File Type](./img/gb-filetype.gif)

#### Multiple groups

```shell
validator -groupby directory,pass-fail
```

![Groupby File Type and Pass/Fail](./img/gb-filetype-and-pass-fail.gif)

### Output results to a file

Output report results to a file (default name is `result.{extension}`). Must provide reporter flag with a supported extension format. Available options are `junit` and `json`. If an existing directory is provided, create a file named default name in the given directory. If a file name is provided, create a file named the given name at the current working directory.

```shell
validator --reporter=json --output=/path/to/dir
```

### Suppress output

Passing the `--quiet` flag suppresses all output to stdout. If there are invalid config files the validator tool will exit with 1. Any errors in execution such as an invalid path will still be displayed.

```shell
validator --quiet /path/to/search
```

### Search files using a glob pattern

Use the `-globbing` flag to validate files matching a specified pattern. Include the pattern as a positional argument in double quotes. Multiple glob patterns and direct file paths are supported. If invalid config files are detected, the validator tool exits with code 1, and errors (e.g., invalid patterns) are displayed.

[Learn more about glob patterns](https://www.digitalocean.com/community/tools/glob)

```shell
# Validate all `.json` files in a directory
validator -globbing "/path/to/files/*.json"

# Recursively validate all `.json` files in subdirectories
validator -globbing "/path/to/files/**/*.json"

# Mix glob patterns and paths
validator -globbing "/path/*.json" /path/to/search
```

## Calling the config-file-validator programatically

The config-file-validator can be called programatically from within a Go program through the `cli` package.

### Default configuration

The default configuration will perform the following actions:

* Search for all supported configuration file types in the current directory and its subdirectories 
* Uses the default reporter which will output validation results to console (stdout)

```go
package main

import (
	"os"
	"log"

	"github.com/Boeing/config-file-validator/pkg/cli"
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

### Custom Search Paths

The below example will search the provided search paths.

```go
package main

import (
	"os"
	"log"

	"github.com/Boeing/config-file-validator/pkg/cli"
	"github.com/Boeing/config-file-validator/pkg/finder"
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

### Custom Reporter

Will output JSON to stdout. To output to a file, pass a value to the `reporter.NewJSONReporter` constructor.

```go
package main

import (
	"os"
	"log"

	"github.com/Boeing/config-file-validator/pkg/cli"
	"github.com/Boeing/config-file-validator/pkg/reporter"
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

### Additional Configuration Options

#### Exclude Directories

```go
excludeDirs := []string{"test", "subdir"}
fileSystemFinder := finder.FileSystemFinderInit(
	finder.WithExcludeDirs(excludeDirs),
)
cfv := cli.Init(
      cli.WithFinder(fileSystemFinder),
)
```

#### Exclude File Types

```go
excludeFileTypes := []string{"yaml", "json"}
fileSystemFinder := finder.FileSystemFinderInit(
      finder.WithExcludeFileTypes(excludeFileTypes),
)
cfv := cli.Init(
      cli.WithFinder(fileSystemFinder),
)
```

#### Set Recursion Depth

```go
fileSystemFinder := finder.FileSystemFinderInit(
      finder.WithDepth(0)
)
cfv := cli.Init(
      cli.WithFinder(fileSystemFinder),
)
```

### Suppress Output

```go
cfv := cli.Init(
      cli.WithQuiet(true),
)
```

### Group Output

```go
groupOutput := []string{"pass-fail"} 
cfv := cli.Init(
      cli.WithGroupOutput(groupOutput),
)
```

## Build

The project can be downloaded and built from source using an environment with Go 1.25+ installed. After a successful build, the binary can be moved to a location on your operating system PATH.

### macOS

#### Build

```shell
CGO_ENABLED=0 \
GOOS=darwin \
GOARCH=arm64 \ # for Intel use amd64
go build \
-ldflags='-w -s -extldflags "-static"' \
-tags netgo \
-o validator \
cmd/validator/validator.go
```

#### Install

```shell
cp ./validator /usr/local/bin/
chmod +x /usr/local/bin/validator
```

### Linux

#### Build

```shell
CGO_ENABLED=0 \
GOOS=linux \
GOARCH=amd64 \
go build \
-ldflags='-w -s -extldflags "-static"' \
-tags netgo \
-o validator \
cmd/validator/validator.go
```

#### Install

```shell
cp ./validator /usr/local/bin/
chmod +x /usr/local/bin/validator
```

### Windows

#### Build

```powershell
$env:CGO_ENABLED = '0'; `
$env:GOOS = 'windows'; `
$env:GOARCH = 'amd64'; `
go build `
-ldflags='-w -s -extldflags "-static"' `
-tags netgo `
-o validator.exe `
cmd/validator/validator.go
```

#### Install

The below script will install the config-file-validator as a user to Local App Data:

```powershell
$install = Join-Path $env:LOCALAPPDATA 'Programs\validator'; `
New-Item -Path $install -ItemType Directory -Force | Out-Null; `
Copy-Item -Path .\validator.exe -Destination $install -Force; `
$up = [Environment]::GetEnvironmentVariable('Path', [EnvironmentVariableTarget]::User); `
if (-not ($up.Split(';') -contains $install)) { `
  $new = if ([string]::IsNullOrEmpty($up)) { $install } else { $up + ';' + $install }; `
  [Environment]::SetEnvironmentVariable('Path', $new, [EnvironmentVariableTarget]::User); `
  Write-Host "Added $install to User PATH. Open a new shell to pick up the change."; `
} else { `
  Write-Host "$install is already in the User PATH."; `
}
```

### Docker

You can also use the provided Dockerfile to build the config file validator tool as a container

```shell
docker build . -t config-file-validator:v1.8.1
```

## Contributors

<a href="https://github.com/Boeing/config-file-validator/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Boeing/config-file-validator" />
</a>

## Contributing

We welcome contributions! Please refer to our [contributing guide](./CONTRIBUTING.md)

## License

The Config File Validator is released under the [Apache 2.0](./LICENSE) License