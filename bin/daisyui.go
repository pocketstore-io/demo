package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}

func main() {
	baseDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	baselinePath := filepath.Join(baseDir, "baseline", "daisyui.css")
	customPath := filepath.Join(baseDir, "custom", "daisyui.css")
	storefrontPath := filepath.Join(baseDir, "storefront", "daisyui.css")

	// Copy from baseline
	if _, err := os.Stat(baselinePath); err == nil {
		err = copyFile(baselinePath, storefrontPath)
		if err != nil {
			fmt.Printf("Failed to copy from baseline: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Copied baseline/daisyui.css to storefront/daisyui.css")
	} else {
		fmt.Println("baseline/daisyui.css does not exist!")
		os.Exit(1)
	}

	// Overwrite with custom if exists
	if _, err := os.Stat(customPath); err == nil {
		err = copyFile(customPath, storefrontPath)
		if err != nil {
			fmt.Printf("Failed to copy from custom: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Custom daisyui.css found. Overwrote storefront/daisyui.css with custom/daisyui.css")
	}
}
