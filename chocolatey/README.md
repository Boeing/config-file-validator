# Chocolatey Package for Config File Validator

This directory contains the Chocolatey package definition for config-file-validator.

## Package Structure

```
chocolatey/
├── config-file-validator.nuspec     # Package specification
├── tools/
│   ├── chocolateyinstall.ps1       # Installation script
│   └── chocolateyuninstall.ps1     # Uninstallation script
├── update-checksums.ps1            # PowerShell script to generate checksums
├── update-checksums.sh             # Bash script to generate checksums
└── README.md                       # This file
```

## Creating the Package

1. **Install Chocolatey CLI** (if not already installed):

   ```powershell
   Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
   ```

2. **Navigate to the chocolatey directory**:

   ```bash
   cd chocolatey/
   ```

3. **Update checksums** (when releasing a new version):

   ```powershell
   .\update-checksums.ps1
   ```

   Then update the `checksum64` value in `tools\chocolateyinstall.ps1` with the generated hash.

4. **Build the package**:

   ```bash
   choco pack
   ```

5. **Test the package locally**:

   ```bash
   choco install config-file-validator --source . --force
   ```

6. **Test uninstallation**:
   ```bash
   choco uninstall config-file-validator
   ```

## Updating for New Releases

When a new version of config-file-validator is released:

1. Update the `<version>` in `config-file-validator.nuspec`
2. Update the `$url64` in `tools\chocolateyinstall.ps1`
3. Run `update-checksums.ps1` to get the new SHA256 checksum
4. Update the `checksum64` value in `tools\chocolateyinstall.ps1`
5. Update the `<releaseNotes>` URL in the nuspec file
6. Build and test the package
7. Submit to Chocolatey Community Repository

## Publishing to Chocolatey Community Repository

1. **Create account** at https://community.chocolatey.org/

2. **Get your API key** from your account profile

3. **Set the API key**:

   ```bash
   choco apikey add --source https://push.chocolatey.org/ --key YOUR_API_KEY_HERE
   ```

4. **Push the package**:
   ```bash
   choco push config-file-validator.1.8.1.nupkg --source https://push.chocolatey.org/
   ```

## Package Guidelines

This package follows Chocolatey Community Repository guidelines:

- ✅ Uses official release binaries from GitHub releases
- ✅ Includes proper metadata and descriptions
- ✅ Follows naming conventions (lowercase with hyphens)
- ✅ Includes proper installation and uninstallation scripts
- ✅ Uses checksums for security validation
- ✅ Includes all required metadata fields

## Support

For issues with the Chocolatey package, please file issues at:
https://github.com/Boeing/config-file-validator/issues

For general Chocolatey support, see:
https://docs.chocolatey.org/en-us/
