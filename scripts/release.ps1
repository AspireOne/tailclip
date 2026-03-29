[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v\d+\.\d+\.\d+([.-][0-9A-Za-z.-]+)?$')]
    [string]$Version
)

$ErrorActionPreference = 'Stop'

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    throw "GitHub CLI 'gh' is required but was not found in PATH."
}

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

Write-Host "Triggering GitHub release workflow for $Version"
gh workflow run Release -f version=$Version -f prerelease=false

