#!/bin/bash

set -e

# Create bin directory if it doesn't exist
mkdir -p bin

# Fetch the latest kernel version
latest=$(wget "http://spec.ccfc.min.s3.amazonaws.com/?prefix=firecracker-ci/v1.10/x86_64/vmlinux-5.10&list-type=2" -O - 2>/dev/null | grep -oP "(?<=<Key>)(firecracker-ci/v1.10/x86_64/vmlinux-5\.10\.[0-9]{3})(?=</Key>)" | sort -V | tail -n 1)

# Download the latest vmlinux kernel
if [ ! -f "bin/${latest}" ]; then
    echo "Downloading latest vmlinux kernel: ${latest}..."
    wget "https://s3.amazonaws.com/spec.ccfc.min/${latest}" -O "bin/vmlinux"
else
    echo "Latest vmlinux kernel already exists: ${latest}."
fi