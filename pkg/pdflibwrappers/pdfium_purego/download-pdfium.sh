#!/usr/bin/env sh

## 
# Loads the latest pdfium shared lib from Github.
# In honors GOOS and GOARCH but only for combinations of linux and darwin with arm64 and am64
# This scripts aims to be platform independent and should work on MacOS and Linux (glibc or musl).
##

set -o nounset
set -o errexit
# set -x

goos=$(go env GOOS)
arch=$(go env GOARCH)
linux_arch="${arch}"

my_path=$(readlink -f $0)
my_dir=$(dirname "${my_path}")
ext=''

# translate GOARCH to PDFium and Linux architecture IDs
# and abort if not amd64 or arm64
case "${arch}" in
  'amd64')
    arch='x64'
    linux_arch='x86_64'
    ;;
  'arm64')
    linux_arch='aarch64'
    ;;
  *)
    printf "not a supported architecture: %s\n" "${arch}"
    exit 1;
    ;;
esac

# translate GOOS to OS identifiers used on PDFium download page
# also set the dynamic lib file extension and find out if it is a Musl distro
case ${goos} in
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

try_to_strip () {
    du -h "${lib_path}"
    printf "Trying to strip...\n"
    if test "${os}" = 'mac' && strip -S -x "${1}"; then
      du -h "${1}"
    fi
    if test "${goos}" = 'linux' && /usr/bin/${linux_arch}-linux-gnu-strip "${1}"; then 
      du -h "${1}"
    fi
}

url="https://github.com/bblanchon/pdfium-binaries/releases/latest/download/pdfium-${os}-${arch}.tgz"

printf "Downloading %s\n" "${url}"
lib_path="lib/libpdfium.${ext}"
(
    cd "${my_dir}"
    wget  -q -O - "${url}" | tar -xzv "${lib_path}"
    printf "Extracted lib to %s/lib/libpdfium.%s\n" "${my_dir}" "${ext}"
    file "${lib_path}" || true
    try_to_strip "${lib_path}"
    printf "Done.\n"
)
