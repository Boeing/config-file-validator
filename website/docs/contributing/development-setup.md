---
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Development Setup

Requires Go 1.26+ installed.

## Build from source

<Tabs>
<TabItem value="macos" label="macOS" default>

```shell
CGO_ENABLED=0 \
GOOS=darwin \
GOARCH=arm64 \
go build \
-ldflags='-w -s -extldflags "-static"' \
-tags netgo \
-o validator \
cmd/validator/validator.go
```

For Intel Macs, use `GOARCH=amd64`.

Install:

```shell
cp ./validator /usr/local/bin/
chmod +x /usr/local/bin/validator
```

</TabItem>
<TabItem value="linux" label="Linux">

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

Install:

```shell
cp ./validator /usr/local/bin/
chmod +x /usr/local/bin/validator
```

</TabItem>
<TabItem value="windows" label="Windows">

```powershell
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
go build `
  -ldflags='-w -s -extldflags "-static"' `
  -tags netgo `
  -o validator.exe `
  cmd/validator/validator.go
```

Install to Local App Data:

```powershell
$install = Join-Path $env:LOCALAPPDATA 'Programs\validator'
New-Item -Path $install -ItemType Directory -Force | Out-Null
Copy-Item -Path .\validator.exe -Destination $install -Force
```

</TabItem>
</Tabs>

## Run tests

```shell
go test ./...
```

## Docker build

```shell
docker build . -t config-file-validator:latest
```

Run against a local directory:

```shell
docker run --rm -v "$(pwd):/work" config-file-validator:latest /work
```
