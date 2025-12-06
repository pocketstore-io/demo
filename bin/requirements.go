package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Plugin struct {
	Version  string `json:"version"`
	Name     string `json:"name"`
	Vendor   string `json:"vendor"`
	Revision string `json:"revision,omitempty"`
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
func resolveRequirements(plugins []Plugin) ([]Plugin, error) {
	seen := make(map[string]bool)
	result := make([]Plugin, 0)
	queue := make([]Plugin, 0)
	dependencyTree := make(map[string][]string) // parent -> children
	isRoot := make(map[string]bool)

	// Initialize queue with input plugins
	for _, p := range plugins {
		key := p.Vendor + "/" + p.Name
		if !seen[key] {
			seen[key] = true
			queue = append(queue, p)
			isRoot[key] = true
		}
	}

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
			}
			queue = append(queue, newPlugin)
			fmt.Printf("  [%s] requires → %s\n", currentKey, key)
		}
	}

	// Print dependency tree
	if len(dependencyTree) > 0 {
		fmt.Println("\n==> Dependency Tree:")
		visited := make(map[string]bool)
		for _, root := range plugins {
			key := root.Vendor + "/" + root.Name
			if isRoot[key] {
				printNode(dependencyTree, key, "", visited, true, true)
			}
		}
	}

	return result, nil
}

// printNode prints a visual tree node with its children
func printNode(tree map[string][]string, key string, prefix string, visited map[string]bool, isLast bool, isRoot bool) {
	marker := "├──"
	if isLast {
		marker = "└──"
	}

	if isRoot {
		fmt.Printf("%s\n", key)
	} else {
		fmt.Printf("%s%s %s\n", prefix, marker, key)
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
	if !isRoot {
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
	}

	for i, child := range children {
		printNode(tree, child, newPrefix, visited, i == len(children)-1, false)
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

// loadInstalledPlugins reads the installed.json file
func loadInstalledPlugins() ([]Plugin, error) {
	data, err := os.ReadFile(".plugins/installed.json")
	if err != nil {
		return nil, fmt.Errorf("error reading .plugins/installed.json: %v", err)
	}

	var plugins []Plugin
	if err := json.Unmarshal(data, &plugins); err != nil {
		return nil, fmt.Errorf("error parsing installed.json: %v", err)
	}

	return plugins, nil
}

func main() {
	fmt.Println("==> Resolving plugin requirements")

	// Load plugins from installed.json
	plugins, err := loadInstalledPlugins()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d plugins from installed.json\n", len(plugins))

	// Resolve all requirements recursively
	resolved, err := resolveRequirements(plugins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total plugins after resolving requirements: %d\n", len(resolved))

	// Save back to installed.json
	if err := savePlugins(resolved); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("==> Requirements resolved and saved to installed.json")
}
