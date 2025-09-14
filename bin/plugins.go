package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "sort"
    "time"
)

func readPluginArray(filename string) ([]string, error) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    var plugins []string
    if err := json.Unmarshal(data, &plugins); err != nil {
        return nil, err
    }
    return plugins, nil
}

func fetchPluginJSON(slug string) (map[string]interface{}, error) {
    url := fmt.Sprintf("https://pocketstore.io/plugin/%s/plugin.json", slug)
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
    }
    var plugin map[string]interface{}
    decoder := json.NewDecoder(resp.Body)
    if err := decoder.Decode(&plugin); err != nil {
        return nil, err
    }
    return plugin, nil
}

func main() {
    plugins1, err := readPluginArray(".plugins/config.json")
    if err != nil {
        log.Fatalf("Failed to read .plugins/config.json: %v", err)
    }
    plugins2, err := readPluginArray("custom/plugins.json")
    if err != nil {
        log.Fatalf("Failed to read custom/plugins.json: %v", err)
    }

    // Merge arrays and deduplicate
    uniq := make(map[string]struct{})
    for _, p := range append(plugins1, plugins2...) {
        uniq[p] = struct{}{}
    }
    merged := make([]string, 0, len(uniq))
    for p := range uniq {
        merged = append(merged, p)
    }
    sort.Strings(merged)

    // Ensure .plugins/cache directory exists
    if _, err := os.Stat(".plugins/cache"); os.IsNotExist(err) {
        err = os.MkdirAll(".plugins/cache", 0755)
        if err != nil {
            log.Fatalf("Failed to create .plugins/cache directory: %v", err)
        }
    }

    // Write merged slugs to installed.json
    mergedData, err := json.MarshalIndent(merged, "", "  ")
    if err != nil {
        log.Fatalf("Failed to marshal merged plugins: %v", err)
    }
    if err := ioutil.WriteFile(".plugins/installed.json", mergedData, 0644); err != nil {
        log.Fatalf("Failed to write .plugins/installed.json: %v", err)
    }

    // Fetch plugin.json for each slug and create plugins array
    var plugins []map[string]interface{}
    for _, slug := range merged {
        plugin, err := fetchPluginJSON(slug)
        if err != nil {
            log.Printf("Failed to fetch plugin.json for %s: %v", slug, err)
            continue
        }
        plugins = append(plugins, plugin)

        // Write each plugin.json to .plugins/cache/{slug}.json
        pluginFile := filepath.Join(".plugins", "cache", slug+".json")
        pluginData, err := json.MarshalIndent(plugin, "", "  ")
        if err != nil {
            log.Printf("Failed to marshal plugin.json for %s: %v", slug, err)
            continue
        }
        if err := ioutil.WriteFile(pluginFile, pluginData, 0644); err != nil {
            log.Printf("Failed to write %s: %v", pluginFile, err)
            continue
        }
    }

    // download version list
    // sort by prio
    // install via command (download zip)
}