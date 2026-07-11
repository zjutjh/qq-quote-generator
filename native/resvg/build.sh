#!/bin/sh
set -eu

version=0.47.0
source_dir="${TMPDIR:-/tmp}/qq-quote-resvg-$version"
if [ ! -d "$source_dir" ]; then
  git clone --depth 1 --branch "v$version" https://github.com/linebender/resvg "$source_dir"
fi
cargo build --release -p resvg-capi --manifest-path "$source_dir/Cargo.toml"
output="$(dirname "$0")/lib/linux-amd64"
mkdir -p "$output"
cp "$source_dir/target/release/libresvg.a" "$output/libresvg.a"
cp "$source_dir/crates/c-api/resvg.h" "$(dirname "$0")/../../resvg.h"
