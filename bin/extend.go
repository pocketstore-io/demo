package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer input.Close()

	output, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	if err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %w", src, dst, err)
	}

	return nil
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	// Get the properties of the source directory
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get properties of source directory %s: %w", src, err)
	}

	// Create the destination directory
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dst, err)
	}

	// Read the contents of the source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read contents of source directory %s: %w", src, err)
	}

	// Loop through each entry in the source directory
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectories
			if err := copyDir(srcPath, dstPath); err != nil {
				fmt.Printf("Error copying directory %s: %v\n", srcPath, err)
			}
		} else {
			// Copy individual files
			if err := copyFile(srcPath, dstPath); err != nil {
				fmt.Printf("Error copying file %s: %v\n", srcPath, err)
			}
		}
	}

	return nil
}

func main() {
	// Define the source and destination directories
	directories := []struct {
		Storefront string
		Baseline   string
		Custom     string
		Name       string
	}{
		{"storefront/components", "baseline/components", "custom/components", "components"},
		{"storefront/pages", "baseline/pages", "custom/pages", "pages"},
		{"storefront/layouts", "baseline/layouts", "custom/layouts", "layouts"},
		{"storefront/public", "baseline/public", "custom/public", "public"},
	}

	copyDir("baseline", "storefront")

	// Iterate through each source-destination pair and copy directories
	for _, dir := range directories {
		fmt.Printf("Copying %s...\n", dir.Name)
		if err := copyDir(dir.Baseline, dir.Storefront); err != nil {
			fmt.Printf("Error copying %s: %v\n", dir.Name, err)
		} else {
			fmt.Printf("Successfully copied %s!\n", dir.Name)
		}
	}

	// Copy the pocketstore.json file
	srcFile := "custom/pocketstore.json"
	dstFile := "storefront/pocketstore.json"
	fmt.Println("Copying pocketstore.json...")
	if err := copyFile(srcFile, dstFile); err != nil {
		fmt.Printf("Error copying pocketstore.json: %v\n", err)
	} else {
		fmt.Println("Successfully copied pocketstore.json!")
	}

	cmd := exec.Command("go", "run", "bin/daisyui.go")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if err != nil {
		os.Exit(1)
	}

	fmt.Println("All copy operations completed.")
}
