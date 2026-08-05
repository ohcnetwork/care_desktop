# Caddy with the Coraza WAF and the Cloudflare DNS provider compiled in (stock caddy
# ships neither). CRS is embedded in the WAF module, so it's self-contained/offline.
# The DNS module is what lets Caddy answer ACME's DNS-01 challenge, which is the only
# way this box - behind clinic NAT, never reachable from outside - can get a publicly
# trusted certificate. Built at setup (xcaddy needs internet).
ARG CADDY_IMAGE=caddy:2.11.4

FROM ${CADDY_IMAGE}-builder AS build
RUN xcaddy build \
	--with github.com/corazawaf/coraza-caddy/v2 \
	--with github.com/caddy-dns/cloudflare

FROM ${CADDY_IMAGE}
COPY --from=build /usr/bin/caddy /usr/bin/caddy
