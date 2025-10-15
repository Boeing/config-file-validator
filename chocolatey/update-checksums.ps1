# PowerShell script to generate checksums for Chocolatey package
# This script should be run when updating the package version

$VERSION = "1.8.1"
$URL_64 = "https://github.com/Boeing/config-file-validator/releases/download/v$VERSION/validator-v$VERSION-windows-amd64.zip"

Write-Host "Downloading Windows AMD64 binary to calculate checksum..." -ForegroundColor Yellow
$tempFile = "validator-windows-amd64.zip"

try {
    Invoke-WebRequest -Uri $URL_64 -OutFile $tempFile -ErrorAction Stop
    
    Write-Host "Calculating SHA256 checksum..." -ForegroundColor Yellow
    $hash = Get-FileHash -Path $tempFile -Algorithm SHA256
    $checksum = $hash.Hash.ToLower()
    
    Write-Host ""
    Write-Host "=== UPDATE CHOCOLATEY INSTALL SCRIPT ===" -ForegroundColor Green
    Write-Host "Update the checksum64 value in chocolateyinstall.ps1:" -ForegroundColor Cyan
    Write-Host "checksum64 = '$checksum'" -ForegroundColor White
    Write-Host ""
    Write-Host "URL: $URL_64" -ForegroundColor White  
    Write-Host "SHA256: $checksum" -ForegroundColor White
    
    # Clean up
    Remove-Item $tempFile -Force
    
    Write-Host ""
    Write-Host "Checksum generation complete!" -ForegroundColor Green
    
} catch {
    Write-Error "Failed to download or process file: $_"
    if (Test-Path $tempFile) {
        Remove-Item $tempFile -Force
    }
}