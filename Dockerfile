# Use an official Node.js base image
FROM node:22-alpine

# Set the working directory
COPY . /var/www/demo
WORKDIR /var/www/demo/storefront

# Install global dependencies
RUN npm install -g pm2 bun npm

# Install project dependencies
RUN bun install && bun run build

# Expose the desired port
EXPOSE 4000

CMD ["bun", "run", "preview"]
