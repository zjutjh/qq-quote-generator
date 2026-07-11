$ErrorActionPreference = "Stop"
$Version = "0.47.0"
$Source = Join-Path $env:TEMP "qq-quote-resvg-$Version"

if (-not (Test-Path $Source)) {
    git clone --depth 1 --branch "v$Version" https://github.com/linebender/resvg $Source
}

cargo build --release -p resvg-capi --manifest-path (Join-Path $Source "Cargo.toml")
$Output = Join-Path $PSScriptRoot "lib/windows-amd64"
New-Item -ItemType Directory -Force $Output | Out-Null
Copy-Item (Join-Path $Source "target/release/libresvg.a") (Join-Path $Output "libresvg.a") -Force
Copy-Item (Join-Path $Source "crates/c-api/resvg.h") (Join-Path $PSScriptRoot "../../resvg.h") -Force
