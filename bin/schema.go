package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func main() {
	// Pattern: .plugins/repos/*/*/schema.json
	pattern := ".plugins/repos/*/*/schema.json"

	matches, err := filepath.Glob(pattern)
	if err != nil {
		panic(err)
	}

	if len(matches) == 0 {
		fmt.Println("No schema.json files found.")
		return
	}

	merged := []map[string]interface{}{}

	for _, file := range matches {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", file, err)
			continue
		}

		var arr []map[string]interface{}
		if err := json.Unmarshal(data, &arr); err != nil {
			fmt.Printf("Error parsing JSON in %s: %v\n", file, err)
			continue
		}

		merged = append(merged, arr...)
		fmt.Printf("Merged %d entries from %s\n", len(arr), file)
	}

	// Ensure output directory exists
	outputDir := ".data"
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			panic(err)
		}
	}

	// Write the merged JSON array
	outputFile := filepath.Join(outputDir, "schema.json")
	outData, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		panic(err)
	}

	if err := ioutil.WriteFile(outputFile, outData, 0644); err != nil {
		panic(err)
	}

	fmt.Printf("Merged schema written to %s (%d total objects)\n", outputFile, len(merged))
}
