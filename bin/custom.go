package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// copyDirContents copies the contents of src into dst without creating the src folder itself.
func copyDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Make destination directory for this sub-directory.
			if err := os.MkdirAll(dstPath, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dstPath, err)
			}
			if err := copyDirContents(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer srcFile.Close()

	// Ensure the destination directory exists.
	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directories for %s: %w", dst, err)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %w", src, dst, err)
	}

	// Copy file mode
	if fi, err := srcFile.Stat(); err == nil {
		if chmodErr := os.Chmod(dst, fi.Mode()); chmodErr != nil {
			// Log an error but don't stop execution.
			fmt.Printf("Warning: failed to set permissions for %s: %v\n", dst, chmodErr)
		}
	}
	return nil
}

func main() {
	baseline := "baseline"
	storefront := "storefront"
	custom := "custom"
	targetConfig := filepath.Join(storefront, "nuxt.config.ts")

	// Copy baseline to storefront if the target config file does not exist.
	if _, err := os.Stat(targetConfig); os.IsNotExist(err) {
		fmt.Println("Copying baseline to storefront...")
		if err := copyDirContents(baseline, storefront); err != nil {
			fmt.Printf("Error copying baseline: %v\n", err)
			return
		}
	}

	// List of directories to override.
	overrideDirs := []string{"public", "components", "pages", "layouts", "utils"}

	for _, dir := range overrideDirs {
		src := filepath.Join(custom, dir)
		var dst string
		if dir == "public" {
			dst = filepath.Join(storefront, "public")
		} else {
			dst = filepath.Join(storefront, "app", dir)
		}

		// Only override if the source directory exists.
		if _, err := os.Stat(src); err == nil {
			fmt.Printf("Overriding %s -> %s...\n", src, dst)
			if err := copyDirContents(src, dst); err != nil {
				fmt.Printf("Error overriding %s: %v\n", dir, err)
			}
		}
	}

	// Copy `custom/pocketstore.json` -> `storefront/app/pocketstore.json` if it exists.
	pocketstoreSrc := filepath.Join(custom, "pocketstore.json")
	pocketstoreDst := filepath.Join(storefront, "app", "pocketstore.json")
	if _, err := os.Stat(pocketstoreSrc); err == nil {
		fmt.Println("Copying custom/pocketstore.json to storefront...")
		if err := copyFile(pocketstoreSrc, pocketstoreDst); err != nil {
			fmt.Printf("Error copying pocketstore.json: %v\n", err)
		}
	}

	// Copy `custom/daisyui.css` -> `storefront/daisyui.css` if it exists.
	daisySrc := filepath.Join(custom, "daisyui.css")
	daisyDst := filepath.Join(storefront, "daisyui.css")
	if _, err := os.Stat(daisySrc); err == nil {
		fmt.Println("Copying custom/daisyui.css to storefront...")
		if err := copyFile(daisySrc, daisyDst); err != nil {
			fmt.Printf("Error copying daisyui.css: %v\n", err)
		}
	}

	fmt.Println("Copy complete.")
}