package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Badge URL template
const badgeURLTemplate = "https://img.shields.io/badge/%s-%s-brightgreen"

// Folder to save badges
const badgeFolder = ".github/badges"

// Struct to store submodule info
type Submodule struct {
	Name    string
	Path    string
	Version string
}

// Parse .gitmodules to extract submodule names and paths
func parseGitmodules(filePath string) ([]Submodule, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var submodules []Submodule
	var current Submodule
	scanner := bufio.NewScanner(file)

	reSubmodule := regexp.MustCompile(`^\[submodule "(.*)"\]`)
	rePath := regexp.MustCompile(`^\s*path\s*=\s*(.*)`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := reSubmodule.FindStringSubmatch(line); matches != nil {
			if current.Name != "" {
				submodules = append(submodules, current)
			}
			current = Submodule{Name: matches[1]}
		} else if matches := rePath.FindStringSubmatch(line); matches != nil {
			current.Path = matches[1]
		}
	}

	if current.Name != "" {
		submodules = append(submodules, current)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return submodules, nil
}

// Get the latest version tag of a submodule
func getLatestVersionTag(path string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no version tag found in %s", path)
	}
	return strings.TrimSpace(string(output)), nil
}

// Replace special characters in submodule names for safe badge names
func sanitizeName(name string) string {
	return strings.ReplaceAll(name, "/", "_")
}

// Generate a badge for a submodule
func generateBadge(submodule Submodule) error {
	// Ensure the badge folder exists
	if err := os.MkdirAll(badgeFolder, os.ModePerm); err != nil {
		return err
	}

	// Sanitize and format badge details
	badgeName := sanitizeName(submodule.Name)
	version := submodule.Version

	// URL-encode badge details
	badgeName = urlQueryEscape(badgeName)
	version = urlQueryEscape(version)

	// Format badge URL
	badgeURL := fmt.Sprintf(badgeURLTemplate, badgeName, version)

	// Badge output file
	outputFile := filepath.Join(badgeFolder, fmt.Sprintf("%s.svg", badgeName))
	cmd := exec.Command("wget", "-q", "-O", outputFile, badgeURL)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate badge for %s: %v", submodule.Name, err)
	}

	fmt.Printf("Generated badge for %s: %s\n", submodule.Name, outputFile)
	return nil
}

// URL encode a string for safe inclusion in URLs
func urlQueryEscape(input string) string {
	replacer := strings.NewReplacer(" ", "%20", "/", "%2F", "#", "%23", "&", "%26")
	return replacer.Replace(input)
}

func main() {
	// Path to .gitmodules
	gitmodulesPath := ".gitmodules"

	// Parse submodules
	submodules, err := parseGitmodules(gitmodulesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading .gitmodules: %v\n", err)
		os.Exit(1)
	}

	// Get version tags and generate badges
	for i, submodule := range submodules {
		version, err := getLatestVersionTag(submodule.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting version tag for submodule %s: %v\n", submodule.Name, err)
			continue
		}
		submodules[i].Version = version

		// Generate the badge
		if err := generateBadge(submodules[i]); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating badge for submodule %s: %v\n", submodule.Name, err)
		}
	}
}
