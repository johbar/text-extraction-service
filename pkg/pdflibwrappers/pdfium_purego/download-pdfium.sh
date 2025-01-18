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
# default for *nix
path_in_tar='lib'
name_in_tar='libpdfium'

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
  'windows')
    os='win'
    # overwrite for windows:
    path_in_tar='bin'
    name_in_tar='pdfium'
    ext='dll'
    ;;
  *)
    printf "not a supported OS: %s\n" "${os}"
    exit 1;
    ;;
esac

try_to_strip () {
    printf "Trying to strip...\n"
    du -h "${1}"
    if test "${goos}" = 'darwin' && strip -S -x "${1}"; then
      du -h "${1}"
    fi
    if test "${goos}" = 'linux'; then 
      for strip_bin in strip /usr/bin/*strip; do
        "$strip_bin" "${1}" && break || continue
      done
      du -h "${1}"
    fi
}

download () {
  if which curl 2>&1 >/dev/null ; then
    curl -sS --location "$1"
  else 
    wget -q -O - "$1"
  fi
}

url="https://github.com/bblanchon/pdfium-binaries/releases/latest/download/pdfium-${os}-${arch}.tgz"

printf "Downloading %s\n" "${url}"
local_name="${name_in_tar}.${ext}"
(
    mkdir -p "${my_dir}/lib"
    cd "${my_dir}/lib"
    download "${url}" | tar -xz --strip-components 1 "${path_in_tar}/${local_name}"
    printf "Extracted lib to %s\n" "${my_dir}/${name_in_tar}${ext}"
    file "${local_name}" || true
    try_to_strip "${local_name}"
    printf "Done.\n"
)
