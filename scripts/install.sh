#!/bin/bash

# Set the installation directory
INSTALL_DIR="/usr/local/bin"

# Check if the installation directory exists, create it if not
if [ ! -d "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR"
fi

# Set the script name
SCRIPT_NAME="install.sh"

# Set the script path
SCRIPT_PATH=$(readlink -f "$0")

# Check if the script is already installed
if [ -f "$INSTALL_DIR/$SCRIPT_NAME" ]; then
  echo "Script already installed, skipping installation."
  exit 0
fi

# Copy the script to the installation directory
cp "$SCRIPT_PATH" "$INSTALL_DIR/$SCRIPT_NAME"

# Make the script executable
chmod +x "$INSTALL_DIR/$SCRIPT_NAME"

# Print a success message
echo "Script installed successfully!"