#!/usr/bin/env sh

## 
# Loads the latest pdfium shared lib from Github for the current platform
# This scripts aims to be platform independent and should work on MacOS and Linux (glibc or musl).

##
set -o nounset
set -o errexit
# set -x

musl="$(ldd '/bin/true' | grep -qF musl && printf '-musl' || true)"
os=$(go env GOOS)
arch="$(go env GOARCH)${musl:-}"

my_path=$(readlink -f $0)
my_dir=$(dirname "$my_path")
ext=''


case "$arch" in
  'amd64')
    arch='x64'
    ;;
  '386')
    arch='x86'
esac

case $os in
  'linux')
    ext='so'
    ;;
  'darwin')
    os='mac'
    ext='dylib'
    ;;
  *)
    printf "not a supported OS: %s\n" "$os"; exit 1;
    ;;
esac

printf "arch=%s\n" "$arch"
printf "os=%s\n" "$os"

url="https://github.com/bblanchon/pdfium-binaries/releases/latest/download/pdfium-${os}-${arch}.tgz"

printf "Downloading %s to %s\n" "$url" "$my_dir"
(
    cd "$my_dir"
    wget  -q -O - "$url" | tar -xzv lib/libpdfium.${ext}
    file lib/libpdfium.${ext}
)
