"""Clinic settings - CARE's production settings, adjusted for running behind Caddy.

Mounted at /settings/ and selected via DJANGO_SETTINGS_MODULE (see backend.env).
Rationale for each setting: docs/architecture.md#django-behind-the-proxy
"""

import os

from config.settings.deployment import *  # noqa: F401,F403

PUBLIC_ORIGIN = os.environ.get("CARE_PUBLIC_ORIGIN", "").rstrip("/")

DEBUG = False

# Caddy terminates TLS, so Django must be told the browser's original scheme or it
# builds http:// absolute URLs. Safe to trust: Caddy sets this itself on every
# proxied request, and the backend port is never published outside the compose network.
SECURE_PROXY_SSL_HEADER = ("HTTP_X_FORWARDED_PROTO", "https")

SESSION_COOKIE_SECURE = True
CSRF_COOKIE_SECURE = True

SECURE_SSL_REDIRECT = False  # nothing to redirect: Caddy is the only listener, HTTPS-only
SECURE_HSTS_SECONDS = 0  # Caddy sends HSTS at the edge; setting it here would duplicate it

SECURE_CONTENT_TYPE_NOSNIFF = True
SECURE_REFERRER_POLICY = "strict-origin-when-cross-origin"

# One origin behind Caddy, so nothing legitimate is cross-origin.
CORS_ALLOW_ALL_ORIGINS = False
CORS_ALLOWED_ORIGINS = [PUBLIC_ORIGIN] if PUBLIC_ORIGIN else []
