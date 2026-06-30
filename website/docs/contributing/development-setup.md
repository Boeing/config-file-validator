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
-o cfv \
cmd/cfv/cfv.go
```

For Intel Macs, use `GOARCH=amd64`.

Install:

```shell
cp ./cfv /usr/local/bin/
chmod +x /usr/local/bin/cfv
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
-o cfv \
cmd/cfv/cfv.go
```

Install:

```shell
cp ./cfv /usr/local/bin/
chmod +x /usr/local/bin/cfv
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
  -o cfv.exe `
  cmd/cfv/cfv.go
```

Install to Local App Data:

```powershell
$install = Join-Path $env:LOCALAPPDATA 'Programs\cfv'
New-Item -Path $install -ItemType Directory -Force | Out-Null
Copy-Item -Path .\cfv.exe -Destination $install -Force
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
