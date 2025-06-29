package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Translation is the expected structure for translation records
type Translation struct {
	ID         string `json:"id,omitempty"`
	Key        string `json:"key"`
	Translated string `json:"translated"`
	Lang       string `json:"lang"`
	Type       string `json:"type,omitempty"`
	Collection string `json:"collection,omitempty"`
}

// PocketStoreConfig for loading domain from pocketstore.json
type PocketStoreConfig struct {
	Domain string `json:"domain"`
}

// robustLangFileLoader loads a translation file (either as array or object) by path and lang
func robustLangFileLoader(path string, lang string) ([]Translation, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		// If file does not exist, return empty slice, no error
		if os.IsNotExist(err) {
			return []Translation{}, nil
		}
		return nil, err
	}
	// Try as array
	var arr []Translation
	if err := json.Unmarshal(data, &arr); err == nil {
		// Populate Lang and default Type if missing in object
		for i := range arr {
			if arr[i].Lang == "" {
				arr[i].Lang = lang
			}
			if arr[i].Type == "" {
				arr[i].Type = "words"
			}
		}
		return arr, nil
	}

	// Try as map[string]string or map[string]interface{}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		translations := make([]Translation, 0, len(obj))
		for k, v := range obj {
			// Stringify v
			value := fmt.Sprintf("%v", v)
			translations = append(translations, Translation{
				Key:        k,
				Translated: value,
				Lang:       lang,
				Type:       "words",
			})
		}
		return translations, nil
	}

	return nil, fmt.Errorf("file %s is not a valid translation array or object", path)
}

// mergeTranslations merges base and overrides (custom) translations by Key+Lang
func mergeTranslations(base, overrides []Translation) []Translation {
	merged := make(map[string]Translation)
	for _, t := range base {
		merged[t.Lang+"|"+t.Key] = t
	}
	for _, t := range overrides {
		merged[t.Lang+"|"+t.Key] = t // override or insert
	}
	result := make([]Translation, 0, len(merged))
	for _, t := range merged {
		result = append(result, t)
	}
	return result
}

// loadPocketStoreDomain reads the domain from pocketstore.json and prepends https:// if not present
func loadPocketStoreDomain(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", path, err)
	}
	var cfg PocketStoreConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if cfg.Domain == "" {
		return "", fmt.Errorf("domain not found in %s", path)
	}
	// Ensure domain starts with https:// (or http://), prefer https://
	if !strings.HasPrefix(cfg.Domain, "https://") && !strings.HasPrefix(cfg.Domain, "http://") {
		cfg.Domain = "https://" + cfg.Domain
	} else if strings.HasPrefix(cfg.Domain, "http://") {
		cfg.Domain = "https://" + strings.TrimPrefix(cfg.Domain, "http://")
	}
	return cfg.Domain, nil
}

// postTranslation posts a single translation to PocketBase API
func postTranslation(domain string, translation Translation) error {
	url := fmt.Sprintf("%s/api/collections/translations/records", domain)
	payload := map[string]interface{}{
		"key":        translation.Key,
		"translated": translation.Translated,
		"lang":       translation.Lang,
		"type":       translation.Type,
	}
	if translation.Collection != "" {
		payload["collection"] = translation.Collection
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to post translation: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("error from server: %s\n%s", resp.Status, string(respBody))
	}
	return nil
}

// getLangFiles returns the language files for baseline and custom folders for the given lang
func getLangFiles(lang string) (string, string) {
	return filepath.Join(".translations", lang+".json"), filepath.Join("custom", "translations", lang+".json")
}

func main() {
	langs := []string{"de", "en", "fr"}

	// 1. Load domain from custom/pocketstore.json
	domain, err := loadPocketStoreDomain("custom/pocketstore.json")
	if err != nil {
		fmt.Println("Error loading pocketstore.json:", err)
		os.Exit(1)
	}

	for _, lang := range langs {
		baselinePath, customPath := getLangFiles(lang)

		// Load baseline (from .translations/lang.json)
		baseline, err := robustLangFileLoader(baselinePath, lang)
		if err != nil {
			fmt.Printf("Error loading baseline for %s: %v\n", lang, err)
			continue
		}

		// Load custom (from custom/translations/lang.json)
		custom, err := robustLangFileLoader(customPath, lang)
		if err != nil {
			fmt.Printf("Error loading custom override for %s: %v\n", lang, err)
			continue
		}

		// Merge: custom overrides baseline
		final := mergeTranslations(baseline, custom)

		if len(final) == 0 {
			fmt.Printf("No translations for %s\n", lang)
			continue
		}

		fmt.Printf("Posting %d translations for %s...\n", len(final), lang)
		success := 0
		for i, t := range final {
			err := postTranslation(domain, t)
			if err != nil {
				fmt.Printf("Error posting (%s) #%d (key=%s): %v\n", lang, i+1, t.Key, err)
			} else {
				success++
			}
			time.Sleep(30 * time.Millisecond)
		}
		fmt.Printf("Successfully posted %d/%d translations for %s to %s\n", success, len(final), lang, domain)
	}
}
