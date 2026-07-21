# Caddy with the Coraza WAF compiled in (stock caddy ships none). CRS is embedded in
# the module, so it's self-contained/offline. Built at setup (xcaddy needs internet).
ARG CADDY_IMAGE=caddy:2.11.4

FROM ${CADDY_IMAGE}-builder AS build
RUN xcaddy build --with github.com/corazawaf/coraza-caddy/v2

FROM ${CADDY_IMAGE}
COPY --from=build /usr/bin/caddy /usr/bin/caddy
