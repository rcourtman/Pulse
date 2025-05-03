#!/bin/bash

# Check if a filename is provided
if [ -z "$1" ]; then
  echo "Usage: $0 <filename>"
  exit 1
fi

# Check if the file exists
if [ ! -f "$1" ]; then
  echo "Error: File '$1' not found."
  exit 1
fi

# Use awk to remove lines starting with optional whitespace then //
# This does NOT remove comments appearing after code on the same line.
# Note: This creates a temporary file for safety.
awk '!/^[[:space:]]*\/\// {print}' "$1" > "$1.tmp" && mv "$1.tmp" "$1"

echo "Removed full-line comments (potentially indented) from '$1'." 