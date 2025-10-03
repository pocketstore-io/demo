package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
		url := fmt.Sprintf("https://download.pocketstore.io/d/%s/%s/v%s.zip", plugin.Vendor, plugin.Name, plugin.Version)
		zipPath := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-%s.zip", plugin.Vendor, plugin.Name, plugin.Version))
		destDir := filepath.Join(".plugins", "repos")

		err := os.MkdirAll(cacheDir, 0755)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Downloading %s...\n", url)
		if err := DownloadFile(zipPath, url); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to download %s: %v\n", url, err)
			_ = os.Remove(zipPath) // Remove the zip file on unzip failure
			os.Exit(11)
		}

		fmt.Printf("Unzipping to %s...\n", destDir)
		if err := Unzip(zipPath, destDir); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to unzip %s: %v\n", zipPath, err)
			_ = os.Remove(zipPath) // Remove the zip file on unzip failure
			_ = os.Remove(destDir) // Remove the zip file on unzip failure
			os.Exit(12)
		}

		fmt.Printf("Installed %s/%s %s\n", plugin.Vendor, plugin.Name, plugin.Version)
	}
}
