#!/bin/bash
# Script to generate checksums for Chocolatey package
# This script should be run when updating the package version

VERSION="1.8.1"
URL_64="https://github.com/Boeing/config-file-validator/releases/download/v${VERSION}/validator-v${VERSION}-windows-amd64.tar.gz"

echo "Downloading Windows AMD64 binary to calculate checksum..."
curl -L -o "validator-windows-amd64.tar.gz" "$URL_64"

echo "Calculating SHA256 checksum..."
CHECKSUM_64=$(sha256sum validator-windows-amd64.tar.gz | cut -d' ' -f1)

echo ""
echo "=== UPDATE CHOCOLATEY INSTALL SCRIPT ==="
echo "Update the checksum64 value in chocolateyinstall.ps1:"
echo "checksum64 = '$CHECKSUM_64'"
echo ""
echo "URL: $URL_64"
echo "SHA256: $CHECKSUM_64"

# Clean up
rm -f validator-windows-amd64.tar.gz

echo ""
echo "Checksum generation complete!"