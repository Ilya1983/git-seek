<#
.SYNOPSIS
    Build script for git-seek on Windows.

.DESCRIPTION
    This script handles building git-seek on Windows, including downloading
    ONNX Runtime dependencies and creating release packages.

.PARAMETER Help
    Show help message.

.PARAMETER Setup
    Download ONNX Runtime for Windows.

.PARAMETER Build
    Build the binary (downloads deps if needed).

.PARAMETER Test
    Run tests.

.PARAMETER Clean
    Remove build artifacts.

.PARAMETER CleanAll
    Remove all generated files including dependencies.

.PARAMETER Release
    Create release package.

.PARAMETER Info
    Show build information.

.EXAMPLE
    .\build.ps1 -Help
    Show available commands.

.EXAMPLE
    .\build.ps1 -Build
    Build the binary.

.EXAMPLE
    .\build.ps1 -Setup -Build -Test
    Setup, build, and test in one command.
#>

[CmdletBinding()]
param(
    [switch]$Help,
    [switch]$Setup,
    [switch]$Build,
    [switch]$Test,
    [switch]$Clean,
    [switch]$CleanAll,
    [switch]$Release,
    [switch]$Info
)

#==============================================================================
# Configuration
#==============================================================================

$ErrorActionPreference = "Stop"

# Check for unsupported ARM64 Windows
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    Write-Host "ERROR: Windows ARM64 is not supported." -ForegroundColor Red
    Write-Host ""
    Write-Host "ONNX Runtime does not provide ARM64 Windows builds."
    Write-Host "Consider using WSL2 with the Linux ARM64 build instead:"
    Write-Host "  wsl --install"
    Write-Host "  # Then in WSL: make build"
    exit 1
}

# Project metadata
$PROJECT_NAME = "git-seek"
$VERSION = try { git describe --tags --always --dirty 2>$null } catch { "dev" }
if (-not $VERSION) { $VERSION = "dev" }
$GIT_COMMIT = try { git rev-parse --short HEAD 2>$null } catch { "unknown" }
if (-not $GIT_COMMIT) { $GIT_COMMIT = "unknown" }
$BUILD_DATE = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

# ONNX Runtime settings
$ORT_VERSION = "1.23.2"
$ORT_PLATFORM = "win-x64"
$ORT_BASE_URL = "https://github.com/microsoft/onnxruntime/releases/download/v$ORT_VERSION"
$ORT_ARCHIVE = "onnxruntime-$ORT_PLATFORM-$ORT_VERSION.zip"
$ORT_URL = "$ORT_BASE_URL/$ORT_ARCHIVE"

# Directory structure
$DEPS_DIR = "deps"
$ORT_DIR = "$DEPS_DIR\onnxruntime"
$ORT_DOWNLOAD_DIR = "$ORT_DIR\$ORT_PLATFORM"
$ORT_LIB_DIR = "$ORT_DOWNLOAD_DIR\lib"
$ORT_LIB_FILE = "$ORT_LIB_DIR\onnxruntime.dll"
$DIST_DIR = "dist"

# Build output
$BINARY = "$PROJECT_NAME.exe"

#==============================================================================
# Helper Functions
#==============================================================================

function Write-Header {
    param([string]$Message)
    Write-Host ""
    Write-Host "=== $Message ===" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Green
}

function Write-Info {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Yellow
}

function Test-Dependencies {
    if (-not (Test-Path $ORT_LIB_FILE)) {
        return $false
    }
    return $true
}

#==============================================================================
# Commands
#==============================================================================

function Show-Help {
    Write-Host "git-seek - Semantic search for git commits" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage: .\build.ps1 [options]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -Help       Show this help message"
    Write-Host "  -Setup      Download ONNX Runtime for Windows"
    Write-Host "  -Build      Build the binary (downloads deps if needed)"
    Write-Host "  -Test       Run tests"
    Write-Host "  -Clean      Remove build artifacts"
    Write-Host "  -CleanAll   Remove all generated files including dependencies"
    Write-Host "  -Release    Create release package"
    Write-Host "  -Info       Show build information"
    Write-Host ""
    Write-Host "Examples:"
    Write-Host "  .\build.ps1 -Build              # Build the binary"
    Write-Host "  .\build.ps1 -Setup -Build       # Setup and build"
    Write-Host "  .\build.ps1 -Build -Test        # Build and test"
    Write-Host "  .\build.ps1 -Release            # Create release package"
    Write-Host ""
    Write-Host "Environment:"
    Write-Host "  Platform:     $ORT_PLATFORM"
    Write-Host "  ORT Version:  $ORT_VERSION"
    Write-Host "  Go Version:   $(try { go version 2>$null } catch { 'not found' })"
}

function Invoke-Setup {
    Write-Header "Setting up ONNX Runtime $ORT_VERSION for $ORT_PLATFORM"

    if (Test-Path $ORT_LIB_FILE) {
        Write-Info "ONNX Runtime already installed at $ORT_DOWNLOAD_DIR"
        return
    }

    # Create directories
    if (-not (Test-Path $ORT_DIR)) {
        New-Item -ItemType Directory -Path $ORT_DIR -Force | Out-Null
    }

    $archivePath = "$ORT_DIR\$ORT_ARCHIVE"

    # Download
    Write-Host "Downloading from $ORT_URL..."
    try {
        $ProgressPreference = 'SilentlyContinue'  # Speed up download
        Invoke-WebRequest -Uri $ORT_URL -OutFile $archivePath -UseBasicParsing
        $ProgressPreference = 'Continue'
    }
    catch {
        Write-Error "Failed to download ONNX Runtime: $_"
        exit 1
    }

    # Extract
    Write-Host "Extracting..."
    try {
        Expand-Archive -Path $archivePath -DestinationPath $ORT_DIR -Force

        # Move contents to expected location
        $extractedDir = "$ORT_DIR\onnxruntime-$ORT_PLATFORM-$ORT_VERSION"
        if (Test-Path $extractedDir) {
            if (Test-Path $ORT_DOWNLOAD_DIR) {
                Remove-Item -Path $ORT_DOWNLOAD_DIR -Recurse -Force
            }
            Move-Item -Path $extractedDir -Destination $ORT_DOWNLOAD_DIR
        }

        # Cleanup archive
        Remove-Item -Path $archivePath -Force
    }
    catch {
        Write-Error "Failed to extract ONNX Runtime: $_"
        exit 1
    }

    Write-Success "ONNX Runtime installed to $ORT_DOWNLOAD_DIR"
}

function Invoke-Build {
    Write-Header "Building $PROJECT_NAME $VERSION"

    # Ensure dependencies are installed
    if (-not (Test-Dependencies)) {
        Write-Info "Dependencies not found, running setup..."
        Invoke-Setup
    }

    # Set environment for build
    $env:CGO_ENABLED = "1"
    $env:PATH = "$(Resolve-Path $ORT_LIB_DIR);$env:PATH"
    $env:ONNXRUNTIME_LIB_PATH = Resolve-Path $ORT_LIB_FILE

    # Build
    $ldflags = "-s -w -X main.version=$VERSION -X main.commit=$GIT_COMMIT -X main.date=$BUILD_DATE"

    try {
        go build -trimpath -ldflags $ldflags -o $BINARY .
    }
    catch {
        Write-Error "Build failed: $_"
        exit 1
    }

    Write-Success "Build successful: .\$BINARY"
    Write-Host ""
    Write-Host "To run, set the environment:"
    Write-Host "  `$env:PATH = `"$(Resolve-Path $ORT_LIB_DIR);`$env:PATH`""
    Write-Host "  `$env:ONNXRUNTIME_LIB_PATH = `"$(Resolve-Path $ORT_LIB_FILE)`""
    Write-Host ""
    Write-Host "Then run:"
    Write-Host "  .\$BINARY --help"
    Write-Host ""
    Write-Host "To make permanent, add to your PowerShell profile:" -ForegroundColor Yellow
    Write-Host "  notepad `$PROFILE"
    Write-Host "  # Add the export lines above to the file"
    Write-Host ""
    Write-Host "Or set system environment variables via:"
    Write-Host "  Control Panel > System > Advanced > Environment Variables"
}

function Invoke-Test {
    Write-Header "Running tests"

    # Ensure dependencies are installed
    if (-not (Test-Dependencies)) {
        Write-Info "Dependencies not found, running setup..."
        Invoke-Setup
    }

    # Set environment for tests
    $env:CGO_ENABLED = "1"
    $env:PATH = "$(Resolve-Path $ORT_LIB_DIR);$env:PATH"
    $env:ONNXRUNTIME_LIB_PATH = Resolve-Path $ORT_LIB_FILE

    try {
        go test -v ./...
    }
    catch {
        Write-Error "Tests failed: $_"
        exit 1
    }

    Write-Success "All tests passed!"
}

function Invoke-Clean {
    Write-Header "Cleaning build artifacts"

    if (Test-Path $BINARY) {
        Remove-Item -Path $BINARY -Force
    }
    if (Test-Path $DIST_DIR) {
        Remove-Item -Path $DIST_DIR -Recurse -Force
    }
    if (Test-Path "coverage.out") {
        Remove-Item -Path "coverage.out" -Force
    }
    if (Test-Path "coverage.html") {
        Remove-Item -Path "coverage.html" -Force
    }

    Write-Success "Cleaned build artifacts."
}

function Invoke-CleanAll {
    Invoke-Clean

    Write-Header "Cleaning dependencies"

    if (Test-Path $DEPS_DIR) {
        Remove-Item -Path $DEPS_DIR -Recurse -Force
    }

    Write-Success "Cleaned everything."
}

function Invoke-Release {
    Write-Header "Creating release package"

    # Build first
    Invoke-Build

    $RELEASE_NAME = "$PROJECT_NAME-$ORT_PLATFORM-$VERSION"
    $RELEASE_DIR = "$DIST_DIR\$RELEASE_NAME"

    # Create directories
    if (-not (Test-Path $DIST_DIR)) {
        New-Item -ItemType Directory -Path $DIST_DIR -Force | Out-Null
    }
    if (Test-Path $RELEASE_DIR) {
        Remove-Item -Path $RELEASE_DIR -Recurse -Force
    }
    New-Item -ItemType Directory -Path "$RELEASE_DIR\lib" -Force | Out-Null

    # Copy files
    Copy-Item -Path $BINARY -Destination $RELEASE_DIR
    Copy-Item -Path "$ORT_LIB_DIR\*" -Destination "$RELEASE_DIR\lib" -Recurse
    if (Test-Path "LICENSE") {
        Copy-Item -Path "LICENSE" -Destination $RELEASE_DIR
    }

    # Create README
    $readme = @"
# $PROJECT_NAME $VERSION

Semantic search for git commits.

## Quick Start

```powershell
`$env:PATH = "`$(pwd)\lib;`$env:PATH"
`$env:ONNXRUNTIME_LIB_PATH = "`$(pwd)\lib\onnxruntime.dll"
.\$PROJECT_NAME.exe --help
```

## Usage

```powershell
# Index your repository
.\$PROJECT_NAME.exe --index

# Search commits
.\$PROJECT_NAME.exe "your search query"
```
"@
    $readme | Out-File -FilePath "$RELEASE_DIR\README.md" -Encoding utf8

    # Create zip
    $zipPath = "$DIST_DIR\$RELEASE_NAME.zip"
    if (Test-Path $zipPath) {
        Remove-Item -Path $zipPath -Force
    }
    Compress-Archive -Path $RELEASE_DIR -DestinationPath $zipPath

    # Cleanup directory
    Remove-Item -Path $RELEASE_DIR -Recurse -Force

    # Generate checksum
    $hash = Get-FileHash -Path $zipPath -Algorithm SHA256
    $checksumLine = "$($hash.Hash.ToLower())  $RELEASE_NAME.zip"
    $checksumLine | Out-File -FilePath "$DIST_DIR\checksums.txt" -Encoding utf8

    Write-Host ""
    Write-Success "Release package created:"
    Write-Host "  $zipPath"
    Write-Host ""
    Write-Host "Checksum:"
    Write-Host "  $checksumLine"
}

function Show-Info {
    Write-Host "Project:       $PROJECT_NAME"
    Write-Host "Version:       $VERSION"
    Write-Host "Git Commit:    $GIT_COMMIT"
    Write-Host "Build Date:    $BUILD_DATE"
    Write-Host "Go Version:    $(try { (go version) -replace 'go version ', '' } catch { 'not found' })"
    Write-Host "Platform:      $ORT_PLATFORM"
    Write-Host "ORT Version:   $ORT_VERSION"
    Write-Host "ORT Library:   $ORT_LIB_FILE"
    Write-Host "Binary:        $BINARY"
}

#==============================================================================
# Main
#==============================================================================

# If no parameters, show help
if (-not ($Help -or $Setup -or $Build -or $Test -or $Clean -or $CleanAll -or $Release -or $Info)) {
    Show-Help
    exit 0
}

# Execute requested commands in order
if ($Help) { Show-Help }
if ($Info) { Show-Info }
if ($CleanAll) { Invoke-CleanAll }
elseif ($Clean) { Invoke-Clean }
if ($Setup) { Invoke-Setup }
if ($Build -and -not $Release) { Invoke-Build }
if ($Test) { Invoke-Test }
if ($Release) { Invoke-Release }
