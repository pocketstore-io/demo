package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runCommand runs a command with optional arguments and detached mode.
// If `detached` is true, the command runs in the background.
func runCommand(command string, args []string, detached bool) error {
	cmd := exec.Command(command, args...)

	// Handle detached mode for background execution
	if detached {
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to run command '%s %s' in detached mode: %w", command, strings.Join(args, " "), err)
		}
		return nil // Detached commands don't block, so we return early
	}

	// Attach stdout and stderr for visible output
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command '%s %s': %w", command, strings.Join(args, " "), err)
	}

	return nil
}

func main() {
	// Store the initial directory to return later
	initialDir, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting the current directory:", err)
		return
	}

	// 1. Run ./bin/update.go
	err = runCommand("go", []string{"run", filepath.Join(".", "bin", "update.go")}, false)
	if err != nil {
		fmt.Println("Error running ./bin/update.go:", err)
		return
	}

	// 2. Change directory to storefront
	err = os.Chdir("storefront")
	if err != nil {
		fmt.Println("Error changing directory to storefront:", err)
		return
	}

	// 3. Run ./bin/sync.go
	err = runCommand("go", []string{"run", filepath.Join(".", "bin", "sync.go")}, false)
	if err != nil {
		fmt.Println("Error running ./bin/sync.go:", err)
		return
	}

	// 4. Run bun install
	err = runCommand("bun", []string{"install"}, false)
	if err != nil {
		fmt.Println("Error running bun install:", err)
		return
	}

	// 5. Run go run lang.go
	err = runCommand("go", []string{"run", filepath.Join(".", "bin", "lang.go"), "lang"}, false)
	if err != nil {
		fmt.Println("Error running go run lang.go:", err)
		return
	}

	// 6. Run bun run build
	err = runCommand("bun", []string{"run", "build"}, false)
	if err != nil {
		fmt.Println("Error running bun run build:", err)
		return
	}

	// 7. Run pm2 start ecosystem.config.cjs
	err = runCommand("pm2", []string{"start", "ecosystem.config.cjs"}, false)
	if err != nil {
		fmt.Println("Error running pm2 start ecosystem.config.cjs:", err)
		return
	}

	// 8. Change directory back to the original path
	err = os.Chdir(initialDir)
	if err != nil {
		fmt.Println("Error changing back to the original directory:", err)
		return
	}

	// 9. Run ./pocketbase serve in detached mode
	err = runCommand(filepath.Join(".", "pocketbase"), []string{"serve"}, true)
	if err != nil {
		fmt.Println("Error running ./pocketbase serve:", err)
		return
	}

	fmt.Println("All commands executed successfully.")
}
