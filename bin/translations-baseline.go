package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Translation represents a single translation entry in the collection.
type Translation struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	Translated string `json:"translated"`
	Lang       string `json:"lang"`
	Type       string `json:"type"`
	Collection string `json:"collection"`
}

// PocketBaseResponse represents the response structure from PocketBase.
type PocketBaseResponse struct {
	Page       int           `json:"page"`
	PerPage    int           `json:"perPage"`
	TotalItems int           `json:"totalItems"`
	Items      []Translation `json:"items"`
}

// LoadTranslations fetches translations from a PocketBase collection.
func LoadTranslations(ctx context.Context, baseURL, collectionName string) ([]Translation, error) {
	url := fmt.Sprintf("%s/api/collections/%s/records", baseURL, collectionName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// Add PocketBase API token header if needed:
	// req.Header.Set("Authorization", "Bearer YOUR_API_TOKEN")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch translations: %s", resp.Status)
	}

	var result PocketBaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

func groupByLang(translations []Translation) map[string]map[string]string {
	byLang := make(map[string]map[string]string)
	for _, t := range translations {
		if _, ok := byLang[t.Lang]; !ok {
			byLang[t.Lang] = make(map[string]string)
		}
		byLang[t.Lang][t.Key] = t.Translated
	}
	return byLang
}

func writeTranslationsFiles(translationsByLang map[string]map[string]string, folder string) error {
	if err := os.MkdirAll(folder, 0755); err != nil {
		return err
	}
	for lang, entries := range translationsByLang {
		filePath := filepath.Join(folder, lang+".json")
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("could not marshal translations for %s: %w", lang, err)
		}
		if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
			return fmt.Errorf("could not write file %s: %w", filePath, err)
		}
	}
	return nil
}

func main() {
	baseURL := os.Getenv("POCKETBASE_URL")
	if baseURL == "" {
		baseURL = "http://admin.pocketstore.io"
	}

	collectionName := "translations" // Change if your collection has a different name

	ctx := context.Background()
	translations, err := LoadTranslations(ctx, baseURL, collectionName)
	if err != nil {
		log.Fatalf("Error loading translations: %v", err)
	}

	fmt.Printf("Loaded %d translations\n", len(translations))

	byLang := groupByLang(translations)
	if err := writeTranslationsFiles(byLang, ".translations"); err != nil {
		log.Fatalf("Error writing translation files: %v", err)
	}
	fmt.Printf("Saved translations for languages: ")
	for lang := range byLang {
		fmt.Printf("%s ", lang)
	}
	fmt.Println()
}
