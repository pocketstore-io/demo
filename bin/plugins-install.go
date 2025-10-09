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
	Type    string `json:"type"` // "plugin" or "theme"
}

const BaseURL = "https://download.pocketstore.io"

// DownloadFile downloads a file from the given URL and saves it to the given filepath
func DownloadFile(filepath string, url string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

// Unzip extracts a zip archive to a specified destination
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
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
func FetchLatestVersion(prefix, vendor, name string) string {
	// Construct URL to check if latest exists
	url := fmt.Sprintf("%s/d/%s/%s/%s/latest.zip", BaseURL, prefix, vendor, name)
	fmt.Println("Fetching latest version from:", url)

	resp, err := http.Head(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("Warning: failed to fetch latest: %v, defaulting to 'latest'\n", err)
		return "latest"
	}
	defer resp.Body.Close()
	return "latest"
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

func main() {
	installedPath := ".plugins/installed.json"
	pluginsJSON, err := os.ReadFile(installedPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading installed plugins file: %v\n", err)
		os.Exit(1)
	}

	var plugins []Plugin
	if err := json.Unmarshal(pluginsJSON, &plugins); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing plugins JSON: %v\n", err)
		os.Exit(1)
	}

	cacheDir := ".plugins/cache"
	destBase := ".plugins/repos"

	os.MkdirAll(cacheDir, os.ModePerm)
	os.MkdirAll(destBase, os.ModePerm)

	for _, plugin := range plugins {
		version := plugin.Version
        prefix := "plugins"
        if plugin.Type == "theme" || strings.HasPrefix(plugin.Name, "theme-") {
            prefix = "themes"
        }

        // Resolve "latest" to main branch
        if strings.ToLower(version) == "v-latest" || isSpecialLatestVersion(version) {
            version = FetchLatestVersion(prefix, plugin.Vendor, plugin.Name)
            fmt.Printf("Resolved latest version for %s/%s: %s\n", plugin.Vendor, plugin.Name, version)
        }

        // Use main branch if version is "latest"
        zipVersion := version

        zipURL := fmt.Sprintf("%s/d/%s/%s/%s/%s.zip", BaseURL, prefix, plugin.Vendor, plugin.Name, zipVersion)
        zipPath := filepath.Join(cacheDir, fmt.Sprintf("%s-%s.zip", plugin.Name, zipVersion))
        moduleDest := filepath.Join(destBase, fmt.Sprintf("%s", plugin.Name))

        fmt.Printf("Downloading %s...\n", zipURL)
        if err := DownloadFile(zipPath, zipURL); err != nil {
            fmt.Fprintf(os.Stderr, "Failed to download %s: %v\n", zipURL, err)
            os.Remove(zipPath)
            os.Exit(11)
        }

		fmt.Printf("Unzipping to %s...\n", moduleDest)
		if err := Unzip(zipPath, moduleDest); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to unzip %s: %v\n", zipPath, err)
			os.Remove(zipPath)
			os.RemoveAll(moduleDest)
			os.Exit(12)
		}

		fmt.Printf("Installed %s/%s %s as plugin-%s\n", plugin.Vendor, plugin.Name, version, plugin.Name)
	}
}
