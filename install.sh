#!/bin/bash

# QueryDB Installation Script
# Builds the binary for the current platform and installs it to /usr/local/bin

set -e  # Exit on any error

echo "🔨 Building QueryDB for current platform..."

# Build the binary
go build -o querydb

echo "✅ Build completed successfully"

# Check if binary was created
if [ ! -f "querydb" ]; then
    echo "❌ Error: Binary 'querydb' was not created"
    exit 1
fi

echo "📦 Installing QueryDB to /usr/local/bin..."

# Install to /usr/local/bin (requires sudo)
sudo cp querydb /usr/local/bin/
sudo chmod +x /usr/local/bin/querydb

echo "🧹 Cleaning up build artifacts..."

# Remove the local binary
rm querydb

echo "✅ Installation completed successfully!"
echo ""
echo "🎉 QueryDB is now installed and available globally"
echo "   Run 'querydb --version' to verify the installation"
echo "   Run 'querydb --help' to see usage information"