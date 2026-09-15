ARG POSTGRES_IMAGE
FROM ${POSTGRES_IMAGE}
RUN apk add --no-cache openssl
