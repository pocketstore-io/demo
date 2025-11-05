package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Plugin struct {
	Vendor   string
	Name     string
	Prio     int
	BasePath string
}

type PluginJson struct {
	Prio int `json:"prio"`
}

var (
	pluginRoot     = ".plugins/repos"
	baselineRoot   = "baseline"
	storefrontRoot = "storefront"
)

func main() {
	// Get list of language files from baseline/translations
	baselineTranslationsDir := filepath.Join(baselineRoot, "translations")
	langFiles, err := os.ReadDir(baselineTranslationsDir)
	if err != nil {
		fmt.Printf("Error reading baseline translations: %v\n", err)
		return
	}

	// Extract language codes
	var langCodes []string
	for _, file := range langFiles {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			langCode := file.Name()[:len(file.Name())-5] // remove .json
			langCodes = append(langCodes, langCode)
		}
	}

	// Get plugins sorted by priority (descending)
	plugins := getPlugins()
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Prio > plugins[j].Prio
	})

	// Process each language
	for _, langCode := range langCodes {
		fmt.Printf("Processing language: %s\n", langCode)
		merged := make(map[string]interface{})

		// Start with baseline
		baselineFile := filepath.Join(baselineTranslationsDir, langCode+".json")
		if err := mergeTranslationFile(baselineFile, merged); err != nil {
			fmt.Printf("  Error loading baseline: %v\n", err)
			continue
		}

		// Override with plugin translations (highest priority first)
		for _, plugin := range plugins {
			pluginTransFile := filepath.Join(plugin.BasePath, "translations", langCode+".json")
			if exists(pluginTransFile) {
				id := plugin.Name
				if plugin.Vendor != "" {
					id = plugin.Vendor + "/" + plugin.Name
				}
				fmt.Printf("  Merging from plugin %s (prio: %d)\n", id, plugin.Prio)
				if err := mergeTranslationFile(pluginTransFile, merged); err != nil {
					fmt.Printf("    Error merging: %v\n", err)
				}
			}
		}

		// Write merged result to storefront/i18n/locales/{langcode}.json
		outputDir := filepath.Join(storefrontRoot, "i18n", "locales")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Printf("  Error creating output directory: %v\n", err)
			continue
		}

		outputFile := filepath.Join(outputDir, langCode+".json")
		if err := writeJSON(outputFile, merged); err != nil {
			fmt.Printf("  Error writing output: %v\n", err)
			continue
		}

		fmt.Printf("  Successfully generated %s\n", outputFile)
	}
}

func getPlugins() []Plugin {
	var plugins []Plugin

	vendorDirs, err := os.ReadDir(pluginRoot)
	if err != nil {
		fmt.Printf("Failed to read plugin root: %v\n", err)
		return plugins
	}

	for _, vendorEntry := range vendorDirs {
		if !vendorEntry.IsDir() {
			continue
		}
		vendorPath := filepath.Join(pluginRoot, vendorEntry.Name())

		// Check for legacy single-level layout
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

		// Two-level layout: vendor/plugin
		pluginDirs, err := os.ReadDir(vendorPath)
		if err != nil {
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

	return plugins
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

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

func mergeTranslationFile(path string, target map[string]interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var source map[string]interface{}
	if err := json.Unmarshal(data, &source); err != nil {
		return err
	}

	mergeMaps(source, target)
	return nil
}

func mergeMaps(source, target map[string]interface{}) {
	for key, value := range source {
		if sourceMap, ok := value.(map[string]interface{}); ok {
			if targetMap, ok := target[key].(map[string]interface{}); ok {
				// Both are maps, merge recursively
				mergeMaps(sourceMap, targetMap)
			} else {
				// Target doesn't have a map, overwrite with source
				target[key] = deepCopy(sourceMap)
			}
		} else {
			// Simple value, overwrite
			target[key] = value
		}
	}
}

func deepCopy(source map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range source {
		if mapValue, ok := value.(map[string]interface{}); ok {
			result[key] = deepCopy(mapValue)
		} else {
			result[key] = value
		}
	}
	return result
}

func writeJSON(path string, data map[string]interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
