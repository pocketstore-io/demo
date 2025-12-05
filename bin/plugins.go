package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Plugin struct {
	Version  string `json:"version"`
	Name     string `json:"name"`
	Vendor   string `json:"vendor"`
	Prio     int    `json:"prio,omitempty"`
	BasePath string `json:"-"`
}

type PluginJson struct {
	Prio int `json:"prio"`
}

var (
	pluginRoot = ".plugins/repos"
	dirsToCopy = []string{"pages", "components", "layouts", "public", "utils"}
)

// readPluginsFromFile reads and parses a plugins JSON file
func readPluginsFromFile(filePath string) ([]Plugin, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var plugins []Plugin
	if err := json.Unmarshal(data, &plugins); err != nil {
		return nil, err
	}
	return plugins, nil
}

// exists checks if a file or directory exists
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readPrio reads the priority from a plugin.json file
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

// copyDir recursively copies a directory
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

// copyFile copies a file from src to dst
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

// DownloadFile downloads a file from the given URL and saves it to the given filepath
func DownloadFile(filepathDest string, url string) error {
	fmt.Printf("[debug] DownloadFile called: url=%s dest=%s\n", url, filepathDest)
	out, err := os.Create(filepathDest)
	if err != nil {
		fmt.Printf("[error] failed to create file %s: %v\n", filepathDest, err)
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("[error] http.Get failed for %s: %v\n", url, err)
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("[debug] http.Get response status: %s for url=%s\n", resp.Status, url)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		fmt.Printf("[error] io.Copy failed while writing %s: %v\n", filepathDest, err)
		return err
	}
	fmt.Printf("[debug] DownloadFile wrote %d bytes to %s\n", written, filepathDest)
	return nil
}

// Unzip extracts a zip archive to a specified destination
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	// Find common prefix (strip top-level directory from GitHub zips)
	var prefix string
	if len(r.File) > 0 {
		firstPath := r.File[0].Name
		if strings.Contains(firstPath, "/") {
			prefix = strings.Split(firstPath, "/")[0] + "/"
		}
	}

	for _, f := range r.File {
		// Strip the prefix from the path
		relativePath := f.Name
		if prefix != "" && strings.HasPrefix(f.Name, prefix) {
			relativePath = strings.TrimPrefix(f.Name, prefix)
		}

		// Skip if we've stripped everything (root directory itself)
		if relativePath == "" {
			continue
		}

		fpath := filepath.Join(dest, relativePath)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

// FetchLatestVersion queries the plugin API for the latest version string
func FetchLatestVersion(vendor, name string) (string, error) {
	url := fmt.Sprintf("https://download.pocketstore.io/d/plugins/%s/%s/latest.zip", vendor, name)
	fmt.Printf("[debug] FetchLatestVersion HEAD %s\n", url)
	resp, err := http.Head(url)
	if err != nil {
		fmt.Printf("[error] http.Head failed for %s: %v\n", url, err)
		return "", err
	}
	defer resp.Body.Close()

	fmt.Printf("[debug] HEAD response status for %s: %s\n", url, resp.Status)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest: %s", resp.Status)
	}

	// Just return "latest"
	return "latest", nil
}

// isSpecialLatestVersion checks if the version string means "latest"
func isSpecialLatestVersion(version string) bool {
	version = strings.ToLower(version)
	if version == "latest" {
		return true
	}
	// Match version pattern like 0.0.1.3 (four numbers separated by dots)
	matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, version)
	return matched
}

// Step 1: mergePlugins merges baseline and custom plugins into a unique list
func mergePlugins() error {
	fmt.Println("==> Step 1: Merging plugins")

	baselinePlugins, err := readPluginsFromFile("baseline/plugins.json")
	if err != nil {
		return fmt.Errorf("error reading baseline/plugins.json: %v", err)
	}

	customPlugins, err := readPluginsFromFile("custom/plugins.json")
	if err != nil {
		return fmt.Errorf("error reading custom/plugins.json: %v", err)
	}

	merged := append(baselinePlugins, customPlugins...)

	unique := make(map[string]Plugin)
	for _, p := range merged {
		key := p.Name + ":" + p.Vendor
		unique[key] = Plugin{
			Version: p.Version,
			Name:    p.Name,
			Vendor:  p.Vendor,
		}
	}

	// Convert map to slice
	result := make([]Plugin, 0, len(unique))
	for _, p := range unique {
		result = append(result, p)
	}

	// Sort alphabetically by vendor, then by name
	sort.Slice(result, func(i, j int) bool {
		if result[i].Vendor == result[j].Vendor {
			return result[i].Name < result[j].Name
		}
		return result[i].Vendor < result[j].Vendor
	})

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling merged plugins: %v", err)
	}

	pluginsDir := ".plugins"
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("error creating .plugins directory: %v", err)
	}

	outputFile := filepath.Join(pluginsDir, "installed.json")
	if err := os.WriteFile(outputFile, out, 0644); err != nil {
		return fmt.Errorf("error writing to %s: %v", outputFile, err)
	}

	fmt.Printf("Unique merged plugin list written to %s\n", outputFile)
	return nil
}

// Step 2: installPlugins downloads and installs plugins from the installed.json file
func installPlugins() error {
	fmt.Println("\n==> Step 2: Installing plugins")

	installedPath := ".plugins/installed.json"
	fmt.Printf("[debug] reading installed plugins file: %s\n", installedPath)
	pluginsJSON, err := os.ReadFile(installedPath)
	if err != nil {
		return fmt.Errorf("error reading installed plugins file: %v", err)
	}

	var plugins []Plugin
	if err := json.Unmarshal(pluginsJSON, &plugins); err != nil {
		return fmt.Errorf("error parsing plugins JSON: %v", err)
	}
	fmt.Printf("[debug] parsed %d plugins from %s\n", len(plugins), installedPath)
	for i, p := range plugins {
		fmt.Printf("[debug] plugin[%d] = %+v\n", i, p)
	}

	cacheDir := ".plugins/cache"
	fmt.Printf("[debug] ensuring cache dir exists: %s\n", cacheDir)
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating cache directory: %v", err)
	}

	for _, plugin := range plugins {
		fmt.Printf("[debug] processing plugin: vendor=%s name=%s version=%s\n", plugin.Vendor, plugin.Name, plugin.Version)
		pluginVersion := plugin.Version
		// Remove v- prefix if it exists and version is "v-latest" or "v-LATEST" etc
		if strings.ToLower(pluginVersion) == "v-latest" {
			fmt.Printf("[debug] normalizing v-latest -> latest for %s/%s\n", plugin.Vendor, plugin.Name)
			pluginVersion = "latest"
		}
		if isSpecialLatestVersion(pluginVersion) {
			fmt.Printf("[debug] pluginVersion %q considered special/latest for %s/%s\n", pluginVersion, plugin.Vendor, plugin.Name)
			ver, err := FetchLatestVersion(plugin.Vendor, plugin.Name)
			if err != nil {
				return fmt.Errorf("failed to fetch latest version for %s/%s: %v", plugin.Vendor, plugin.Name, err)
			}
			pluginVersion = ver
			fmt.Printf("[debug] Resolved latest version for %s/%s: %s\n", plugin.Vendor, plugin.Name, pluginVersion)
		}

		url := fmt.Sprintf("https://download.pocketstore.io/d/plugins/%s/%s/%s.zip", plugin.Vendor, plugin.Name, pluginVersion)
		zipPath := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-%s.zip", plugin.Vendor, plugin.Name, pluginVersion))

		// Ensure each plugin is extracted into its own directory under .plugins/repos/<vendor>/<name>
		destDir := filepath.Join(".plugins", "repos", plugin.Vendor, plugin.Name)
		fmt.Printf("[debug] plugin destination dir: %s\n", destDir)

		// Remove any existing contents in the plugin destDir so old files do not persist
		if err := os.RemoveAll(destDir); err != nil {
			// Non-fatal: warn and continue
			fmt.Printf("[warn] failed to remove existing dest dir %s: %v\n", destDir, err)
		}

		fmt.Printf("[info] Downloading %s...\n", url)
		if err := DownloadFile(zipPath, url); err != nil {
			_ = os.Remove(zipPath)
			return fmt.Errorf("failed to download %s: %v", url, err)
		}

		fmt.Printf("[info] Unzipping to %s...\n", destDir)
		if err := Unzip(zipPath, destDir); err != nil {
			_ = os.Remove(zipPath)
			_ = os.RemoveAll(destDir)
			return fmt.Errorf("failed to unzip %s: %v", zipPath, err)
		}

		fmt.Printf("[info] Installed %s/%s %s into %s\n", plugin.Vendor, plugin.Name, pluginVersion, destDir)
	}
	return nil
}

// Step 3: mergePluginFiles merges plugin files into the storefront directory
func mergePluginFiles() error {
	fmt.Println("\n==> Step 3: Merging plugin files")

	fmt.Println("Reading plugin root directory:", pluginRoot)
	vendorDirs, err := os.ReadDir(pluginRoot)
	if err != nil {
		return fmt.Errorf("failed to read plugin root: %v", err)
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

			// public goes to storefront/public, others to storefront/app/<dir>
			var dst string
			if d == "public" {
				dst = filepath.Join("public")
			} else {
				dst = filepath.Join("app", d)
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
	return nil
}

func main() {
	// Step 1: Merge baseline and custom plugins
	if err := mergePlugins(); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	// Step 2: Install plugins
	if err := installPlugins(); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	// Step 3: Merge plugin files into storefront
	if err := mergePluginFiles(); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n==> All plugin steps complete!")
}
