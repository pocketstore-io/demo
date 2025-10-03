package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type Plugin struct {
	Name     string
	Prio     int
	BasePath string
}

type PluginJson struct {
	Prio int `json:"prio"`
}

var (
	pluginRoot     = ".plugins/repos"
	storefrontRoot = "storefront"
	dirsToCopy     = []string{"pages", "components", "layouts", "public", "translations"}
)

func main() {
	fmt.Println("Reading plugin root directory:", pluginRoot)
	pluginDirs, err := os.ReadDir(pluginRoot)
	if err != nil {
		fmt.Println("Failed to read plugin root:", err)
		return
	}

	var plugins []Plugin

	// Collect plugin info and prio
	for _, entry := range pluginDirs {
		if entry.IsDir() {
			pluginPath := filepath.Join(pluginRoot, entry.Name())
			pluginJsonPath := filepath.Join(pluginPath, "plugin.json")
			fmt.Printf("Checking for plugin.json in %s\n", pluginPath)
			if exists(pluginJsonPath) {
				prio := readPrio(pluginJsonPath)
				fmt.Printf("Found plugin %s with prio %d\n", entry.Name(), prio)
				plugins = append(plugins, Plugin{
					Name:     entry.Name(),
					Prio:     prio,
					BasePath: pluginPath,
				})
			} else {
				fmt.Printf("No plugin.json found in %s, skipping\n", pluginPath)
			}
		}
	}

	fmt.Println("Sorting plugins by prio desc")
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Prio > plugins[j].Prio
	})

	// Copy folders for each plugin in order
	for _, plugin := range plugins {
		fmt.Printf("Processing plugin: %s (prio: %d)\n", plugin.Name, plugin.Prio)
		for _, d := range dirsToCopy {
			src := filepath.Join(plugin.BasePath, d)
			dst := filepath.Join(storefrontRoot, d)
			fmt.Printf("  Checking if %s exists in plugin\n", src)
			if exists(src) {
				fmt.Printf("  Copying from %s to %s\n", src, dst)
				if err := copyDir(src, dst); err != nil {
					fmt.Printf("  Error copying %s: %v\n", d, err)
				} else {
					fmt.Printf("  Successfully copied %s\n", d)
				}
			} else {
				fmt.Printf("  Directory %s does not exist, skipping\n", d)
			}
		}
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readPrio(jsonPath string) int {
	fmt.Printf("  Reading prio from %s\n", jsonPath)
	file, err := os.Open(jsonPath)
	if err != nil {
		fmt.Printf("    Error opening %s: %v\n", jsonPath, err)
		return 0
	}
	defer file.Close()
	var pj PluginJson
	if err := json.NewDecoder(file).Decode(&pj); err != nil {
		fmt.Printf("    Error decoding JSON in %s: %v\n", jsonPath, err)
		return 0
	}
	return pj.Prio
}

func copyDir(src, dst string) error {
	fmt.Printf("    Walking directory %s\n", src)
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("    Error walking %s: %v\n", path, err)
			return err
		}
		rel, _ := filepath.Rel(src, path)
		targetPath := filepath.Join(dst, rel)
		if info.IsDir() {
			fmt.Printf("    Creating directory %s\n", targetPath)
			return os.MkdirAll(targetPath, 0755)
		}
		fmt.Printf("    Copying file %s to %s\n", path, targetPath)
		return copyFile(path, targetPath)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		fmt.Printf("      Error opening source file %s: %v\n", src, err)
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		fmt.Printf("      Error creating destination file %s: %v\n", dst, err)
		return err
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		fmt.Printf("      Error copying data from %s to %s: %v\n", src, dst, err)
		return err
	}
	if err := out.Chmod(0644); err != nil {
		fmt.Printf("      Error setting permissions on %s: %v\n", dst, err)
		return err
	}
	return nil
}
