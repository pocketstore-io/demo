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
	Vendor   string
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
	vendorDirs, err := os.ReadDir(pluginRoot)
	if err != nil {
		fmt.Println("Failed to read plugin root:", err)
		return
	}

	var plugins []Plugin

	// Support the new two-level layout: .plugins/repos/<vendor>/<plugin>
	// Also tolerate legacy single-level layout where plugin.json may live directly under .plugins/repos/<plugin>
	for _, vendorEntry := range vendorDirs {
		if !vendorEntry.IsDir() {
			// skip non-dir entries at top-level
			continue
		}
		vendorPath := filepath.Join(pluginRoot, vendorEntry.Name())

		// Check if this vendorPath itself looks like a plugin (contains plugin.json)
		pluginJsonPath := filepath.Join(vendorPath, "plugin.json")
		if exists(pluginJsonPath) {
			prio := readPrio(pluginJsonPath)
			fmt.Printf("Found plugin (legacy single-level) %s with prio %d at %s\n", vendorEntry.Name(), prio, vendorPath)
			plugins = append(plugins, Plugin{
				Vendor:   "",
				Name:     vendorEntry.Name(),
				Prio:     prio,
				BasePath: vendorPath,
			})
			// continue scanning other top-level entries
			continue
		}

		// Otherwise treat vendorEntry as a vendor and look for plugins inside it
		pluginDirs, err := os.ReadDir(vendorPath)
		if err != nil {
			fmt.Printf("Failed to read vendor dir %s: %v\n", vendorPath, err)
			continue
		}
		for _, p := range pluginDirs {
			if !p.IsDir() {
				continue
			}
			pluginPath := filepath.Join(vendorPath, p.Name())
			pluginJsonPath := filepath.Join(pluginPath, "plugin.json")
			fmt.Printf("Checking for plugin.json in %s\n", pluginPath)
			if exists(pluginJsonPath) {
				prio := readPrio(pluginJsonPath)
				fmt.Printf("Found plugin %s/%s with prio %d\n", vendorEntry.Name(), p.Name(), prio)
				plugins = append(plugins, Plugin{
					Vendor:   vendorEntry.Name(),
					Name:     p.Name(),
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
		id := plugin.Name
		if plugin.Vendor != "" {
			id = plugin.Vendor + "/" + plugin.Name
		}
		fmt.Printf("Processing plugin: %s (prio: %d) at %s\n", id, plugin.Prio, plugin.BasePath)
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
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		fmt.Printf("      Error creating destination dir for %s: %v\n", dst, err)
		return err
	}
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