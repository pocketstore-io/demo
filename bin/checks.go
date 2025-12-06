package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

var (
	timeoutFlag = flag.Duration("timeout", 60*time.Second, "per-script timeout (e.g. 30s, 2m)")
	stopOnError = flag.Bool("stop-on-error", false, "stop on first non-zero exit")
	verbose     = flag.Bool("v", true, "verbose output")
	auditDir    = flag.String("audit", ".plugins/audit", "audit directory to prune empty folders from")
	dryRun      = flag.Bool("dry-run", false, "don't actually remove folders; just print what would be done")
)

func main() {
	flag.Parse()

	// The two checks to run (relative to repo root)
	scripts := []string{
		"bin/checks/lines.go",
		"bin/checks/pages.go",
		"bin/checks/translations.go",
	}

	// Normalize to absolute paths where possible and verify existence
	absScripts := make([]string, 0, len(scripts))
	for _, p := range scripts {
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if _, err := os.Stat(abs); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s not found (%v); skipping\n", abs, err)
			continue
		}
		absScripts = append(absScripts, abs)
	}

	if len(absScripts) == 0 {
		fmt.Fprintln(os.Stderr, "no scripts found to run")
		os.Exit(1)
	}

	var failures int
	for _, s := range absScripts {
		if *verbose {
			fmt.Printf("\n=== Running: go run %s ===\n", s)
		}
		if err := runGoRun(s, *timeoutFlag); err != nil {
			fmt.Fprintf(os.Stderr, "script failed: %s -> %v\n", s, err)
			failures++
			if *stopOnError {
				os.Exit(2)
			}
		} else {
			if *verbose {
				fmt.Printf("script succeeded: %s\n", s)
			}
		}
	}

	// After scripts have run, prune empty audit folders
	if *verbose {
		fmt.Printf("\npruning empty audit folders under: %s\n", *auditDir)
	}
	if err := removeEmptyDirs(*auditDir, *dryRun, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "error pruning audit folders: %v\n", err)
		// don't change exit code for prune errors; continue to final handling
	}

	if failures > 0 {
		fmt.Fprintf(os.Stderr, "finished with %d failure(s)\n", failures)
		os.Exit(3)
	}
	fmt.Println("all scripts finished successfully")
}

func runGoRun(path string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start error: %w", err)
	}

	err := cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("timed out after %s", timeout.String())
	}
	if err != nil {
		return err
	}
	return nil
}

// removeEmptyDirs walks the audit root and removes any empty directories below it.
// It removes directories from deepest to shallowest so that parent dirs that become
// empty as a result will also be removed. The auditRoot itself will not be removed.
func removeEmptyDirs(auditRoot string, dryRun, verbose bool) error {
	if auditRoot == "" {
		return nil
	}

	// If auditRoot doesn't exist, nothing to do.
	info, err := os.Stat(auditRoot)
	if err != nil {
		if os.IsNotExist(err) {
			if verbose {
				fmt.Printf("audit root does not exist: %s\n", auditRoot)
			}
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("audit root is not a directory: %s", auditRoot)
	}

	// Collect directories.
	var dirs []string
	err = filepath.WalkDir(auditRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// don't stop on single path errors
			if verbose {
				fmt.Fprintf(os.Stderr, "warning walking %s: %v\n", path, walkErr)
			}
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Sort by descending path length so we remove deepest directories first.
	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})

	for _, d := range dirs {
		// Never remove the auditRoot itself.
		cleanRoot, _ := filepath.Abs(auditRoot)
		cleanDir, _ := filepath.Abs(d)
		if cleanDir == cleanRoot {
			continue
		}

		entries, err := os.ReadDir(d)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "warning: cannot read directory %s: %v\n", d, err)
			}
			continue
		}
		if len(entries) == 0 {
			if dryRun {
				fmt.Printf("DRY RUN: would remove empty directory: %s\n", d)
				continue
			}
			if verbose {
				fmt.Printf("removing empty directory: %s\n", d)
			}
			if err := os.Remove(d); err != nil {
				// report error but keep going
				fmt.Fprintf(os.Stderr, "error removing %s: %v\n", d, err)
			}
		}
	}

	return nil
}