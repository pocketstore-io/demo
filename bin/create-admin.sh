#!/bin/bash

# Script to create a PocketBase admin/superuser
# Usage: ./bin/create-admin.sh [email] [password]

set -e

# Default values
DEFAULT_EMAIL="admin@example.com"
DEFAULT_PASSWORD="admin123456"

# Get email and password from arguments or use defaults
ADMIN_EMAIL="${1:-$DEFAULT_EMAIL}"
ADMIN_PASSWORD="${2:-$DEFAULT_PASSWORD}"

# Validate email format
if [[ ! "$ADMIN_EMAIL" =~ ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$ ]]; then
    echo "Error: Invalid email format"
    echo "Usage: $0 [email] [password]"
    exit 1
fi

# Validate password length (minimum 10 characters for PocketBase)
if [ ${#ADMIN_PASSWORD} -lt 10 ]; then
    echo "Error: Password must be at least 10 characters long"
    echo "Usage: $0 [email] [password]"
    exit 1
fi

echo "=== Creating PocketBase Admin User ==="
echo "Email: $ADMIN_EMAIL"
echo "Password: $(echo $ADMIN_PASSWORD | sed 's/./*/g')"
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
echo "Creating admin user..."
docker exec -it "$CONTAINER_NAME" pocketbase superuser create "$ADMIN_EMAIL" "$ADMIN_PASSWORD"

echo ""
echo "=== Admin User Created Successfully ==="
echo "You can now log in to the PocketBase admin panel at:"
echo "http://localhost:${PORT_POCKETBASE:-8090}/_/"
echo ""
echo "Email: $ADMIN_EMAIL"
echo "Password: [hidden]"
