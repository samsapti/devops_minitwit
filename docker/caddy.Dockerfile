FROM caddy:2-alpine

COPY docker/caddy/Caddyfile /etc/caddy/Caddyfile
WORKDIR /srv

CMD [ "caddy" "run" "--config" "/etc/caddy/Caddyfile" "--adapter" "caddyfile" ]