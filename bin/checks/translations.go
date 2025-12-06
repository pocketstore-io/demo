package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Occurrence struct {
	File string
	Line int
}

func main() {
	root := flag.String("root", ".plugins", "root directory to scan")
	localePath := flag.String("locale", "storefront/i18n/locales/de.json", "path to locale JSON file")
	extsFlag := flag.String("exts", ".vue", "comma-separated file extensions to scan")
	showOnlyMissing := flag.Bool("missing-only", true, "show only missing keys in the output")
	flag.Parse()

	exts := make(map[string]bool)
	for _, e := range strings.Split(*extsFlag, ",") {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		exts[e] = true
	}

	localeBytes, err := ioutil.ReadFile(*localePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read locale file %s: %v\n", *localePath, err)
		os.Exit(2)
	}

	var parsed interface{}
	if err := json.Unmarshal(localeBytes, &parsed); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse locale JSON %s: %v\n", *localePath, err)
		os.Exit(2)
	}

	// Build a map of locale key => value (terminal values only), so we can detect missing keys
	localeValues := make(map[string]interface{})
	flattenValues("", parsed, localeValues)

	// RE2 does not support backreferences like \1; use alternation instead.
	// This matches:
	// $t('key')  -> group 1
	// $t("key")  -> group 2
	// $t(unquoted.key) -> group 3
	re := regexp.MustCompile(`\$t\(\s*'([^']*)'\s*\)|\$t\(\s*"([^"]*)"\s*\)|\$t\(\s*([^)\s'"]+)\s*\)`)

	foundOccurrences := make(map[string][]Occurrence)        // keys present & have translation
	missingOccurrences := make(map[string][]Occurrence)      // keys referenced but missing in locale
	untranslatedOccurrences := make(map[string][]Occurrence) // keys present in locale but empty value (referenced)
	ignoredOccurrences := make(map[string][]Occurrence)      // empty keys or other

	err = filepath.WalkDir(*root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// If there's an error accessing a path, log and continue.
			fmt.Fprintf(os.Stderr, "warning: can't access %s: %v\n", path, walkErr)
			return nil
		}
		if d.IsDir() {
			// Skip node_modules and storefront entirely
			if d.Name() == "node_modules" || d.Name() == "storefront" {
				return filepath.SkipDir
			}
			return nil
		}
		// skip non-target extensions
		if !exts[filepath.Ext(path)] {
			return nil
		}
		contentBytes, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to read %s: %v\n", path, err)
			return nil
		}
		content := string(contentBytes)

		// Find all matches with index so we can compute line numbers
		matches := re.FindAllStringSubmatchIndex(content, -1)
		for _, idx := range matches {
			// idx layout: [fullStart, fullEnd, g1start, g1end, g2start, g2end, g3start, g3end]
			if len(idx) < 8 {
				continue
			}
			fullStart := idx[0]
			var key string
			// group 1: single-quoted, group 2: double-quoted, group 3: unquoted
			if idx[2] >= 0 && idx[3] >= 0 {
				key = content[idx[2]:idx[3]]
			} else if idx[4] >= 0 && idx[5] >= 0 {
				key = content[idx[4]:idx[5]]
			} else if idx[6] >= 0 && idx[7] >= 0 {
				key = content[idx[6]:idx[7]]
			} else {
				// No captured key (shouldn't happen), skip
				continue
			}
			// compute line number (1-based)
			line := 1 + strings.Count(content[:fullStart], "\n")
			occ := Occurrence{File: path, Line: line}
			if strings.TrimSpace(key) == "" {
				ignoredOccurrences[key] = append(ignoredOccurrences[key], occ)
				continue
			}

			if val, ok := localeValues[key]; ok {
				// present in locale, check if empty string
				if s, isStr := val.(string); isStr && strings.TrimSpace(s) == "" {
					untranslatedOccurrences[key] = append(untranslatedOccurrences[key], occ)
				} else {
					foundOccurrences[key] = append(foundOccurrences[key], occ)
				}
			} else {
				missingOccurrences[key] = append(missingOccurrences[key], occ)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk error: %v\n", err)
		os.Exit(2)
	}

	// Also collect all locale keys that are empty (regardless of whether referenced)
	localeEmptyKeys := make([]string, 0)
	for k, v := range localeValues {
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			localeEmptyKeys = append(localeEmptyKeys, k)
		}
	}
	sort.Strings(localeEmptyKeys)

	// Print results
	fmt.Println("Translation key scan results")
	fmt.Printf("Locale file: %s\n", *localePath)
	fmt.Printf("Root scanned: %s\n\n", *root)

	// Helper to print map in deterministic order
	printMap := func(m map[string][]Occurrence, header string) {
		if len(m) == 0 {
			fmt.Printf("%s: none\n\n", header)
			return
		}
		fmt.Printf("%s (%d keys):\n", header, len(m))
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s\n", k)
			for _, occ := range m[k] {
				fmt.Printf("    - %s:%d\n", occ.File, occ.Line)
			}
		}
		fmt.Println()
	}

	// Helper to print a simple list of strings
	printList := func(list []string, header string) {
		if len(list) == 0 {
			fmt.Printf("%s: none\n\n", header)
			return
		}
		fmt.Printf("%s (%d):\n", header, len(list))
		for _, k := range list {
			fmt.Printf("  %s\n", k)
		}
		fmt.Println()
	}

	if *showOnlyMissing {
		printMap(missingOccurrences, "Missing keys referenced in templates")
	} else {
		printMap(foundOccurrences, "Found keys (present & translated)")
		printMap(missingOccurrences, "Missing keys referenced in templates (not in locale)")
		printMap(untranslatedOccurrences, "Referenced keys present but untranslated (empty) in locale")
		if len(ignoredOccurrences) > 0 {
			printMap(ignoredOccurrences, "Ignored / empty keys in templates")
		}
		printList(localeEmptyKeys, "All keys in locale with empty translation (regardless of usage)")
	}

	// Summary
	totalFound := 0
	for _, occs := range foundOccurrences {
		totalFound += len(occs)
	}
	totalMissing := 0
	for _, occs := range missingOccurrences {
		totalMissing += len(occs)
	}
	totalUntranslated := 0
	for _, occs := range untranslatedOccurrences {
		totalUntranslated += len(occs)
	}
	fmt.Printf("Summary: occurrences found=%d, missing=%d, referenced-but-empty=%d\n", totalFound, totalMissing, totalUntranslated)
	if totalMissing > 0 || len(localeEmptyKeys) > 0 {
		fmt.Println("\nMissing / empty translation keys can be used to add translations to the locale JSON.")
	}
}

// flattenValues converts nested JSON into dot-notation keys and stores terminal values.
// e.g. {"a": {"b": "x"}} -> out["a.b"] = "x"
func flattenValues(prefix string, v interface{}, out map[string]interface{}) {
	switch vv := v.(type) {
	case map[string]interface{}:
		for k, val := range vv {
			next := k
			if prefix != "" {
				next = prefix + "." + k
			}
			flattenValues(next, val, out)
		}
	default:
		if prefix != "" {
			out[prefix] = vv
		}
	}
}