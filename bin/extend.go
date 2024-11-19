package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	return err
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	// Get the properties of the source directory
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create the destination directory
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return err
	}

	// Read the contents of the source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	// Loop through each entry in the source directory
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectories
			err := copyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			// Copy individual files
			err := copyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func main() {
	// Define the source and destination directories for components and pages
	componentSrc := "custom/components"
	componentDst := "storefront/components"

	pageSrc := "custom/pages"
	pageDst := "storefront/pages"

	layoutsSrc := "custom/layouts"
	layoutDsg := "storefront/layouts"

	// Copy the components directory
	err := copyDir(componentSrc, componentDst)
	if err != nil {
		fmt.Printf("Error copying components: %v\n", err)
		return
	}
	fmt.Println("Successfully copied components!")

	// Copy the pages directory
	err = copyDir(pageSrc, pageDst)
	if err != nil {
		fmt.Printf("Error copying pages: %v\n", err)
		return
	}
	fmt.Println("Successfully copied pages!")

	// Copy the pages directory
	err = copyDir(layoutsSrc, layoutsDst)
	if err != nil {
		fmt.Printf("Error copying layouts: %v\n", err)
		return
	}
	fmt.Println("Successfully copied layouts!")
}
