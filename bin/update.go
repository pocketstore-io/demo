package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runCommand runs a command and returns any error
func runCommand(command string, args []string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run command '%s %s': %w", command, strings.Join(args, " "), err)
	}
	return nil
}

func main() {
	// 1. Run git submodule update --init --recursive
	err := runCommand("git", []string{"submodule", "update", "--init", "--recursive"})
	if err != nil {
		fmt.Println("Error updating submodules:", err)
		return
	}

	2. Optionally run git submodule foreach git fetch
	Uncomment the lines below to fetch updates for all submodules
	err = runCommand("git", []string{"submodule", "foreach", "git", "checkout", "main"})
	if err != nil {
		fmt.Println("Error fetching for submodules:", err)
		return
	}

	3. Optionally run git submodule foreach git pull
	Uncomment the lines below to pull the latest changes for each submodule
	err = runCommand("git", []string{"submodule", "foreach", "git", "pull"})
	if err != nil {
		fmt.Println("Error pulling for submodules:", err)
		return
	}

	fmt.Println("Submodules updated successfully.")
}
