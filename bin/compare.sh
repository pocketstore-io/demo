#!/bin/bash

# Define the directories
custom_dir="custom"
storefront_dir="storefront"

# Find all regular files in the custom directory, excluding .md files
find "$custom_dir" -type f ! -name "*.md" | while read file; do
  # Get the relative path of the file within the custom directory
  relative_path="${file#$custom_dir/}"

  # Check if the file exists in the storefront folder
  if [ ! -f "$storefront_dir/$relative_path" ]; then
    echo "File $custom_dir/$relative_path doesnt exists anymore in $storefront_dir"
  fi
done
