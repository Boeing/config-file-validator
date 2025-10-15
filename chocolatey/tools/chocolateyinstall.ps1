$ErrorActionPreference = 'Stop'

$packageName = $env:ChocolateyPackageName
$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$url64 = 'https://github.com/Boeing/config-file-validator/releases/download/v1.8.1/validator-v1.8.1-windows-amd64.zip'

$packageArgs = @{
  packageName   = $packageName
  unzipLocation = $toolsDir
  url64bit      = $url64
  softwareName  = 'config-file-validator*'
  checksum64    = 'f6b004eda4507221a77a862cd78a640cd6cc8e658cf5464d6310c4fe60df442a'
  checksumType64= 'sha256'
  validExitCodes= @(0)
}

# Download and extract the zip file
Install-ChocolateyZipPackage @packageArgs

# The executable should now be available as validator.exe in the tools directory
$executablePath = Join-Path $toolsDir "validator.exe"
if (Test-Path $executablePath) {
    Write-Host "Config File Validator installed successfully to $executablePath"
} else {
    Write-Error "Installation failed: validator.exe not found in expected location."
}