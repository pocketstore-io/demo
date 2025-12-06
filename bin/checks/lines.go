package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type FailItem struct {
	Path  string `json:"path"`
	Lines int    `json:"lines"`
}

var (
	baseDir    = flag.String("base", ".plugins/repos", "base directory containing repos (expects pattern base/*/*)")
	auditDir   = flag.String("audit", ".plugins/audit", "directory to write audit components.json files (will mirror org/repo structure)")
	threshold  = flag.Int("threshold", 50, "maximum allowed lines per .vue file (files with more lines are recorded)")
	components = flag.String("components", "components", "components directory name inside each repo")
)

func main() {
	flag.Parse()

	pattern := filepath.Join(*baseDir, "*", "*")
	repoDirs, err := filepath.Glob(pattern)
	if err != nil {
		// silent on errors per request
		os.Exit(2)
	}
	if len(repoDirs) == 0 {
		os.Exit(0)
	}

	var anyFail bool

	for _, repo := range repoDirs {
		info, err := os.Stat(repo)
		if err != nil || !info.IsDir() {
			continue
		}

		componentsDir := filepath.Join(repo, *components)

		// gather .vue files under componentsDir recursively
		var vueFiles []string
		_ = filepath.WalkDir(componentsDir, func(path string, d os.DirEntry, walkErr error) error {
			// ignore errors; be silent
			if walkErr != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(d.Name()) == ".vue" {
				vueFiles = append(vueFiles, path)
			}
			return nil
		})

		var longItems []FailItem
		for _, f := range vueFiles {
			n, err := countLines(f)
			if err != nil {
				// ignore read errors silently
				continue
			}
			if n > *threshold {
				relPath, err := filepath.Rel(repo, f)
				if err != nil {
					relPath = f
				}
				relRepo, err := filepath.Rel(*baseDir, repo)
				if err != nil || relRepo == "." {
					relRepo = filepath.Base(repo)
				}
				// Only output failing components to stdout in the requested format:
				// org/repo/path/to/Component.vue (N)
				fmt.Printf("%s/%s (%d)\n", relRepo, relPath, n)
				anyFail = true

				longItems = append(longItems, FailItem{
					Path:  relPath,
					Lines: n,
				})
			}
		}

		// deterministic ordering for per-repo JSON output by path
		sort.Slice(longItems, func(i, j int) bool {
			return longItems[i].Path < longItems[j].Path
		})

		// compute audit target path: <auditDir>/<org>/<repo>/components.json
		relRepo, err := filepath.Rel(*baseDir, repo)
		if err != nil || relRepo == "." {
			relRepo = filepath.Base(repo)
		}
		auditPath := filepath.Join(*auditDir, relRepo, "components.json")

		// write or remove audit file silently
		if len(longItems) == 0 {
			if exists(auditPath) {
				_ = os.Remove(auditPath)
			}
		} else {
			_ = writeJSONAtomic(auditPath, longItems)
		}
	}

	// prune any components.json files that are empty arrays or empty files
	_ = pruneEmptyAuditFiles(*auditDir)

	if anyFail {
		os.Exit(1)
	}
	os.Exit(0)
}

// pruneEmptyAuditFiles removes any components.json whose content,
// after trimming whitespace, is exactly "[]" or empty. Silent.
func pruneEmptyAuditFiles(auditDir string) error {
	if auditDir == "" {
		return nil
	}
	pattern := filepath.Join(auditDir, "*", "*", "components.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, p := range matches {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		trim := bytes.TrimSpace(data)
		if len(trim) == 0 || bytes.Equal(trim, []byte("[]")) {
			_ = os.Remove(p)
			continue
		}
		var decoded interface{}
		if err := json.Unmarshal(data, &decoded); err == nil {
			if arr, ok := decoded.([]interface{}); ok && len(arr) == 0 {
				_ = os.Remove(p)
			}
		}
	}
	return nil
}

// exists reports whether the named file exists (and is not a directory).
func exists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// countLines returns the number of lines in the given file.
func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return count, err
	}
	return count, nil
}

// writeJSONAtomic writes the given data (any JSON-marshalable value) as pretty JSON to path atomically.
func writeJSONAtomic(path string, data interface{}) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}