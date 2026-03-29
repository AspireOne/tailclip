[CmdletBinding()]
param(
    [string]$OutputDir = "bin",
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$outputPath = Join-Path $repoRoot $OutputDir
$manifestSource = Join-Path $repoRoot "cmd\tailclip-agent\app.manifest"

if (-not (Test-Path -LiteralPath $outputPath)) {
    New-Item -ItemType Directory -Path $outputPath | Out-Null
}

Push-Location $repoRoot
try {
    if (-not $SkipTests) {
        Write-Host "Running tests..."
        go test ./...
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
    }

    $consoleExe = Join-Path $outputPath "tailclip-agent.exe"
    $guiExe = Join-Path $outputPath "tailclip-agent-gui.exe"
    $consoleManifest = "$consoleExe.manifest"
    $guiManifest = "$guiExe.manifest"

    Write-Host "Building console binary: $consoleExe"
    go build -o $consoleExe ./cmd/tailclip-agent
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    Write-Host "Building Windows GUI binary: $guiExe"
    go build -ldflags="-H windowsgui" -o $guiExe ./cmd/tailclip-agent
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    Write-Host "Copying manifests..."
    Copy-Item -LiteralPath $manifestSource -Destination $consoleManifest -Force
    Copy-Item -LiteralPath $manifestSource -Destination $guiManifest -Force

    Write-Host "Build complete."
}
finally {
    Pop-Location
}
