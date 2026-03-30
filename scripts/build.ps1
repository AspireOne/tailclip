[CmdletBinding()]
param(
    [string]$OutputDir = "bin",
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$outputPath = Join-Path $repoRoot $OutputDir
$manifestSource = Join-Path $repoRoot "cmd\tailclip-agent\app.manifest"
$sysoDir = Join-Path $repoRoot "cmd\tailclip-agent"
$targetArch = (go env GOARCH).Trim()
$resourceObject = Join-Path $sysoDir ("rsrc_windows_{0}.syso" -f $targetArch)

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

    Write-Host "Generating embedded manifest resource: $resourceObject"
    go run ./cmd/genwinres -manifest $manifestSource -out $resourceObject -arch $targetArch
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

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

    Remove-Item -LiteralPath "$consoleExe.manifest" -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath "$guiExe.manifest" -Force -ErrorAction SilentlyContinue

    Write-Host "Build complete."
}
finally {
    Pop-Location
}
