package main

import (
	"archive/zip"
	"crypto/sha1"
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
	Revision string `json:"revision,omitempty"`
	Source   string `json:"-"` // Track source: "baseline", "custom", "storefront", or parent plugin key
}

type PluginMeta struct {
	Vendor       string   `json:"vendor"`
	Name         string   `json:"name"`
	Version      string   `json:"version,omitempty"`
	Type         string   `json:"type,omitempty"`
	Prio         int      `json:"prio,omitempty"`
	Revision     string   `json:"revision,omitempty"`
	Requirements []string `json:"requirements,omitempty"`
}

var (
	pluginRoot = ".plugins/repos"
	dirsToCopy = []string{"pages", "components", "layouts", "public", "utils"}
)

// parsePluginURL extracts vendor and name from URLs like:
// "github.com/pocketstore-io/plugin-image-slider" -> ("pocketstore-io", "image-slider")
// "github.com/pocketstore-io/reviews" -> ("pocketstore-io", "reviews")
func parsePluginURL(url string) (vendor, name string, ok bool) {
	parts := strings.Split(url, "/")
	if len(parts) < 3 {
		return "", "", false
	}
	vendor = parts[len(parts)-2]
	name = parts[len(parts)-1]
	// Remove "plugin-" prefix if present
	name = strings.TrimPrefix(name, "plugin-")
	return vendor, name, true
}

// readPluginMeta reads plugin.json from the repos directory
func readPluginMeta(vendor, name string) (*PluginMeta, error) {
	pluginPath := filepath.Join(".plugins", "repos", vendor, name, "plugin.json")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		return nil, err
	}

	var meta PluginMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// resolveRequirements recursively resolves all plugin requirements
func resolveRequirements(baselinePlugins, customPlugins, storefrontPlugins []Plugin) ([]Plugin, error) {
	seen := make(map[string]bool)
	result := make([]Plugin, 0)
	queue := make([]Plugin, 0)
	dependencyTree := make(map[string][]string)   // parent -> children
	sourceMap := make(map[string]string)          // plugin key -> source
	isRoot := make(map[string]bool)

	// Helper to add plugins from a source
	addPlugins := func(plugins []Plugin, source string) {
		for _, p := range plugins {
			key := p.Vendor + "/" + p.Name
			if !seen[key] {
				p.Source = source
				seen[key] = true
				queue = append(queue, p)
				sourceMap[key] = source
				isRoot[key] = true
			}
		}
	}

	// Add plugins in priority order
	addPlugins(baselinePlugins, "baseline")
	addPlugins(customPlugins, "custom")
	addPlugins(storefrontPlugins, "storefront")

	// BFS traversal to resolve all dependencies
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		currentKey := current.Vendor + "/" + current.Name

		// Try to read plugin.json for requirements
		meta, err := readPluginMeta(current.Vendor, current.Name)
		if err != nil {
			// Plugin not yet downloaded, will be downloaded in install phase
			continue
		}

		// Process requirements
		for _, req := range meta.Requirements {
			vendor, name, ok := parsePluginURL(req)
			if !ok {
				fmt.Printf("Warning: invalid requirement URL: %s\n", req)
				continue
			}

			key := vendor + "/" + name
			dependencyTree[currentKey] = append(dependencyTree[currentKey], key)

			if seen[key] {
				continue // Already processed or queued
			}

			seen[key] = true
			newPlugin := Plugin{
				Vendor:  vendor,
				Name:    name,
				Version: "latest",
				Source:  currentKey, // Track which plugin required this
			}
			sourceMap[key] = currentKey
			queue = append(queue, newPlugin)
			fmt.Printf("  [%s] requires → %s\n", currentKey, key)
		}
	}

	// Print dependency tree with sources
	if len(dependencyTree) > 0 {
		fmt.Println("\n==> Dependency Tree:")
		visited := make(map[string]bool)

		// Print baseline plugins
		if len(baselinePlugins) > 0 {
			fmt.Println("\n📦 FROM baseline/plugins.json:")
			for _, root := range baselinePlugins {
				key := root.Vendor + "/" + root.Name
				printNodeWithSource(dependencyTree, sourceMap, key, "  ", visited, true, true)
			}
		}

		// Print custom plugins
		if len(customPlugins) > 0 {
			fmt.Println("\n📦 FROM custom/plugins.json:")
			for _, root := range customPlugins {
				key := root.Vendor + "/" + root.Name
				printNodeWithSource(dependencyTree, sourceMap, key, "  ", visited, true, true)
			}
		}

		// Print storefront plugins
		if len(storefrontPlugins) > 0 {
			fmt.Println("\n📦 FROM storefront/plugins.json:")
			for _, root := range storefrontPlugins {
				key := root.Vendor + "/" + root.Name
				printNodeWithSource(dependencyTree, sourceMap, key, "  ", visited, true, true)
			}
		}
	}

	return result, nil
}

// printNodeWithSource prints a visual tree node with source information
func printNodeWithSource(tree map[string][]string, sourceMap map[string]string, key string, prefix string, visited map[string]bool, isLast bool, isRoot bool) {
	marker := "├──"
	if isLast {
		marker = "└──"
	}

	if isRoot {
		fmt.Printf("%s%s\n", prefix, key)
	} else {
		source := sourceMap[key]
		sourceLabel := ""
		if source != "" && source != "baseline" && source != "custom" && source != "storefront" {
			sourceLabel = fmt.Sprintf(" (required by: %s)", source)
		}
		fmt.Printf("%s%s %s%s\n", prefix, marker, key, sourceLabel)
	}

	children := tree[key]
	if len(children) == 0 {
		return
	}

	// Prevent infinite recursion on circular dependencies
	if visited[key] {
		return
	}
	visited[key] = true

	newPrefix := prefix
	if isLast {
		newPrefix += "    "
	} else {
		newPrefix += "│   "
	}

	for i, child := range children {
		printNodeWithSource(tree, sourceMap, child, newPrefix, visited, i == len(children)-1, false)
	}
}

// savePlugins writes the plugin list to installed.json
func savePlugins(plugins []Plugin) error {
	data, err := json.MarshalIndent(plugins, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling plugins: %v", err)
	}

	pluginsDir := ".plugins"
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("error creating .plugins directory: %v", err)
	}

	outputFile := filepath.Join(pluginsDir, "installed.json")
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("error writing to %s: %v", outputFile, err)
	}

	return nil
}

// loadPluginsFromFile reads plugins from a specific JSON file
func loadPluginsFromFile(filePath string) ([]Plugin, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Plugin{}, nil // Return empty list if file doesn't exist
		}
		return nil, fmt.Errorf("error reading %s: %v", filePath, err)
	}

	var plugins []Plugin
	if err := json.Unmarshal(data, &plugins); err != nil {
		return nil, fmt.Errorf("error parsing %s: %v", filePath, err)
	}

	return plugins, nil
}

// exists checks if a file or directory exists
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DownloadFile downloads a file from the given URL and saves it to the given filepath
func DownloadFile(filepathDest string, url string) (int, error) {
	out, err := os.Create(filepathDest)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	if status != http.StatusOK {
		return status, fmt.Errorf("bad status: %s", resp.Status)
	}

	if _, err = io.Copy(out, resp.Body); err != nil {
		return status, err
	}
	return status, nil
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
	resp, err := http.Head(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest: %s", resp.Status)
	}
	return "latest", nil
}

// isSpecialLatestVersion checks if the version string means "latest"
func isSpecialLatestVersion(version string) bool {
	version = strings.ToLower(version)
	if version == "latest" {
		return true
	}
	matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, version)
	return matched
}

// tryReadGitHead tries to read a commit hash from a .git directory inside pluginDir
func tryReadGitHead(pluginDir string) string {
	gitHeadPath := filepath.Join(pluginDir, ".git", "HEAD")
	b, err := os.ReadFile(gitHeadPath)
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(b))
	if strings.HasPrefix(head, "ref: ") {
		ref := strings.TrimPrefix(head, "ref: ")
		refPath := filepath.Join(pluginDir, ".git", filepath.FromSlash(ref))
		if rb, err := os.ReadFile(refPath); err == nil {
			return strings.TrimSpace(string(rb))
		}
		packedPath := filepath.Join(pluginDir, ".git", "packed-refs")
		if pb, err := os.ReadFile(packedPath); err == nil {
			lines := strings.Split(string(pb), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) == 2 && parts[1] == ref {
					return parts[0]
				}
			}
		}
		return ""
	}
	if head != "" {
		return head
	}
	return ""
}

// computeDirSHA1 computes a SHA1 over the files contained in dir in a deterministic way
func computeDirSHA1(dir string) (string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if base == ".DS_Store" || base == "Thumbs.db" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha1.New()
	for _, f := range files {
		rel, _ := filepath.Rel(dir, f)
		if _, err := h.Write([]byte(rel)); err != nil {
			return "", err
		}
		if _, err := h.Write([]byte{0}); err != nil {
			return "", err
		}
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		if _, err := h.Write(data); err != nil {
			return "", err
		}
		if _, err := h.Write([]byte{0}); err != nil {
			return "", err
		}
	}
	sum := h.Sum(nil)
	return fmt.Sprintf("%x", sum), nil
}

// installPlugins downloads and installs plugins from the installed.json file
func installPlugins() error {
	installedPath := ".plugins/installed.json"
	pluginsJSON, err := os.ReadFile(installedPath)
	if err != nil {
		return fmt.Errorf("error reading installed plugins file: %v", err)
	}

	var plugins []Plugin
	if err := json.Unmarshal(pluginsJSON, &plugins); err != nil {
		return fmt.Errorf("error parsing plugins JSON: %v", err)
	}

	cacheDir := ".plugins/cache"
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating cache directory: %v", err)
	}

	for i := range plugins {
		plugin := &plugins[i]
		pluginVersion := plugin.Version

		if strings.ToLower(pluginVersion) == "v-latest" {
			pluginVersion = "latest"
		}
		// Don't check if latest exists, just try to download it
		plugin.Version = pluginVersion

		url := fmt.Sprintf("https://download.pocketstore.io/d/plugins/%s/%s/%s.zip", plugin.Vendor, plugin.Name, pluginVersion)
		zipPath := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-%s.zip", plugin.Vendor, plugin.Name, pluginVersion))
		destDir := filepath.Join(".plugins", "repos", plugin.Vendor, plugin.Name)

		_ = os.RemoveAll(destDir)

		_, err := DownloadFile(zipPath, url)
		if err != nil {
			_ = os.Remove(zipPath)
			return fmt.Errorf("failed to download %s: %v", url, err)
		}

		if err := Unzip(zipPath, destDir); err != nil {
			_ = os.Remove(zipPath)
			_ = os.RemoveAll(destDir)
			return fmt.Errorf("failed to unzip %s: %v", zipPath, err)
		}

		// Determine revision/commit hash
		pluginJSONPath := filepath.Join(destDir, "plugin.json")
		if exists(pluginJSONPath) {
			if pj, err := readPluginMeta(plugin.Vendor, plugin.Name); err == nil {
				if pj.Revision != "" {
					plugin.Revision = pj.Revision
				} else if pj.Version != "" {
					plugin.Revision = pj.Version
				}
				if pj.Version != "" {
					plugin.Version = pj.Version
				}
			}
		}

		if plugin.Revision == "" {
			if rev := tryReadGitHead(destDir); rev != "" {
				plugin.Revision = rev
			}
		}
		if plugin.Revision == "" {
			if rev, err := computeDirSHA1(destDir); err == nil {
				plugin.Revision = rev
			}
		}

		fmt.Printf("✓ %s/%s (version=%s)\n", plugin.Vendor, plugin.Name, plugin.Version)
	}

	// Write back resolved metadata INCLUDING revision field
	out, err := json.MarshalIndent(plugins, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling updated installed plugins: %v", err)
	}
	if err := os.WriteFile(installedPath, out, 0644); err != nil {
		return fmt.Errorf("error writing updated installed plugins to %s: %v", installedPath, err)
	}

	return nil
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

// readPrio reads the priority from a plugin.json file
func readPrio(vendor, name string) int {
	pj, err := readPluginMeta(vendor, name)
	if err != nil {
		return 0
	}
	return pj.Prio
}

// mergePluginFiles merges plugin files into the storefront directory
func mergePluginFiles() error {
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
			plugins = append(plugins, Plugin{
				Vendor: "",
				Name:   vendorEntry.Name(),
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
				plugins = append(plugins, Plugin{
					Vendor: vendorEntry.Name(),
					Name:   p.Name(),
				})
			}
		}
	}

	// Sort plugins by priority descending
	sort.Slice(plugins, func(i, j int) bool {
		prioi := readPrio(plugins[i].Vendor, plugins[i].Name)
		prioj := readPrio(plugins[j].Vendor, plugins[j].Name)
		return prioi > prioj
	})

	// Copy folders for each plugin
	for _, plugin := range plugins {
		basePath := filepath.Join(pluginRoot, plugin.Vendor, plugin.Name)
		if plugin.Vendor == "" {
			basePath = filepath.Join(pluginRoot, plugin.Name)
		}

		for _, d := range dirsToCopy {
			src := filepath.Join(basePath, d)

			// public goes to storefront/public, others to storefront/app/<dir>
			var dst string
			if d == "public" {
				dst = filepath.Join("public")
			} else {
				dst = filepath.Join("app", d)
			}

			if exists(src) {
				finalDst := filepath.Join("storefront", dst)
				if err := copyDir(src, finalDst); err != nil {
					fmt.Printf("  Error copying %s: %v\n", d, err)
				}
			}
		}
		prio := readPrio(plugin.Vendor, plugin.Name)
		fmt.Printf("✓ %s/%s (prio: %d)\n", plugin.Vendor, plugin.Name, prio)
	}
	return nil
}

func main() {
	fmt.Println("==> Resolving plugin requirements")

	// Load plugins from custom and storefront only (ignore baseline)
	customPlugins, err := loadPluginsFromFile("custom/plugins.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	storefrontPlugins, err := loadPluginsFromFile("storefront/plugins.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d plugins from custom/plugins.json\n", len(customPlugins))
	fmt.Printf("Loaded %d plugins from storefront/plugins.json\n", len(storefrontPlugins))

	// Resolve all requirements recursively (baseline is empty)
	resolved, err := resolveRequirements([]Plugin{}, customPlugins, storefrontPlugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nTotal plugins after resolving requirements: %d\n", len(resolved))

	// Save back to installed.json
	if err := savePlugins(resolved); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("==> Requirements resolved and saved to installed.json")

	// Install plugins (download and extract)
	fmt.Println("\n==> Installing plugins")
	if err := installPlugins(); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	// Merge plugin files into storefront
	fmt.Println("\n==> Merging plugin files")
	if err := mergePluginFiles(); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}
}
