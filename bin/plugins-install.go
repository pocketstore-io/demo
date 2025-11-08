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
	"strings"
)

type Plugin struct {
	Name    string `json:"name"`
	Vendor  string `json:"vendor"`
	Version string `json:"version"`
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

func main() {
	installedPath := ".plugins/installed.json"
	fmt.Printf("[debug] reading installed plugins file: %s\n", installedPath)
	pluginsJSON, err := os.ReadFile(installedPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] Error reading installed plugins file: %v\n", err)
		os.Exit(1)
	}

	var plugins []Plugin
	if err := json.Unmarshal(pluginsJSON, &plugins); err != nil {
		fmt.Fprintf(os.Stderr, "[error] Error parsing plugins JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[debug] parsed %d plugins from %s\n", len(plugins), installedPath)
	for i, p := range plugins {
		fmt.Printf("[debug] plugin[%d] = %+v\n", i, p)
	}

	cacheDir := ".plugins/cache"
	fmt.Printf("[debug] ensuring cache dir exists: %s\n", cacheDir)
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		fmt.Fprintf(os.Stderr, "[error] Error creating cache directory: %v\n", err)
		os.Exit(1)
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
				fmt.Fprintf(os.Stderr, "[error] Failed to fetch latest version for %s/%s: %v\n", plugin.Vendor, plugin.Name, err)
				os.Exit(10)
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
			fmt.Fprintf(os.Stderr, "[error] Failed to download %s: %v\n", url, err)
			_ = os.Remove(zipPath)
			os.Exit(11)
		}

		fmt.Printf("[info] Unzipping to %s...\n", destDir)
		if err := Unzip(zipPath, destDir); err != nil {
			fmt.Fprintf(os.Stderr, "[error] Failed to unzip %s: %v\n", zipPath, err)
			_ = os.Remove(zipPath)
			_ = os.RemoveAll(destDir)
			os.Exit(12)
		}

		fmt.Printf("[info] Installed %s/%s %s into %s\n", plugin.Vendor, plugin.Name, pluginVersion, destDir)
	}
}