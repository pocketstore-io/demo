# Use an official Node.js base image
FROM node:22-alpine

RUN apk add go git
# Set the working directory
COPY . /var/www/demo
WORKDIR /var/www/demo
RUN go run bin/update.go
RUN go run bin/extend.go

WORKDIR /var/www/demo/storefront
RUN go run bin/lang.go

# Install global dependencies
RUN npm install -g pm2 npm

# Install project dependencies
RUN npm install

# Expose the desired port
EXPOSE 3000

CMD ["npx", "nuxi", "preview"]
