#!/bin/bash

set -e

echo "Building memadvise..."

# Get dependencies
echo "Getting dependencies..."
go mod tidy

# Build the binary
echo "Building binary..."
go build -o memadvise

echo "Done! Binary is at: $(pwd)/memadvise" 