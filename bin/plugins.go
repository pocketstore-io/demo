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
	Prio     int    `json:"prio,omitempty"`
	Revision string `json:"revision,omitempty"` // include revision in installed.json
	BasePath string `json:"-"`
}

type PluginJson struct {
	Prio     int    `json:"prio"`
	Revision string `json:"revision,omitempty"`
	Version  string `json:"version,omitempty"`
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

// readPluginMeta reads plugin.json and returns PluginJson metadata
func readPluginMeta(jsonPath string) (PluginJson, error) {
	file, err := os.Open(jsonPath)
	if err != nil {
		return PluginJson{}, err
	}
	defer file.Close()
	var pj PluginJson
	if err := json.NewDecoder(file).Decode(&pj); err != nil {
		return PluginJson{}, err
	}
	return pj, nil
}

// readPrio reads the priority from a plugin.json file (keeps legacy name)
func readPrio(jsonPath string) int {
	pj, err := readPluginMeta(jsonPath)
	if err != nil {
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
// returns the HTTP status code and an error (if any)
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
// returns "" if not found or any error occurs.
func tryReadGitHead(pluginDir string) string {
	gitHeadPath := filepath.Join(pluginDir, ".git", "HEAD")
	b, err := os.ReadFile(gitHeadPath)
	if err != nil {
		// no .git/HEAD present
		return ""
	}
	head := strings.TrimSpace(string(b))
	// If HEAD is a ref, try to read the ref file
	if strings.HasPrefix(head, "ref: ") {
		ref := strings.TrimPrefix(head, "ref: ")
		refPath := filepath.Join(pluginDir, ".git", filepath.FromSlash(ref))
		if rb, err := os.ReadFile(refPath); err == nil {
			return strings.TrimSpace(string(rb))
		}
		// try packed-refs fallback
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
	// HEAD contains a raw commit SHA
	if head != "" {
		return head
	}
	return ""
}

// computeDirSHA1 computes a SHA1 over the files contained in dir in a deterministic way
// Returns hex-encoded sha1. This is used as a fallback "commit-like" identifier when no
// revision is provided by plugin.json and no .git data is present.
func computeDirSHA1(dir string) (string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// skip common volatile files
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
		// include filename to avoid collisions
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

// Step 1: mergePlugins merges baseline and custom plugins into a unique list
func mergePlugins() error {
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

	return nil
}

// Step 2: installPlugins downloads and installs plugins from the installed.json file
// Clean, minimal output: per plugin two lines:
// vendor/name version=<resolved-version>
// status: <http-status-code>
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
		if isSpecialLatestVersion(pluginVersion) {
			ver, err := FetchLatestVersion(plugin.Vendor, plugin.Name)
			if err != nil {
				return fmt.Errorf("failed to fetch latest version for %s/%s: %v", plugin.Vendor, plugin.Name, err)
			}
			pluginVersion = ver
		}
		plugin.Version = pluginVersion

		url := fmt.Sprintf("https://download.pocketstore.io/d/plugins/%s/%s/%s.zip", plugin.Vendor, plugin.Name, pluginVersion)
		zipPath := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-%s.zip", plugin.Vendor, plugin.Name, pluginVersion))
		destDir := filepath.Join(".plugins", "repos", plugin.Vendor, plugin.Name)

		_ = os.RemoveAll(destDir)

		status, err := DownloadFile(zipPath, url)
		if err != nil {
			_ = os.Remove(zipPath)
			return fmt.Errorf("failed to download %s: %v", url, err)
		}

		if err := Unzip(zipPath, destDir); err != nil {
			_ = os.Remove(zipPath)
			_ = os.RemoveAll(destDir)
			return fmt.Errorf("failed to unzip %s: %v", zipPath, err)
		}

		// Attempt to determine revision/commit hash:
		// Priority:
		// 1) plugin.json "revision" (already handled below)
		// 2) .git/HEAD ref inside the extracted destDir (if present)
		// 3) deterministic SHA1 computed from files as a fallback
		pluginJSONPath := filepath.Join(destDir, "plugin.json")
		if exists(pluginJSONPath) {
			if pj, err := readPluginMeta(pluginJSONPath); err == nil {
				if pj.Revision != "" {
					plugin.Revision = pj.Revision
				} else if pj.Version != "" {
					// keep previous behavior: use version if provided as fallback
					plugin.Revision = pj.Version
				}
				if pj.Version != "" {
					plugin.Version = pj.Version
				}
			}
		}

		if plugin.Revision == "" {
			// try to read .git metadata if the zip included it
			if rev := tryReadGitHead(destDir); rev != "" {
				plugin.Revision = rev
			}
		}
		if plugin.Revision == "" {
			// fallback to deterministic dir hash
			if rev, err := computeDirSHA1(destDir); err == nil {
				plugin.Revision = rev
			}
		}

		// Minimal two-line output (keeps same format: version printed, status printed)
		fmt.Printf("%s/%s version=%s\n", plugin.Vendor, plugin.Name, plugin.Version)
		fmt.Printf("status: %d\n", status)
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

	// Step 2: Install plugins (clean output)
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