# Builds CoDServerPicker.exe and cspcli.exe into the dist folder.
# The version comes from the VERSION file. Use -Version to set a different value.
# The user interface (Fyne) needs a C compiler (MinGW-w64 gcc) on PATH.
param(
    [string]$Version = (Get-Content -Raw "$PSScriptRoot\VERSION").Trim()
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

function Invoke-Step {
    param([string]$Name, [scriptblock]$Command)
    Write-Host "== $Name"
    & $Command
    if ($LASTEXITCODE -ne 0) { throw "$Name failed (exit code $LASTEXITCODE)" }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "Go is not installed. Get it from https://go.dev/dl/." }

New-Item -ItemType Directory -Force dist | Out-Null
$ldflags = "-s -w -X main.version=$Version"

Invoke-Step "build cspcli" { go build -trimpath -ldflags $ldflags -o dist\cspcli.exe .\cmd\cspcli }

if (Test-Path cmd\CoDServerPicker) {
    if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) { throw "gcc is not installed. Install MinGW-w64 and add its bin folder to PATH." }
    $env:CGO_ENABLED = "1"

    Remove-Item cmd\CoDServerPicker\*.syso -ErrorAction SilentlyContinue
    Invoke-Step "build CoDServerPicker" { go build -trimpath -ldflags "$ldflags -H windowsgui" -o dist\CoDServerPicker.exe .\cmd\CoDServerPicker }

    # MinGW links its own default manifest (no administrator rights) into the exe.
    # This step deletes all resources and adds the icon, the version and the administrator manifest.
    Invoke-Step "add windows resources" {
        go run github.com/tc-hib/go-winres@v0.3.3 patch --in winres\winres.json --delete --no-backup --product-version $Version --file-version $Version dist\CoDServerPicker.exe
    }
}

Write-Host "== done: version $Version"
Get-ChildItem dist\*.exe | ForEach-Object { "{0,-24} {1,8:N0} KB" -f $_.Name, ($_.Length / 1KB) }
