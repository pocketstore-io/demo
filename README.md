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
