package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"os/exec"
)

// copyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory will be created if necessary.
func copyDir(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)
		if d.IsDir() {
			return os.MkdirAll(targetPath, os.ModePerm)
		}
		return copyFile(path, targetPath)
	})
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
	// Optionally, copy file mode
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

	if _, err := os.Stat(targetConfig); os.IsNotExist(err) {
		fmt.Println("Copying baseline to storefront...")
		err := copyDir(baseline, storefront)
		if err != nil {
			fmt.Printf("Error copying baseline: %v\n", err)
			return
		}
	}
	// Override custom/public -> storefront/public
	overrideDirs := []string{"public", "components", "pages", "layouts"}
	for _, dir := range overrideDirs {
		src := filepath.Join(custom, dir)
		dst := filepath.Join(storefront, dir)
		if _, err := os.Stat(src); err == nil {
			fmt.Printf("Overriding %s -> %s...\n", src, dst)
			err := copyDir(src, dst)
			if err != nil {
				fmt.Printf("Error overriding %s: %v\n", dir, err)
			}
		}
	}

	// Copy custom/pocketstore.json -> storefront/pocketstore.json if exists
	pocketstoreSrc := filepath.Join(custom, "pocketstore.json")
	pocketstoreDst := filepath.Join(storefront, "pocketstore.json")
	if _, err := os.Stat(pocketstoreSrc); err == nil {
		fmt.Println("Copying custom/pocketstore.json to storefront...")
		err := copyFile(pocketstoreSrc, pocketstoreDst)
		if err != nil {
			fmt.Printf("Error copying pocketstore.json: %v\n", err)
		}
	}

	// Copy custom/pocketstore.json -> storefront/pocketstore.json if exists
	pocketstoreSrc = filepath.Join(custom, "daisyui.css")
	pocketstoreDst = filepath.Join(storefront, "daisyui.css")
	if _, err := os.Stat(pocketstoreSrc); err == nil {
		fmt.Println("Copying baseline/daisyui.css to storefront...")
		err := copyFile(pocketstoreSrc, pocketstoreDst)
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

	runStep("go run bin/plugins.go", exec.Command("go", "run", "bin/plugins.go"))
	runStep("go run bin/plugins-install.go", exec.Command("go", "run", "bin/plugins-install.go"))
	runStep("go run bin/plugins-merge.go", exec.Command("go", "run", "bin/plugins-merge.go"))

	fmt.Println("Copy complete.")
}

func runStep(name string, cmd *exec.Cmd) {
	fmt.Printf("==> Running: %s\n", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %s: %v\n", name, err)
		os.Exit(1)
	}
	fmt.Printf("==> Done: %s\n", name)
}
