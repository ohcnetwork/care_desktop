# Postgres + openssl, so daily dumps can be encrypted (stock postgres has no crypto
# CLI). Built at setup — `apk add` needs internet, the clinic is offline after.
ARG POSTGRES_IMAGE=postgres:17.10-alpine
FROM ${POSTGRES_IMAGE}
RUN apk add --no-cache openssl
