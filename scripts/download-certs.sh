#!/bin/bash

# Script to download CA certificates locally for Docker use
# This script downloads root CA certificates that can be copied into Docker images
# instead of using 'apk add ca-certificates' during build time

set -e

# Define the directory where certificates will be stored
CERT_DIR="./certs"
CERT_BUNDLE_FILE="$CERT_DIR/ca-bundle.crt"

# Create the certs directory if it doesn't exist
mkdir -p "$CERT_DIR"

echo "Downloading CA certificates..."

# Download the Mozilla CA bundle (commonly used root certificates)
curl -fsSL -o "$CERT_BUNDLE_FILE" https://curl.se/ca/cacert.pem

if [ -f "$CERT_BUNDLE_FILE" ]; then
    echo "✓ CA bundle downloaded successfully to $CERT_BUNDLE_FILE"
    echo "  Size: $(du -h "$CERT_BUNDLE_FILE" | cut -f1)"
else
    echo "✗ Failed to download CA certificates"
    exit 1
fi

# Also download individual certificate files (Alpine-style structure)
echo "Downloading individual certificates..."

CERT_CACHE_DIR="$CERT_DIR/ca-certificates"
mkdir -p "$CERT_CACHE_DIR"

# Create a symlink to the bundle with the typical Alpine location
mkdir -p "$CERT_DIR/etc/ssl/certs"
cp "$CERT_BUNDLE_FILE" "$CERT_DIR/etc/ssl/certs/ca-certificates.crt"

echo "✓ Certificates ready for Docker COPY"
echo ""
echo "Next steps:"
echo "1. Update your Dockerfile to use:"
echo "   COPY certs/etc/ssl/certs /etc/ssl/certs/"
echo "2. Or copy the full bundle:"
echo "   COPY certs/ca-bundle.crt /etc/ssl/certs/ca-certificates.crt"

