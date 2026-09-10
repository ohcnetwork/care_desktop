#!/usr/bin/env bash
# Load CARE's fixture/seed data into the running care-desktop backend container.
#
# The clinic image runs with DEBUG=False and ships no dev deps, but the fixture
# loader needs both DEBUG=True and `faker`. So this installs faker into the
# container (ephemeral) and runs load_fixtures under the base deployment settings
# with debug on -- same DB (DATABASE_URL is unchanged), so data lands in postgres.
#
# Usage: ./scripts/load_test_fixtures.sh   (from anywhere; targets the running stack)
set -euo pipefail

# Addressed by compose project name, not by a compose file, so this works from
# any directory and targets whichever stack is actually running -- the app runs
# it from its install dir, which is not this repo.
compose() { docker compose -p care-desktop "$@"; }

# ponytail: faker install is ephemeral (gone on container recreate); the *data*
# it seeds persists in the postgres volume. Bake faker into the image if you need
# to reseed often.
compose exec -T backend .venv/bin/pip install --quiet faker

compose exec -T \
  -e DJANGO_DEBUG=True \
  -e DJANGO_SETTINGS_MODULE=config.settings.deployment \
  backend .venv/bin/python manage.py load_fixtures
