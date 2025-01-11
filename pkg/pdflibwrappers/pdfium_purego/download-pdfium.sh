#!/usr/bin/env sh

## 
# Loads the latest pdfium shared lib from Github.
# In honors GOOS and GOARCH but only for combinations of linux and darwin with arm64 and am64
# This scripts aims to be platform independent and should work on MacOS and Linux (glibc or musl).
##

set -o nounset
set -o errexit
# set -x

os=$(go env GOOS)
arch="$(go env GOARCH)"

my_path=$(readlink -f $0)
my_dir=$(dirname "${my_path}")
ext=''


case "${arch}" in
  'amd64')
    arch='x64'
    ;;
  '386')
    arch='x86'
    ;;
  'arm64')
    ;;
  *)
    printf "not a supported architecture: %s\n" "${arch}"
    exit 1;
    ;;
esac

case ${os} in
  'linux')
    ext='so'
    musl="$(ldd '/bin/true' | grep -qF musl && printf '-musl' || true)"
    os="linux${musl}"
    ;;
  'darwin')
    os='mac'
    ext='dylib'
    ;;
  *)
    printf "not a supported OS: %s\n" "${os}"
    exit 1;
    ;;
esac

url="https://github.com/bblanchon/pdfium-binaries/releases/latest/download/pdfium-${os}-${arch}.tgz"

printf "Downloading %s\n" "${url}"
lib_path="lib/libpdfium.${ext}"
(
    cd "${my_dir}"
    wget  -q -O - "${url}" | tar -xzv "${lib_path}"
    printf "Extracted lib to %s/lib/libpdfium.%s\n" "${my_dir}" "${ext}"
    file "${lib_path}"
    du -h "${lib_path}"
    printf "Trying to strip...\n"
    if strip -S -x "${lib_path}"; then
      du -h "${lib_path}"
    fi
    printf "Done.\n"
)
