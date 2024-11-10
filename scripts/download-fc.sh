#!/bin/bash

set -e

# Create bin directory if it doesn't exist
mkdir -p bin

# Download the latest Firecracker binary
if [ ! -f bin/firecracker ]; then
    echo "Downloading Firecracker binary..."
    curl -Lo bin/firecracker https://github.com/firecracker-microvm/firecracker/releases/download/v1.10.0/firecracker-v1.10.0-x86_64.tgz
    chmod +x bin/firecracker
else
    echo "Firecracker binary already exists."
fi

