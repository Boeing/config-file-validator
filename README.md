# Config File Validator

<p align="center">
  <a href="https://opensource.org/licenses/Apache-2.0">
  <img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="Apache 2 License">
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


  <a href='https://coveralls.io/github/kehoecj/config-file-validator?branch=readme_updates'>
  <img src='https://coveralls.io/repos/github/kehoecj/config-file-validator/badge.svg?branch=readme_updates' alt='Coverage Status' />
  </a>



</p>

## About
How many deployments have you done that needed to be rolled back due to a missing character in a configuration file in your repo? If you're like most teams that number is greater than zero. The config file validator was created to solve this problem by searching through your project and validating the syntax of all configuation files. 

### Where can you use this tool?
* In a CI/CD pipeline as a quality gate
* On your desktop to validate configuration files as you write them
* As a library within your existing go code

### What types of files are supported?
* XML
* JSON
* YAML
* TOML

## Getting Started
Binaries and a container on Dockerhub will eventually be available but for now the project must be built on an environment that has golang 1.17+ installed.

### Linux
#### Build
```
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
```
cp ./validator /usr/local/bin/
chmod +x /usr/local/bin/validator
```

### Windows
#### Build
```
CGO_ENABLED=0 \
GOOS=windows \
GOARCH=amd64 \
go build \
-ldflags='-w -s -extldflags "-static"' \
-tags netgo \
-o validator.exe \
cmd/validator/validator.go
```

#### Install
```powershell
mkdir -p 'C:\Program Files\validator'
cp .\validator.exe 'C:\Program Files\validator'
[Environment]::SetEnvironmentVariable("C:\Program Files\validator", $env:Path, [System.EnvironmentVariableTarget]::Machine)
```

### Docker
The config file validator can be built as a docker container
```
docker build . -t config-file-validator
```

## Using
```
Usage of /validator:
  -exclude-dirs string
    	Subdirectories to exclude when searching for configuration files
  -search-path string
    	The search path for configuration files
```

### Examples
#### Standard Run
```
validator -search-path /path/to/search
```

![Standard Run](./img/standard_run.png)

#### Exclude dirs
Exclude subdirectories in the search path

```
validator -search-path /path/to/search -exclude-dirs=/path/to/search/tests
```

![Exclude Dirs Run](./img/exclude_dirs.png)

#### Container Run
```
docker run -it --rm -v /path/to/config/file/location:/test -search-path=/test
```

![Standard Run](./img/docker_run.png)

## Contributing
We welcome contributions! Please refer to our [contributing guide](/CONTRIBUTING.md)

## License

The Config File Validator is released under the [Apache 2.0](/LICENSE) License
