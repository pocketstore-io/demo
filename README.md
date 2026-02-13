# PocketStore - Demo Template

## Packages

![](https://raw.githubusercontent.com/pocketstore-io/demo/refs/heads/main/.github/badges/baseline.svg)

## Dependecies Storefront

![](https://raw.githubusercontent.com/pocketstore-io/storefront/refs/heads/main/.github/badges/nuxt.svg)
![](https://raw.githubusercontent.com/pocketstore-io/storefront/refs/heads/main/.github/badges/vue.svg)
![](https://raw.githubusercontent.com/pocketstore-io/storefront/refs/heads/main/.github/badges/eslint.svg)
![](https://raw.githubusercontent.com/pocketstore-io/storefront/refs/heads/main/.github/badges/vite.svg)
![](https://raw.githubusercontent.com/pocketstore-io/storefront/refs/heads/main/.github/badges/daisyui.svg)
![](https://raw.githubusercontent.com/pocketstore-io/storefront/refs/heads/main/.github/badges/pinia.svg)
![](https://raw.githubusercontent.com/pocketstore-io/storefront/refs/heads/main/.github/badges/tailwindcss.svg)
![](https://raw.githubusercontent.com/pocketstore-io/storefront/refs/heads/main/.github/badges/pocketbase.svg)

## Tools

![](https://img.shields.io/badge/Hetzner+Cloud-Server-red)
![](https://img.shields.io/badge/ChatGpt-Code+Support-red)
![](https://img.shields.io/badge/VsCode-Editor-red)
![](https://img.shields.io/badge/PhpStorm-Editor-red)

## Get Started

install the requirements under
[www.PocketStore.io](https://www.PocketStore.io).
and your read to go.

to customization follow the guide under
[www.PocketStore.io/customazation](https://www.PocketStore.io/customazation).


```bash
git clone https://github.com/pocketstore-io/demo.git
```

```bash
cd demo
```

```bash
docker compose up
```

## Creating a PocketBase Admin User

After starting the containers, you need to create an admin user to access the PocketBase admin panel.

### Using the provided script:

The script supports multiple ways to provide credentials for security:

**1. Interactive prompts (most secure - recommended):**
```bash
./bin/create-admin.sh
# You'll be prompted for email and password
```

**2. Environment variables (secure for automation):**
```bash
ADMIN_EMAIL=admin@example.com ADMIN_PASSWORD=your_secure_password ./bin/create-admin.sh
```

**3. Using stdin (secure for scripts):**
```bash
echo "your_secure_password" | ADMIN_EMAIL=admin@example.com ./bin/create-admin.sh
```

**4. Command-line arguments (⚠️ NOT recommended for production):**
```bash
./bin/create-admin.sh admin@yourdomain.com your_secure_password
```
> **Warning:** Using command-line arguments exposes credentials in shell history and process lists. Use this method only for local development.

**Note:** The password must be at least 10 characters long.

### Manual creation:

Alternatively, you can create an admin user directly using docker exec:

```bash
docker exec -i pb_test pocketbase superuser create admin@example.com your_password
```

### Accessing the Admin Panel:

Once the admin user is created, you can access the PocketBase admin panel at:
- Local: `http://localhost:8090/_/`
- Production: `http://admin.pocketstore.io/_/`

This is our Demo Template for Demo.PocketStore.io


github.com - secrets
```
host: SSH_HOST
username: SSH_USER
key: SSH_KEY
port: SSH_PORT
folder: SSH_FOLDER
```

For env prod:
```
echo "PORT_NUXT=${{ secrets.PORT_NUXT }}" >> .env
echo "PORT_POCKETBASE=${{ secrets.PORT_POCKETBASE }}" >> .env
echo "CONTAINER_NUXT=${{ secrets.CONTAINER_NUXT }}" >> .env
echo "CONTAINER_POCKETBASE=${{ secrets.CONTAINER_POCKETBASE }}" >> .env
```
