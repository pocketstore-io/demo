package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"regexp"
	"os"
	"path/filepath"
)

type Plugin struct {
	Name    string `json:"name"`
	Vendor  string `json:"vendor"`
	Version string `json:"version"`
}

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
func FetchLatestVersion(vendor, name string) (string, error) {
	url := fmt.Sprintf("https://download.pocketstore.io/d/%s/%s/latest.zip", vendor, name)
	resp, err := http.Head(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

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
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating cache directory: %v\n", err)
		os.Exit(1)
	}

	for _, plugin := range plugins {
		pluginVersion := plugin.Version
		// Remove v- prefix if it exists and version is "v-latest" or "v-LATEST" etc
		if strings.ToLower(pluginVersion) == "v-latest" {
			pluginVersion = "latest"
		}
		if isSpecialLatestVersion(pluginVersion) {
			ver, err := FetchLatestVersion(plugin.Vendor, plugin.Name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to fetch latest version for %s/%s: %v\n", plugin.Vendor, plugin.Name, err)
				os.Exit(10)
			}
			pluginVersion = ver
			fmt.Printf("Resolved latest version for %s/%s: %s\n", plugin.Vendor, plugin.Name, pluginVersion)
		}

		url := fmt.Sprintf("https://download.pocketstore.io/d/%s/%s/%s.zip", plugin.Vendor, plugin.Name, pluginVersion)
		zipPath := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-%s.zip", plugin.Vendor, plugin.Name, pluginVersion))
		destDir := filepath.Join(".plugins", "repos")

		err := os.MkdirAll(cacheDir, 0755)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Downloading %s...\n", url)
		if err := DownloadFile(zipPath, url); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to download %s: %v\n", url, err)
			_ = os.Remove(zipPath)
			os.Exit(11)
		}

		fmt.Printf("Unzipping to %s...\n", destDir)
		if err := Unzip(zipPath, destDir); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to unzip %s: %v\n", zipPath, err)
			_ = os.Remove(zipPath)
			_ = os.Remove(destDir)
			os.Exit(12)
		}

		fmt.Printf("Installed %s/%s %s\n", plugin.Vendor, plugin.Name, pluginVersion)
	}
}