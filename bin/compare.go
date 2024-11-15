package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Define the directories
	customDir := "custom"
	storefrontDir := "storefront"

	// Walk through the custom directory recursively
	err := filepath.Walk(customDir, func(path string, info os.FileInfo, err error) error {
		// Skip if there is an error reading the file
		if err != nil {
			return err
		}

		// Skip directories, only process files
		if info.IsDir() {
			return nil
		}

		// Exclude .md files
		if strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		// Get the relative path of the file within the custom directory
		relativePath, err := filepath.Rel(customDir, path)
		if err != nil {
			return err
		}

		// Check if the corresponding file exists in the storefront folder
		storefrontFilePath := filepath.Join(storefrontDir, relativePath)
		if _, err := os.Stat(storefrontFilePath); os.IsNotExist(err) {
			// File doesn't exist in the storefront folder
			fmt.Printf("File %s doesnt exist anymore in %s\n", filepath.Join(customDir, relativePath), storefrontDir)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the directory: %v\n", err)
	}
}
