"""Clinic settings - production settings served over https:// on a trusted offline
LAN (Caddy terminates a self-signed cert; :80 only redirects to https). Mounted into
the backend image at /settings/ and selected via DJANGO_SETTINGS_MODULE=clinic_settings
(see backend.env). It imports the image's own production settings and adjusts the
HTTPS guards for a self-signed LAN cert. Not debug.
"""

from config.settings.deployment import *  # noqa: F401,F403

DEBUG = False                   # never debug on a clinic box
SECURE_SSL_REDIRECT = False     # Caddy already redirects http -> https; don't double it
SESSION_COOKIE_SECURE = True    # https-only origin
CSRF_COOKIE_SECURE = True
SECURE_HSTS_SECONDS = 0         # self-signed cert - don't HSTS-pin clients to https
# Caddy terminates TLS and forwards to the backend over http with this header, so
# Django sees the original https request (request.is_secure(), secure cookies, CSRF).
SECURE_PROXY_SSL_HEADER = ("HTTP_X_FORWARDED_PROTO", "https")
CORS_ALLOW_ALL_ORIGINS = True   # the reverse proxy is same-origin anyway
