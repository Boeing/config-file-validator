# Chocolatey Package Submission Guide for config-file-validator

## Overview

This guide explains how to complete the submission of the config-file-validator package to the Chocolatey Community Repository.

## Current Status ✅

The following has been completed:

- ✅ Created Chocolatey package structure
- ✅ Generated nuspec file with proper metadata
- ✅ Created installation and uninstallation scripts
- ✅ Generated correct SHA256 checksum for v1.8.1
- ✅ Successfully built package locally
- ✅ Updated README.md and index.md with Chocolatey installation instructions

## Next Steps for Submission

### 1. Create Chocolatey Community Account

1. Go to https://community.chocolatey.org/
2. Click "Sign Up" to create an account
3. Verify your email address
4. Complete your profile

### 2. Get API Key

1. Once logged in, go to your account page
2. Navigate to "API Keys"
3. Generate an API key for package publishing

### 3. Set Up Chocolatey CLI for Publishing

```powershell
# Set your API key (replace YOUR_API_KEY_HERE with actual key)
choco apikey add --source https://push.chocolatey.org/ --key YOUR_API_KEY_HERE
```

### 4. Final Testing (Recommended)

Before submitting, test the package locally:

```powershell
# Install the package locally
choco install config-file-validator --source . --force

# Test the executable
validator --version

# Test uninstallation
choco uninstall config-file-validator
```

### 5. Submit the Package

```powershell
# Navigate to the chocolatey directory
cd chocolatey/

# Push the package to Chocolatey Community Repository
choco push config-file-validator.1.8.1.nupkg --source https://push.chocolatey.org/
```

### 6. Package Review Process

- After submission, your package will be reviewed by Chocolatey moderators
- You will receive email notifications about the review status
- The review process typically takes a few days to a week
- If changes are requested, update the package and resubmit

## Package Guidelines Compliance ✅

This package complies with Chocolatey Community Repository guidelines:

- ✅ **Legal**: Uses official release binaries from GitHub
- ✅ **Naming**: Follows lowercase hyphenated naming convention
- ✅ **Security**: Includes SHA256 checksum verification
- ✅ **Metadata**: Complete package information and description
- ✅ **Scripts**: Proper installation and uninstallation scripts
- ✅ **Dependencies**: No additional dependencies required
- ✅ **Documentation**: Clear description and usage instructions

## Maintaining the Package

For future releases:

1. Update version in `config-file-validator.nuspec`
2. Update URL in `chocolateyinstall.ps1`
3. Run `update-checksums.ps1` to get new checksum
4. Update checksum in `chocolateyinstall.ps1`
5. Update release notes URL in nuspec
6. Build, test, and submit updated package

## Files Created

```
chocolatey/
├── config-file-validator.nuspec          # Package specification
├── config-file-validator.1.8.1.nupkg    # Built package
├── tools/
│   ├── chocolateyinstall.ps1            # Installation script
│   └── chocolateyuninstall.ps1          # Uninstallation script
├── update-checksums.ps1                 # PowerShell checksum generator
├── update-checksums.sh                  # Bash checksum generator (Linux/Mac)
├── README.md                            # Package documentation
└── SUBMISSION_GUIDE.md                  # This guide
```

## Support

- **Package Issues**: File issues at https://github.com/Boeing/config-file-validator/issues
- **Chocolatey Help**: https://docs.chocolatey.org/en-us/
- **Community Support**: https://community.chocolatey.org/

## Success Criteria

Once approved and published:

- Users will be able to install with: `choco install config-file-validator`
- Package will appear in search results at https://community.chocolatey.org/packages
- Installation instructions in README.md and index.md will be accurate

---

**Ready to submit!** 🚀

The package is fully prepared and ready for submission to the Chocolatey Community Repository.
