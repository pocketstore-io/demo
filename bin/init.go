package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runCommand runs a command and returns any error
func runCommand(command string, args []string, detached bool) error {
	cmd := exec.Command(command, args...)
	if detached {
		// Run the command in detached mode (background)
		cmd.Stdout = nil
		cmd.Stderr = nil
		err := cmd.Start()
		if err != nil {
			return fmt.Errorf("failed to run command '%s %s' in detached mode: %w", command, strings.Join(args, " "), err)
		}
	} else {
		// Run the command and wait for it to finish
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to run command '%s %s': %w", command, strings.Join(args, " "), err)
		}
	}
	return nil
}

func main() {
	// Run the commands in sequence as per the provided script

	// 1. Run ./bin/update.go
	err := runCommand("go run ./bin/update.go", nil, false)
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
	err = runCommand("go run ./bin/sync.go", nil, false)
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

	// 6. Run go run lang.go
	err = runCommand("go run ./bin/lang.go", []string{"lang"}, false)
	if err != nil {
		fmt.Println("Error running go run lang:", err)
		return
	}

	// 7. Run bun run build
	err = runCommand("bun", []string{"run", "build"}, false)
	if err != nil {
		fmt.Println("Error running bun run build:", err)
		return
	}

	// 8. Run pm2 start ecosystem.config.cjs
	err = runCommand("pm2", []string{"start", "ecosystem.config.cjs"}, false)
	if err != nil {
		fmt.Println("Error running pm2 start:", err)
		return
	}

	// 9. Change directory back to the original path
	err = os.Chdir("..")
	if err != nil {
		fmt.Println("Error changing back to the original directory:", err)
		return
	}

	// 10. Run ./pocketbase serve in detached mode
	err = runCommand("./pocketbase", []string{"serve"}, true)
	if err != nil {
		fmt.Println("Error running ./pocketbase serve:", err)
		return
	}

	fmt.Println("All commands executed successfully.")
}
