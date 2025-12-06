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
	File string `json:"file"`
	Line int    `json:"line"`
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
	// $t('key') -> group 1
	// $t("key") -> group 2
	// $t(unquoted.key) -> group 3
	re := regexp.MustCompile(`\$t\(\s*'([^']*)'\s*\)|\$t\(\s*"([^"]*)"\s*\)|\$t\(\s*([^)\s'"]+)\s*\)`)

	// Collect occurrences grouped per module (plugin vendor/name)
	missingPerModule := make(map[string]map[string][]Occurrence)      // module -> key -> occurrences
	untranslatedPerModule := make(map[string]map[string][]Occurrence) // module -> key -> occurrences (present but empty)
	foundPerModule := make(map[string]map[string][]Occurrence)        // module -> key -> occurrences (present & translated)
	ignoredPerModule := make(map[string]map[string][]Occurrence)      // module -> key -> occurrences (empty keys or other)

	// Walk directory tree
	err = filepath.WalkDir(*root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// If there's an error accessing a path, log and continue.
			fmt.Fprintf(os.Stderr, "warning: can't access %s: %v\n", path, walkErr)
			return nil
		}
		if d.IsDir() {
			// Skip node_modules and storefront entirely if present
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

			// Determine module as vendor/name (first two elements under root)
			mod := moduleForVendorName(*root, path)
			if mod == "" {
				mod = "unknown"
			}

			// helpers to ensure maps exist
			ensure := func(m map[string]map[string][]Occurrence, module, k string) {
				if _, ok := m[module]; !ok {
					m[module] = make(map[string][]Occurrence)
				}
				if _, ok := m[module][k]; !ok {
					m[module][k] = make([]Occurrence, 0)
				}
			}

			if strings.TrimSpace(key) == "" {
				ensure(ignoredPerModule, mod, key)
				ignoredPerModule[mod][key] = append(ignoredPerModule[mod][key], occ)
				continue
			}
			if val, ok := localeValues[key]; ok {
				// present in locale, check if empty string
				if s, isStr := val.(string); isStr && strings.TrimSpace(s) == "" {
					ensure(untranslatedPerModule, mod, key)
					untranslatedPerModule[mod][key] = append(untranslatedPerModule[mod][key], occ)
				} else {
					ensure(foundPerModule, mod, key)
					foundPerModule[mod][key] = append(foundPerModule[mod][key], occ)
				}
			} else {
				ensure(missingPerModule, mod, key)
				missingPerModule[mod][key] = append(missingPerModule[mod][key], occ)
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

	// Print results (summary view)
	fmt.Println("Translation key scan results")
	fmt.Printf("Locale file: %s\n", *localePath)
	fmt.Printf("Root scanned: %s\n\n", *root)

	// Helper to print map in deterministic order
	printModuleMap := func(perModule map[string]map[string][]Occurrence, header string) {
		if len(perModule) == 0 {
			fmt.Printf("%s: none\n\n", header)
			return
		}
		fmt.Printf("%s (modules=%d):\n", header, len(perModule))
		modules := make([]string, 0, len(perModule))
		for m := range perModule {
			modules = append(modules, m)
		}
		sort.Strings(modules)
		for _, m := range modules {
			fmt.Printf(" Module: %s\n", m)
			keys := make([]string, 0, len(perModule[m]))
			for k := range perModule[m] {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("  %s\n", k)
				for _, occ := range perModule[m][k] {
					fmt.Printf("   - %s:%d\n", occ.File, occ.Line)
				}
			}
		}
		fmt.Println()
	}

	if *showOnlyMissing {
		printModuleMap(missingPerModule, "Missing keys referenced in templates (by module vendor/name)")
	} else {
		printModuleMap(foundPerModule, "Found keys (present & translated) (by module vendor/name)")
		printModuleMap(missingPerModule, "Missing keys referenced in templates (not in locale) (by module vendor/name)")
		printModuleMap(untranslatedPerModule, "Referenced keys present but untranslated (empty) in locale (by module vendor/name)")
		if len(ignoredPerModule) > 0 {
			printModuleMap(ignoredPerModule, "Ignored / empty keys in templates (by module vendor/name)")
		}
		printList(localeEmptyKeys, "All keys in locale with empty translation (regardless of usage)")
	}

	// Summary totals
	totalFound := 0
	for _, mod := range foundPerModule {
		for _, occs := range mod {
			totalFound += len(occs)
		}
	}
	totalMissing := 0
	for _, mod := range missingPerModule {
		for _, occs := range mod {
			totalMissing += len(occs)
		}
	}
	totalUntranslated := 0
	for _, mod := range untranslatedPerModule {
		for _, occs := range mod {
			totalUntranslated += len(occs)
		}
	}
	fmt.Printf("Summary: occurrences found=%d, missing=%d, referenced-but-empty=%d\n", totalFound, totalMissing, totalUntranslated)

	// Aggregate and write single audit file under <root>/audit/translations.json
	writeAggregateAuditFile(*root, missingPerModule, untranslatedPerModule, ignoredPerModule)

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

func printList(list []string, header string) {
	if len(list) == 0 {
		fmt.Printf("%s: none\n\n", header)
		return
	}
	fmt.Printf("%s (%d):\n", header, len(list))
	for _, k := range list {
		fmt.Printf(" %s\n", k)
	}
	fmt.Println()
}

// moduleForVendorName returns the plugin identifier in vendor/name form for a file path under root.
// It expects the plugin layout to be <root>/<vendor>/<name>/... and returns "vendor/name".
// If the file isn't under root or vendor/name can't be determined, it returns an empty string.
func moduleForVendorName(root, path string) string {
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return ""
	}
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return ""
	}

	// If path is inside root, get the first two path elements of the relative path
	if rel, err := filepath.Rel(absRoot, absPath); err == nil && !strings.HasPrefix(rel, "..") {
		rel = strings.TrimPrefix(rel, string(os.PathSeparator))
		parts := strings.Split(rel, string(os.PathSeparator))
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return filepath.ToSlash(filepath.Join(parts[0], parts[1]))
		}
	}

	// Fallback: try to find vendor/name by ascending until parent == root and then taking last two segments
	dir := filepath.Dir(absPath)
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		if parent == absRoot {
			// dir is <root>/<vendor>/<name> or <root>/<vendor>, so take the last two elements from dir
			parts := strings.Split(strings.TrimPrefix(dir, absRoot+string(os.PathSeparator)), string(os.PathSeparator))
			// parts may be like ["vendor","name",...], ensure at least two
			if len(parts) >= 2 {
				v := parts[0]
				n := parts[1]
				if v != "" && n != "" {
					return filepath.ToSlash(filepath.Join(v, n))
				}
			}
			break
		}
		dir = parent
	}

	// Give up - unknown plugin vendor/name
	return ""
}

// writeAggregateAuditFile writes a single translations.json file under <root>/audit/translations.json
// The structure will be:
// {
//   "missing": { "vendor/name": { "key": [{file,line}, ...], ... }, ... },
//   "untranslated": { "vendor/name": { "key": [...], ... }, ... },
//   "ignored": { "vendor/name": { "": [...], ... }, ... } // optional
// }
func writeAggregateAuditFile(root string, missing, untranslated, ignored map[string]map[string][]Occurrence) {
	audit := make(map[string]interface{})

	// Ensure we include modules even if empty maps so consumers can rely on keys existing if desired.
	audit["missing"] = missing
	audit["untranslated"] = untranslated
	if len(ignored) > 0 {
		audit["ignored"] = ignored
	}

	// Write to <root>/audit/translations.json
	auditDir := filepath.Join(root, "audit")
	if err := os.MkdirAll(auditDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to create audit dir %s: %v\n", auditDir, err)
		return
	}
	outPath := filepath.Join(auditDir, "translations.json")
	b, err := json.MarshalIndent(audit, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to marshal aggregate audit json: %v\n", err)
		return
	}
	if err := ioutil.WriteFile(outPath, b, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write aggregate audit file %s: %v\n", outPath, err)
		return
	}
	fmt.Printf("Wrote aggregate audit file: %s\n", outPath)
}