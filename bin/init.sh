#!/bin/bash

./bin/update.sh && cd storefront && ./bin/sync.sh && bun install && composer update && robo lang && bun run build && pm2 start ecosystem.config.cjs && cd .. && ./pocketbase serve
