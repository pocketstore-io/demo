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
	pluginRoot = ".plugins/repos"
	dirsToCopy = []string{"pages", "components", "layouts", "public", "utils"}
)

func main() {
	fmt.Println("Reading plugin root directory:", pluginRoot)
	vendorDirs, err := os.ReadDir(pluginRoot)
	if err != nil {
		fmt.Println("Failed to read plugin root:", err)
		return
	}

	var plugins []Plugin

	// Support new two-level layout and legacy single-level layout
	for _, vendorEntry := range vendorDirs {
		if !vendorEntry.IsDir() {
			continue
		}
		vendorPath := filepath.Join(pluginRoot, vendorEntry.Name())

		pluginJsonPath := filepath.Join(vendorPath, "plugin.json")
		if exists(pluginJsonPath) {
			prio := readPrio(pluginJsonPath)
			plugins = append(plugins, Plugin{
				Vendor:   "",
				Name:     vendorEntry.Name(),
				Prio:     prio,
				BasePath: vendorPath,
			})
			continue
		}

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
			if exists(pluginJsonPath) {
				prio := readPrio(pluginJsonPath)
				plugins = append(plugins, Plugin{
					Vendor:   vendorEntry.Name(),
					Name:     p.Name(),
					Prio:     prio,
					BasePath: pluginPath,
				})
			}
		}
	}

	// Sort plugins by priority descending
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Prio > plugins[j].Prio
	})

	// Copy folders for each plugin
    for _, plugin := range plugins {
        fmt.Printf("Processing plugin: %s/%s (prio: %d)\n", plugin.Vendor, plugin.Name, plugin.Prio)

        for _, d := range dirsToCopy {
            src := filepath.Join(plugin.BasePath, d)

            // FIX: public goes to storefront/public, others to storefront/app/<dir>
            var dst string
            if d == "public" {
                dst = filepath.Join("public")           // storefront/public
            } else {
                dst = filepath.Join("app", d)           // storefront/app/<dir>
            }

            if exists(src) {
                finalDst := filepath.Join("storefront", dst)
                fmt.Printf("  Copying %s → %s\n", src, finalDst)

                if err := copyDir(src, finalDst); err != nil {
                    fmt.Printf("  Error copying %s: %v\n", d, err)
                }
            }
        }
    }
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readPrio(jsonPath string) int {
	file, err := os.Open(jsonPath)
	if err != nil {
		return 0
	}
	defer file.Close()
	var pj PluginJson
	if err := json.NewDecoder(file).Decode(&pj); err != nil {
		return 0
	}
	return pj.Prio
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		targetPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}
		return copyFile(path, targetPath)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(0644)
}
