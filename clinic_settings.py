"""Plain-HTTP clinic settings - production settings reachable over http:// on a
trusted offline LAN. Mounted into the backend image at /settings/ and selected via
DJANGO_SETTINGS_MODULE=clinic_settings (see backend.env). It imports the image's own
production settings and relaxes only the HTTPS-only guards, so /admin and the API
work over plain http on the LAN. Not debug.
"""

from config.settings.deployment import *  # noqa: F401,F403

DEBUG = False                   # never debug on a clinic box
SECURE_SSL_REDIRECT = False     # don't bounce LAN http -> https
SESSION_COOKIE_SECURE = False   # let admin/session + CSRF cookies work over http
CSRF_COOKIE_SECURE = False
SECURE_HSTS_SECONDS = 0
CORS_ALLOW_ALL_ORIGINS = True   # the reverse proxy is same-origin anyway
