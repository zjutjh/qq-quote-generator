#!/bin/sh
set -eu

version=0.47.0
source_dir="${TMPDIR:-/tmp}/qq-quote-resvg-$version"
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(dirname "$script_dir")

case "$(uname -s)-$(uname -m)" in
  Linux-x86_64) platform=linux-amd64 ;;
  Darwin-x86_64) platform=darwin-amd64 ;;
  Darwin-arm64) platform=darwin-arm64 ;;
  *) echo "unsupported platform: $(uname -s) $(uname -m)" >&2; exit 1 ;;
esac

command -v git >/dev/null 2>&1 || { echo "git is required" >&2; exit 1; }
command -v cargo >/dev/null 2>&1 || { echo "cargo is required" >&2; exit 1; }

if [ ! -d "$source_dir" ]; then
  git clone --depth 1 --branch "v$version" https://github.com/linebender/resvg "$source_dir"
fi

tag=$(git -C "$source_dir" describe --tags --exact-match)
[ "$tag" = "v$version" ] || { echo "resvg cache is not v$version: $tag" >&2; exit 1; }

native_libs=$(cargo rustc --release -p resvg-capi --manifest-path "$source_dir/Cargo.toml" -- --print native-static-libs 2>&1 | sed -n 's/.*native-static-libs: //p' | tail -n 1)
[ -n "$native_libs" ] || { echo "failed to determine Rust native link libraries" >&2; exit 1; }

output="$project_root/internal/resvg/lib/$platform"
mkdir -p "$output"
cp "$source_dir/target/release/libresvg.a" "$output/libresvg.a"
cp "$source_dir/crates/c-api/resvg.h" "$project_root/internal/resvg/resvg.h"

printf '%s\n' "$native_libs" > "$output/native-static-libs.txt"

echo "resvg $version built at $output"
