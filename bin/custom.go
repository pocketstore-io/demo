package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// copyDirContents copies the contents of src into dst without creating the src folder itself
func copyDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
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

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Ensure destination directory exists
	err = os.MkdirAll(filepath.Dir(dst), os.ModePerm)
	if err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// Copy file mode
	fi, err := srcFile.Stat()
	if err == nil {
		_ = os.Chmod(dst, fi.Mode())
	}
	return nil
}

func main() {
	baseline := "baseline"
	storefront := "storefront"
	custom := "custom"
	targetConfig := filepath.Join(storefront, "nuxt.config.ts")

	copyDirContents(baseline,storefront)

	// Copy baseline if storefront does not exist
	if _, err := os.Stat(targetConfig); os.IsNotExist(err) {
		fmt.Println("Copying baseline to storefront...")
		err := copyDirContents(baseline, storefront)
		if err != nil {
			fmt.Printf("Error copying baseline: %v\n", err)
			return
		}
	}

	overrideDirs := []string{"public", "components", "pages", "layouts", "utils"}

        for _, dir := range overrideDirs {
            src := filepath.Join(custom, dir)

            // special case for public
            var dst string
            if dir == "public" {
                dst = filepath.Join(storefront, "public")
            } else {
                dst = filepath.Join(storefront, "app", dir)
            }

            if _, err := os.Stat(src); err == nil {
                fmt.Printf("Overriding %s -> %s...\n", src, dst)
                err := copyDirContents(src, dst)
                if err != nil {
                    fmt.Printf("Error overriding %s: %v\n", dir, err)
                }
            }
        }

	// Copy custom/pocketstore.json -> storefront/pocketstore.json if exists
	pocketstoreSrc := filepath.Join(custom, "pocketstore.json")
	pocketstoreDst := filepath.Join(storefront,"app", "pocketstore.json")
	if _, err := os.Stat(pocketstoreSrc); err == nil {
		fmt.Println("Copying custom/pocketstore.json to storefront...")
		err := copyFile(pocketstoreSrc, pocketstoreDst)
		if err != nil {
			fmt.Printf("Error copying pocketstore.json: %v\n", err)
		}
	}

	// Copy custom/daisyui.css -> storefront/daisyui.css if exists
	daisySrc := filepath.Join(custom, "daisyui.css")
	daisyDst := filepath.Join(storefront, "daisyui.css")
	if _, err := os.Stat(daisySrc); err == nil {
		fmt.Println("Copying custom/daisyui.css to storefront...")
		err := copyFile(daisySrc, daisyDst)
		if err != nil {
			fmt.Printf("Error copying daisyui.css: %v\n", err)
		}
	}

	// Print working directory for debugging
	if wd, err := os.Getwd(); err == nil {
		fmt.Println("Current working directory:", wd)
	} else {
		fmt.Println("Could not determine current working directory:", err)
	}

	fmt.Println("Copy complete.")
}