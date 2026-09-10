"""Clinic settings - CARE's deployment settings served over https:// on a trusted offline
LAN (Caddy terminates a self-signed cert; :80 only redirects to https). Mounted into
the backend image at /settings/ and selected via DJANGO_SETTINGS_MODULE=clinic_settings
(see backend.env). It imports the image's own deployment settings and adjusts the
HTTPS guards for a self-signed LAN cert, then closes OTP login (USE_SMS below).
Not debug.
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

# Counter-intuitive, but this is what DISABLES OTP login. These settings inherit
# deployment (not production), so IS_PRODUCTION and USE_SMS are both False - and
# with USE_SMS False, care/emr/api/otp_viewsets/login.py stores the hardcoded
# non-production OTP "45612" and reports success. Anyone on the WiFi could then
# log in as any patient, or reset a staff password by phone number.
# True instead routes the send through SMS_BACKEND (backend.env), which raises
# immediately, so no OTP row is ever created. IS_PRODUCTION stays False on
# purpose: `care seed` runs manage.py load_fixtures, which refuses to run under
# production settings.
USE_SMS = True
