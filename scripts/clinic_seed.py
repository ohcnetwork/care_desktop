"""Seed a fresh clinic: one region, one facility, and its staff.

Run inside the backend container by the desktop app's "Clinic details" screen:

    docker compose exec -T backend python manage.py load_fixtures --path /clinic_seed.py

The request is JSON on stdin; the reply is one `CLINIC_SEED_RESULT <json>` line on
stdout (load_fixtures prints its own banner around it, so the marker is how the
caller finds ours). Two actions:

    {"action": "options"}   -> the roles and facility types this install accepts
    {"action": "seed", ...} -> create the region, facility, staff and memberships

Deliberately does NOT use care.fixtures.care_fixture_context(), even though this
mirrors what it does. On a clinic box that context would: refuse to run unless
DEBUG is on; reset the admin password to "admin", undoing the one chosen during
install; import Faker, a dev-package absent from this image; and permanently
no-op valueset validation for the whole process. Its two prerequisite steps are
the part worth having, so they are run below instead. Writes still go through the
same DRF viewsets those fixtures use, so validation, slug generation and audit
logs run exactly as for a real API call.

Reads that only need to find an existing row go straight to the ORM; only writes
go through the API.
"""

import json
import sys
from pathlib import Path

from django.contrib.auth import get_user_model
from django.core.management import call_command
from django.db import transaction
from django.urls import reverse
from rest_framework import status as http_status
from rest_framework.test import APIClient

from care.emr.models import Organization
from care.facility.models import REVERSE_REVERSE_FACILITY_TYPES

ADMIN_USERNAME = "admin"

# Roles are a fixed, small set (sync_permissions_roles seeds them), but the
# endpoint is paginated -- ask for more than there could ever be so a default
# page size can never silently truncate the list a clinic picks from.
ROLE_PAGE_SIZE = 500

# The facility organization every new facility is created with. Staff are added
# here so they can actually work in the facility, not just exist as logins.
DEFAULT_FACILITY_ORG = "Administration"

GENDERS = ("male", "female", "non_binary", "transgender")

# Content CARE ships but nothing installs: without questionnaires there is nothing
# to fill in during an encounter, and without report templates there is no
# discharge or treatment summary to print. Paths are relative to the backend's
# working directory (/app), the same ones CARE's own fixtures read.
QUESTIONNAIRE_FIXTURES = Path("data/questionnaire_fixtures.json")
TEMPLATE_FIXTURES = Path("data/template_fixtures.json")

# Defaults for a template entry that omits them, matching CareFixtureBase.
TEMPLATE_DEFAULTS = {
    "template_type": "discharge_summary",
    "context": "encounter_base",
    "status": "active",
    "default_format": "html",
}

RESULT_MARKER = "CLINIC_SEED_RESULT"


class SeedError(Exception):
    """A problem worth showing the person filling in the form."""


# --- input validation -------------------------------------------------------
# This data comes from a form, so nothing below trusts it.


def require(value, label):
    text = str(value if value is not None else "").strip()
    if not text:
        msg = f"{label} is required."
        raise SeedError(msg)
    return text


def require_pincode(value, label):
    text = require(value, label)
    if not text.isdigit():
        msg = f"{label} must be digits only."
        raise SeedError(msg)
    return int(text)


def require_choice(value, choices, label):
    text = require(value, label)
    if text not in choices:
        msg = f"{label} must be one of: {', '.join(choices)}."
        raise SeedError(msg)
    return text


# --- api plumbing -----------------------------------------------------------


def sync_prerequisites():
    """Make sure the rows we look up exist before we look them up.

    Nothing in the install runs these: the api container's start.sh does not, and
    celery-beat does but only when it boots, which races the seed at the end of a
    first install. Both are idempotent, so running them here is safe and removes
    the race.
    """
    call_command("sync_permissions_roles")
    call_command("sync_valueset")


def api_client():
    """Act as the admin the installer already created, without touching its password."""
    user_model = get_user_model()
    admin = user_model.objects.filter(username=ADMIN_USERNAME).first()
    if admin is None:
        msg = (
            f"No '{ADMIN_USERNAME}' user found. Finish the install before "
            f"adding clinic details."
        )
        raise SeedError(msg)
    client = APIClient()
    client.force_authenticate(user=admin)
    return client


def post(client, url, data):
    response = client.post(url, data, format="json")
    if response.status_code not in (
        http_status.HTTP_200_OK,
        http_status.HTTP_201_CREATED,
    ):
        msg = f"POST {url} failed ({response.status_code}): {response.data}"
        raise SeedError(msg)
    return response.data


def get(client, url, params=None):
    response = client.get(url, params or {}, format="json")
    if response.status_code != http_status.HTTP_200_OK:
        msg = f"GET {url} failed ({response.status_code}): {response.data}"
        raise SeedError(msg)
    return response.data


def results(payload):
    return payload.get("results", payload) if isinstance(payload, dict) else payload


# --- actions ----------------------------------------------------------------


def list_roles(client):
    """Role name -> id, as offered by this install."""
    payload = get(client, reverse("role-list"), {"limit": ROLE_PAGE_SIZE})
    return {role["name"]: role["id"] for role in results(payload)}


def get_or_create_organization(client, name, org_type):
    """An organization id, reusing one of the same name and type if it exists.

    Re-running the screen (a failed seed, a second facility) must not trip over
    organizations it created itself -- role organizations especially are shared
    by every member holding that role.
    """
    existing = Organization.objects.filter(name=name, org_type=org_type).first()
    if existing is not None:
        return str(existing.external_id)
    created = post(
        client,
        reverse("organization-list"),
        {"name": name, "org_type": org_type, "active": True},
    )
    return created["id"]


def facility_org_id(client, facility_id, name):
    """The named organization inside a facility, or None if it has none."""
    payload = get(
        client,
        reverse(
            "facility-organization-list",
            kwargs={"facility_external_id": facility_id},
        ),
    )
    for org in results(payload):
        if org["name"] == name:
            return org["id"]
    return None


def create_facility(client, request, geo_id):
    facility = request.get("facility") or {}
    return post(
        client,
        reverse("facility-list"),
        {
            "name": require(facility.get("name"), "Facility name"),
            "description": str(facility.get("description") or "").strip(),
            "facility_type": require(facility.get("facility_type"), "Facility type"),
            "address": require(facility.get("address"), "Facility address"),
            "pincode": require_pincode(facility.get("pincode"), "Facility pincode"),
            "phone_number": require(
                facility.get("phone_number"), "Facility phone number"
            ),
            # A clinic box is a private LAN install: nothing here is listed publicly,
            # and no coordinates are asked for rather than guessed.
            "is_public": False,
            "features": [],
            "geo_organization": geo_id,
        },
    )


def create_member(client, member, label, geo_id, roles):
    role_name = require(member.get("role"), f"{label} role")
    if role_name not in roles:
        msg = f"{label}: '{role_name}' is not a role on this install."
        raise SeedError(msg)
    role_org_id = get_or_create_organization(client, role_name, "role")
    user = post(
        client,
        reverse("users-list"),
        {
            "username": require(member.get("username"), f"{label} username"),
            "first_name": require(member.get("first_name"), f"{label} first name"),
            "last_name": require(member.get("last_name"), f"{label} last name"),
            "email": require(member.get("email"), f"{label} email"),
            "phone_number": require(member.get("phone_number"), f"{label} phone number"),
            "gender": require_choice(member.get("gender"), GENDERS, f"{label} gender"),
            "geo_organization": geo_id,
            "password": member.get("password") or None,
            "role_orgs": [
                {"organization": role_org_id, "role": roles[role_name]},
            ],
        },
    )
    return user, roles[role_name]


def seed(client, request):
    geo_id = get_or_create_organization(
        client,
        require(request.get("geo_organization"), "Region name"),
        "govt",
    )
    facility = create_facility(client, request, geo_id)
    facility_id = facility["id"]

    roles = list_roles(client)
    default_org_id = facility_org_id(client, facility_id, DEFAULT_FACILITY_ORG)

    members = []
    for index, member in enumerate(request.get("members") or [], start=1):
        label = f"Member {index}"
        user, role_id = create_member(client, member, label, geo_id, roles)
        if default_org_id is not None:
            post(
                client,
                reverse(
                    "facility-organization-users-list",
                    kwargs={
                        "facility_external_id": facility_id,
                        "facility_organizations_external_id": default_org_id,
                    },
                ),
                {"user": user["id"], "role": role_id},
            )
        members.append({"username": user["username"], "role": member.get("role")})

    return {
        "geo_organization": geo_id,
        "facility": {"id": facility_id, "name": facility["name"]},
        "members": members,
    }


# --- bundled content --------------------------------------------------------


def read_fixture_file(path):
    """The bundled JSON, or nothing if this backend version does not ship it."""
    if not path.exists():
        return []
    with path.open() as handle:
        return json.load(handle)


def post_each(client, url, entries):
    """POST every entry, skipping the ones the API rejects; return how many stuck.

    Each entry gets its own savepoint so a rejected one cannot poison the rest.
    Failures are counted rather than raised: this is CARE's own data, and a clinic
    with one unloadable questionnaire is still a working clinic.
    """
    loaded = 0
    for entry in entries:
        try:
            with transaction.atomic():
                post(client, url, entry)
        except SeedError:
            continue
        loaded += 1
    return loaded


def load_bundled_content(client, geo_id, facility_id):
    questionnaires = post_each(
        client,
        reverse("questionnaire-list"),
        [
            {**entry, "organizations": [geo_id]}
            for entry in read_fixture_file(QUESTIONNAIRE_FIXTURES)
        ],
    )
    templates = post_each(
        client,
        reverse("template-list"),
        [
            {**TEMPLATE_DEFAULTS, **entry, "facility": facility_id}
            for entry in read_fixture_file(TEMPLATE_FIXTURES)
        ],
    )
    return {"questionnaires": questionnaires, "templates": templates}


def main():
    try:
        request = json.load(sys.stdin)
    except ValueError as exc:
        msg = f"Could not read the clinic details: {exc}"
        raise SeedError(msg) from exc
    if not isinstance(request, dict):
        msg = "Clinic details must be a JSON object."
        raise SeedError(msg)

    sync_prerequisites()
    client = api_client()
    action = request.get("action", "seed")
    if action == "options":
        # The screen's two pickers. Both come from the backend rather than a list
        # kept here: roles come from CARE's role sync, and facility types are an
        # exact-match validated set that changes with the backend version.
        return {
            "roles": sorted(list_roles(client)),
            "facility_types": sorted(REVERSE_REVERSE_FACILITY_TYPES),
        }
    if action == "seed":
        # One transaction for the whole clinic: a bad member must not leave a
        # half-built facility behind for the next attempt to collide with.
        with transaction.atomic():
            result = seed(client, request)
        # Deliberately after that transaction commits, not inside it. The facility
        # and staff are what the operator typed; a bad entry in a file we ship
        # must never roll their work back.
        result.update(load_bundled_content(client, result["geo_organization"], result["facility"]["id"]))
        return result
    msg = f"Unknown action '{action}'."
    raise SeedError(msg)


try:
    print(f"{RESULT_MARKER} {json.dumps(main())}")  # noqa: T201
except SeedError as exc:
    print(f"{RESULT_MARKER} {json.dumps({'error': str(exc)})}")  # noqa: T201
    raise SystemExit(1) from exc
