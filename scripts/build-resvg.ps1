$ErrorActionPreference = "Stop"

$Version = "0.47.0"
$Target = "x86_64-pc-windows-gnu"
$Source = Join-Path $env:TEMP "qq-quote-resvg-$Version"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$Output = Join-Path $ProjectRoot "internal/resvg/lib/windows-amd64"

if (-not (Get-Command git -ErrorAction SilentlyContinue)) { throw "git is required" }
if (-not (Get-Command cargo -ErrorAction SilentlyContinue)) { throw "cargo is required" }
if (-not (Get-Command rustup -ErrorAction SilentlyContinue)) { throw "rustup is required" }
if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) { throw "MinGW-w64 gcc is required" }

if (-not (Test-Path $Source)) {
    git clone --depth 1 --branch "v$Version" https://github.com/linebender/resvg $Source
}

$Commit = git -C $Source describe --tags --exact-match
if ($Commit -ne "v$Version") { throw "resvg cache is not v${Version}: $Commit" }

rustup target add $Target
cargo build --release -p resvg-capi --target $Target --manifest-path (Join-Path $Source "Cargo.toml")

New-Item -ItemType Directory -Force $Output | Out-Null
Copy-Item (Join-Path $Source "target/$Target/release/libresvg.a") (Join-Path $Output "libresvg.a") -Force
Copy-Item (Join-Path $Source "crates/c-api/resvg.h") (Join-Path $ProjectRoot "internal/resvg/resvg.h") -Force

Write-Host "resvg $Version built at $Output"
