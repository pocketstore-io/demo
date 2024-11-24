package main

import (
	"log"
	"os/exec"
	"time"
)

func main() {
	// Path to the script to execute
	scriptToRun := "./bin/extend.go"

	// Time debounce in seconds
	debounceTime := 5 * time.Second

	for {
		// Execute the script
		cmd := exec.Command("go run", scriptToRun)

		// Run the command and capture any errors
		if err := cmd.Run(); err != nil {
			log.Printf("Error running script: %v\n", err)
		} else {
			log.Printf("Script executed successfully: %s\n", scriptToRun)
		}

		// Wait for the debounce period
		time.Sleep(debounceTime)
	}
}
