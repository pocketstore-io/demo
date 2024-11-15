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
	// Run ./pocketbase update
	err := runCommand("./pocketbase", []string{"update"})
	if err != nil {
		fmt.Println("Error running ./pocketbase update:", err)
		return
	}

	fmt.Println("Pocketbase updated successfully.")
}
