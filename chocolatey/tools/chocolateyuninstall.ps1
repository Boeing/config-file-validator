$ErrorActionPreference = 'Stop'

$packageName = $env:ChocolateyPackageName
$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$executablePath = Join-Path $toolsDir "validator.exe"

# Remove the executable if it exists
if (Test-Path $executablePath) {
    Remove-Item $executablePath -Force
    Write-Host "Removed validator.exe from $executablePath"
}

# Clean up any other files that might have been extracted
$filesToRemove = @("validator", "LICENSE", "README.md")
foreach ($file in $filesToRemove) {
    $filePath = Join-Path $toolsDir $file
    if (Test-Path $filePath) {
        Remove-Item $filePath -Force -Recurse
        Write-Host "Removed $file"
    }
}

Write-Host "Config File Validator has been uninstalled successfully."