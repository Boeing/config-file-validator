---
sidebar_position: 2
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Installation

## Package managers

<Tabs>
<TabItem value="brew" label="Homebrew" default>

```shell
brew install config-file-validator
```

</TabItem>
<TabItem value="macports" label="MacPorts">

```shell
sudo port install config-file-validator
```

</TabItem>
<TabItem value="winget" label="Winget">

```shell
winget install Boeing.config-file-validator
```

</TabItem>
<TabItem value="scoop" label="Scoop">

```shell
scoop install config-file-validator
```

</TabItem>
<TabItem value="aqua" label="Aqua">

```shell
aqua g -i Boeing/config-file-validator
```

</TabItem>
<TabItem value="aur" label="Arch Linux (AUR)">

```shell
git clone https://aur.archlinux.org/config-file-validator.git
cd config-file-validator
makepkg -si
```

</TabItem>
</Tabs>

## Binary releases

Pre-built binaries for macOS, Linux, and Windows are available on the [GitHub Releases](https://github.com/Boeing/config-file-validator/releases) page.

Download the archive for your platform, extract it, and place the `cfv` binary somewhere on your `PATH`.

## go install

Requires a working Go toolchain (1.26+):

```shell
go install github.com/Boeing/config-file-validator/v3/cmd/cfv@latest
```

The binary installs to `$GOBIN` (defaults to `$GOPATH/bin` or `$HOME/go/bin`).

## Build from source

If you need a custom build, see [Development Setup](./contributing/development-setup.md) for platform-specific build instructions.

## Verify the installation

```shell
cfv version
```

This prints the installed version and exits. If the command is not found, ensure the binary's location is on your `PATH`.
