#!/bin/bash

# Script to create a PocketBase admin/superuser
# Usage: ./bin/create-admin.sh [email] [password]
#
# For better security, you can also use environment variables:
# ADMIN_EMAIL=admin@example.com ADMIN_PASSWORD=your_password ./bin/create-admin.sh
#
# Or use stdin:
# echo "your_password" | ADMIN_EMAIL=admin@example.com ./bin/create-admin.sh

set -e

# Get email from argument or environment variable
ADMIN_EMAIL="${1:-${ADMIN_EMAIL:-}}"

# Get password from argument, environment variable, or stdin
if [ -n "$2" ]; then
    ADMIN_PASSWORD="$2"
elif [ -n "$ADMIN_PASSWORD" ]; then
    # Password already set from environment
    :
elif [ ! -t 0 ]; then
    # Read from stdin if available (non-interactive)
    read -r -s ADMIN_PASSWORD
else
    # Prompt for credentials if not provided
    if [ -z "$ADMIN_EMAIL" ]; then
        read -p "Enter admin email: " ADMIN_EMAIL
    fi
    read -s -p "Enter admin password (min 10 characters): " ADMIN_PASSWORD
    echo
fi

# Validate email
if [ -z "$ADMIN_EMAIL" ]; then
    echo "Error: Email is required"
    echo "Usage: $0 [email] [password]"
    echo "Or: ADMIN_EMAIL=email ADMIN_PASSWORD=password $0"
    exit 1
fi

if [[ ! "$ADMIN_EMAIL" =~ ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$ ]]; then
    echo "Error: Invalid email format"
    exit 1
fi

# Validate password
if [ -z "$ADMIN_PASSWORD" ]; then
    echo "Error: Password is required"
    exit 1
fi

if [ ${#ADMIN_PASSWORD} -lt 10 ]; then
    echo "Error: Password must be at least 10 characters long"
    exit 1
fi

echo "=== Creating PocketBase Admin User ==="
echo "Email: $ADMIN_EMAIL"
echo "Password: ${ADMIN_PASSWORD//?/*}"
echo ""

# Get the PocketBase container name from environment or use default
CONTAINER_NAME="${CONTAINER_POCKETBASE:-pb_test}"

# Check if container exists and is running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Error: PocketBase container '${CONTAINER_NAME}' is not running"
    echo "Please start the container first with: docker compose up -d"
    exit 1
fi

# Create the superuser using docker exec
# Note: PocketBase requires both email and password as command arguments
echo "Creating admin user..."
docker exec -i "$CONTAINER_NAME" pocketbase superuser create "$ADMIN_EMAIL" "$ADMIN_PASSWORD"

echo ""
echo "=== Admin User Created Successfully ==="
echo "You can now log in to the PocketBase admin panel at:"
echo "http://localhost:${PORT_POCKETBASE:-8090}/_/"
echo ""
echo "Email: $ADMIN_EMAIL"
echo "Password: [hidden]"
