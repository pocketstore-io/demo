#!/bin/bash

# Path to the script you want to execute
SCRIPT_TO_RUN="./bin/extend.sh"

# Time debounce in seconds
DEBOUNCE_TIME=5

while true; do
    # Execute the script
    bash "$SCRIPT_TO_RUN"
    
    # Wait for the debounce period
    sleep $DEBOUNCE_TIME
done