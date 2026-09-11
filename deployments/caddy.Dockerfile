ARG CADDY_IMAGE

FROM ${CADDY_IMAGE}-builder AS build

ARG CORAZA_VERSION
RUN xcaddy build --with github.com/corazawaf/coraza-caddy/v2@${CORAZA_VERSION}

FROM ${CADDY_IMAGE}
COPY --from=build /usr/bin/caddy /usr/bin/caddy
