# Use an official Node.js base image
FROM node:24-alpine

RUN apk add go git
# Set the working directory
COPY . /var/www/demo
WORKDIR /var/www/demo
RUN go run bin/update.go
RUN go run bin/custom.go
RUN go run bin/plugins.go
RUN go run bin/translations.go

WORKDIR /var/www/demo/storefront

# Install global dependencies
RUN npm install -g pm2 npm bun
RUN go run bin/sitemap.go

# Install project dependencies
RUN bun install
RUN bun run build

# Expose the desired port
EXPOSE 3000

CMD ["bun", "x","nuxi", "preview"]
