package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Plugin struct {
	Version string `json:"version"`
	Name    string `json:"name"`
	Vendor  string `json:"vendor"`
}

func readPluginsFromFile(filePath string) ([]Plugin, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var plugins []Plugin
	if err := json.Unmarshal(data, &plugins); err != nil {
		return nil, err
	}
	return plugins, nil
}

func main() {
	baselinePlugins, err := readPluginsFromFile("baseline/plugins.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading baseline/plugins.json: %v\n", err)
		os.Exit(1)
	}

	customPlugins, err := readPluginsFromFile("custom/plugins.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading custom/plugins.json: %v\n", err)
		os.Exit(1)
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

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling merged plugins: %v\n", err)
		os.Exit(1)
	}

	pluginsDir := ".plugins"
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating .plugins directory: %v\n", err)
		os.Exit(1)
	}

	outputFile := filepath.Join(pluginsDir, "installed.json")
	if err := ioutil.WriteFile(outputFile, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to %s: %v\n", outputFile, err)
		os.Exit(1)
	}

	fmt.Printf("Unique merged plugin list written to %s\n", outputFile)
}