# Cronjobs

## Run

```bash
@reboot cd /var/www/demo && ./pocketbase serve
@reboot cd /var/www/demo/storefront && pm2 start ecosystem.config.cjs
```

## Update

```bash
* * * * * cd /var/www/demo/storefront && ./bin/sync.sh
* * * * * cd /var/www/demo/storefront && robo lang
```
