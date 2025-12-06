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

var (
	baseDir   = flag.String("base", ".plugins/repos", "base directory containing repos (expects pattern base/*/*)")
	auditDir  = flag.String("audit", ".plugins/audit", "directory to write audit pages.json files (will mirror org/repo structure)")
	threshold = flag.Int("threshold", 100, "maximum allowed lines per .vue page (files with more lines are recorded)")
	verbose   = flag.Bool("v", true, "verbose output")
	dryRun    = flag.Bool("dry-run", false, "don't write/remove pages.json files; just print what would be done")
	pagesDir  = flag.String("pages", "pages", "pages directory name inside each repo")
)

func main() {
	flag.Parse()

	pattern := filepath.Join(*baseDir, "*", "*")
	repoDirs, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid glob pattern %q: %v\n", pattern, err)
		os.Exit(2)
	}
	if len(repoDirs) == 0 {
		if *verbose {
			fmt.Fprintf(os.Stderr, "no repositories found under %s\n", *baseDir)
		}
		os.Exit(0)
	}

	var globalFails []string
	var totalRepos, totalFiles, totalLong int

	for _, repo := range repoDirs {
		info, err := os.Stat(repo)
		if err != nil || !info.IsDir() {
			continue
		}
		totalRepos++

		pagesPath := filepath.Join(repo, *pagesDir)

		// gather .vue files under pagesPath recursively
		var vueFiles []string
		_ = filepath.WalkDir(pagesPath, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				// ignore permission errors for individual entries
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

		if *verbose && len(vueFiles) == 0 {
			fmt.Printf("repo: %s — no %s/**/*.vue files found\n", repo, *pagesDir)
		}

		longFiles := make([]string, 0, 4)
		for _, f := range vueFiles {
			totalFiles++
			n, err := countLines(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to count lines for %s: %v\n", f, err)
				continue
			}
			relPath, err := filepath.Rel(repo, f)
			if err != nil {
				relPath = f
			}
			if n > *threshold {
				longFiles = append(longFiles, relPath)
				totalLong++

				// print failing page immediately
				fmt.Printf("FAIL: %s / %s (%d lines > %d)\n", repo, relPath, n, *threshold)

				relRepo, err := filepath.Rel(*baseDir, repo)
				if err != nil || relRepo == "." {
					relRepo = filepath.Base(repo)
				}
				globalFails = append(globalFails, filepath.Join(relRepo, relPath))
			} else if *verbose {
				fmt.Printf("OK:   %s / %s (%d lines)\n", repo, relPath, n)
			}
		}

		sort.Strings(longFiles)

		// compute audit target path: <auditDir>/<org>/<repo>/pages.json
		relRepo, err := filepath.Rel(*baseDir, repo)
		if err != nil || relRepo == "." {
			relRepo = filepath.Base(repo)
		}
		auditPath := filepath.Join(*auditDir, relRepo, "pages.json")

		if *dryRun {
			if len(longFiles) == 0 {
				if exists(auditPath) {
					fmt.Printf("DRY RUN: would remove empty audit file %s\n", auditPath)
				} else if *verbose {
					fmt.Printf("DRY RUN: no audit file to remove at %s\n", auditPath)
				}
			} else {
				fmt.Printf("DRY RUN: would write %d entries to %s\n", len(longFiles), auditPath)
			}
			continue
		}

		if len(longFiles) == 0 {
			// Remove existing audit file if present
			if exists(auditPath) {
				if *verbose {
					fmt.Printf("removing empty audit file %s\n", auditPath)
				}
				if err := os.Remove(auditPath); err != nil {
					fmt.Fprintf(os.Stderr, "error removing %s: %v\n", auditPath, err)
				}
			} else if *verbose {
				fmt.Printf("no audit file to remove at %s\n", auditPath)
			}
		} else {
			if err := writeJSONAtomic(auditPath, longFiles); err != nil {
				fmt.Fprintf(os.Stderr, "error writing %s: %v\n", auditPath, err)
				continue
			}
			if *verbose {
				fmt.Printf("wrote %d entries to %s\n", len(longFiles), auditPath)
			}
		}
	}

	// prune any pages.json files that are literally empty arrays "[]"
	if err := pruneEmptyAuditFiles(*auditDir, *dryRun, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "warning: pruning audit files: %v\n", err)
	}

	// consolidated output
	if len(globalFails) > 0 {
		fmt.Println("\nConsolidated failing pages:")
		sort.Strings(globalFails)
		for _, p := range globalFails {
			fmt.Println(p)
		}
	} else {
		fmt.Println("\nNo failing pages found.")
	}

	if *verbose {
		fmt.Printf("\nsummary: repos scanned=%d, vue files checked=%d, files >%d lines=%d\n",
			totalRepos, totalFiles, *threshold, totalLong)
	}
}

// pruneEmptyAuditFiles removes any audit pages.json whose content is literally an empty array "[]"
// (after trimming whitespace). In dry-run mode it prints what would be removed.
func pruneEmptyAuditFiles(auditDir string, dryRun, verbose bool) error {
	if auditDir == "" {
		return nil
	}
	pattern := filepath.Join(auditDir, "*", "*", "pages.json")
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
			if verbose {
				fmt.Fprintf(os.Stderr, "warning: unable to read %s: %v\n", p, err)
			}
			continue
		}
		trim := bytes.TrimSpace(data)
		if len(trim) == 0 {
			// treat empty file as removable
			if dryRun {
				fmt.Printf("DRY RUN: would remove empty audit file %s\n", p)
			} else {
				if verbose {
					fmt.Printf("removing empty audit file %s\n", p)
				}
				_ = os.Remove(p)
			}
			continue
		}
		if bytes.Equal(trim, []byte("[]")) {
			if dryRun {
				fmt.Printf("DRY RUN: would remove audit file containing []: %s\n", p)
			} else {
				if verbose {
					fmt.Printf("removing audit file containing []: %s\n", p)
				}
				if err := os.Remove(p); err != nil {
					fmt.Fprintf(os.Stderr, "error removing %s: %v\n", p, err)
				}
			}
			continue
		}
		// handle pretty-printed JSON or whitespace/newlines: decode and check for empty array
		var decoded interface{}
		if err := json.Unmarshal(data, &decoded); err == nil {
			if arr, ok := decoded.([]interface{}); ok && len(arr) == 0 {
				if dryRun {
					fmt.Printf("DRY RUN: would remove audit file with empty array JSON: %s\n", p)
				} else {
					if verbose {
						fmt.Printf("removing audit file with empty array JSON: %s\n", p)
					}
					if err := os.Remove(p); err != nil {
						fmt.Fprintf(os.Stderr, "error removing %s: %v\n", p, err)
					}
				}
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

// writeJSONAtomic writes the given data (slice of strings) as pretty JSON to path atomically.
func writeJSONAtomic(path string, data []string) error {
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