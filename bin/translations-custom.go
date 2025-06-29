package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

// Translation struct per your schema
type Translation struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	Translated string `json:"translated"`
	Lang       string `json:"lang"`
	Type       string `json:"type"`
	Collection string `json:"collection"`
}

// Config struct for loading config.domain
type Config struct {
	Domain string `json:"domain"`
}

// fetchTranslations fetches all translations from the PocketBase API
func fetchTranslations(apiURL string) ([]Translation, error) {
	// Compose the API endpoint (List all records)
	endpoint := fmt.Sprintf("https://%s/api/collections/translations/records?perPage=1000", apiURL)
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response: %s", resp.Status)
	}

	var apiResp struct {
		Items []Translation `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return apiResp.Items, nil
}

// groupByLang groups translations by their Lang field.
func groupByLang(translations []Translation) map[string][]Translation {
	result := make(map[string][]Translation)
	for _, t := range translations {
		result[t.Lang] = append(result[t.Lang], t)
	}
	return result
}

// saveLangTranslations writes translations for a given lang to custom/translations/{lang}.json.
func saveLangTranslations(lang string, translations []Translation) error {
	dir := "custom/translations"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	outPath := filepath.Join(dir, lang+".json")
	jsonData, err := json.MarshalIndent(translations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode json for lang %s: %w", lang, err)
	}
	if err := ioutil.WriteFile(outPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", outPath, err)
	}
	return nil
}

// loadConfig reads the config file and returns Config
func loadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &cfg, nil
}

func main() {
	cfg, err := loadConfig("custom/pocketstore.json")
	if err != nil {
		fmt.Println("Error loading config:", err)
		os.Exit(1)
	}

	translations, err := fetchTranslations(cfg.Domain)
	if err != nil {
		fmt.Println("Error fetching translations:", err)
		os.Exit(1)
	}
	grouped := groupByLang(translations)
	for lang, langTranslations := range grouped {
		if err := saveLangTranslations(lang, langTranslations); err != nil {
			fmt.Printf("Failed to save translation for lang %s: %v\n", lang, err)
		} else {
			fmt.Printf("Saved translation for lang %s (%d entries)\n", lang, len(langTranslations))
		}
	}
}
