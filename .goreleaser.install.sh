#!/bin/sh
set -e

# GoReleaser installation script
# This script is meant to be used with GoReleaser's brew tap formula

if [ -z "${BINDIR}" ]; then
  BINDIR="/usr/local/bin"
fi

install_binary() {
  bin_dir="${BINDIR}"
  bin_name="tix"
  
  echo "Installing ${bin_name} to ${bin_dir}"
  
  if [ ! -d "${bin_dir}" ]; then
    mkdir -p "${bin_dir}"
  fi
  
  cp "${bin_name}" "${bin_dir}/${bin_name}"
  chmod +x "${bin_dir}/${bin_name}"
  
  echo "Installation successful!"
}

install_binary 