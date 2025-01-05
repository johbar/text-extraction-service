#!/usr/bin/env sh

## 
# Loads the latest pdfium shared lib from Github for the current platform
# This scripts aims to be platform independent and should work on MacOS and Linux (glibc or musl).

##
set -o nounset
set -o errexit
# set -x

os=$(uname)
arch=$(uname -m)
musl="$(ldd '/bin/true' | grep -qF musl && printf '-musl' || true)"

my_path=$(readlink -f $0)
my_dir=$(dirname "$my_path")
ext=''

if test "$arch" = 'x86_64' ; then
    arch='x64'
fi

case $os in
  'Linux')
    os="linux${musl}"
    ext='so'
    ;;
  'Darwin')
    os='mac'
    ext='dylib'
    ;;
esac

printf "arch=%s\n" "$arch"
printf "os=%s\n" "$os"
printf "my_dir=%s\n" "$my_dir\n"

url="https://github.com/bblanchon/pdfium-binaries/releases/latest/download/pdfium-${os}-${arch}.tgz"

printf "Downloading %s to %s\n" "$url" "$my_dir"
(
    cd "$my_dir"
    wget  -q -O - "$url" | tar -xzv lib/libpdfium.${ext}
)
