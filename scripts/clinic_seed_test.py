"""Self-check for clinic_seed.py's contract with the Go caller: stdin in, one
`CLINIC_SEED_RESULT <json>` line out, non-zero exit on a bad request.

    python3 scripts/clinic_seed_test.py

Django and CARE are stubbed, so this runs anywhere -- it checks the request
handling and the reply protocol, not what CARE does with a valid request. That
part needs a running stack, so it is covered by running the installer instead.
"""

import io
import json
import runpy
import sys
from contextlib import redirect_stdout
from pathlib import Path
from unittest.mock import MagicMock

SCRIPT = Path(__file__).with_name("clinic_seed.py")

for name in (
    "django",
    "django.contrib",
    "django.core",
    "django.core.management",
    "django.contrib.auth",
    "django.db",
    "django.urls",
    "rest_framework",
    "rest_framework.test",
    "care",
    "care.emr",
    "care.emr.models",
    "care.facility",
    "care.facility.models",
):
    sys.modules.setdefault(name, MagicMock())


def run(request):
    """Run the script against a request, returning (parsed reply, exit code)."""
    stdin, sys.stdin = sys.stdin, io.StringIO(request)
    out = io.StringIO()
    code = 0
    try:
        with redirect_stdout(out):
            runpy.run_path(str(SCRIPT), run_name="__main__")
    except SystemExit as exc:
        code = exc.code
    finally:
        sys.stdin = stdin

    marker = next(
        (
            line
            for line in out.getvalue().splitlines()
            if line.startswith("CLINIC_SEED_RESULT ")
        ),
        None,
    )
    assert marker is not None, f"no result line in output: {out.getvalue()!r}"
    return json.loads(marker[len("CLINIC_SEED_RESULT ") :]), code


def expect_error(request, fragment):
    reply, code = run(request)
    assert code == 1, f"expected exit 1 for {fragment!r}, got {code}"
    assert fragment in reply.get("error", ""), (
        f"expected an error mentioning {fragment!r}, got {reply!r}"
    )


def main():
    # A reply always parses and always says which field was wrong: the Go side
    # shows this string to the person filling in the form.
    expect_error("not json", "Could not read")
    expect_error('["a list"]', "must be a JSON object")
    expect_error('{"action": "nope"}', "Unknown action")
    expect_error("{}", "Region name is required")
    expect_error('{"geo_organization": "   "}', "Region name is required")
    expect_error(
        '{"geo_organization": "Ernakulam", "facility": {}}',
        "Facility name is required",
    )
    expect_error(
        json.dumps(
            {
                "geo_organization": "Ernakulam",
                "facility": {
                    "name": "Town Clinic",
                    "facility_type": "Clinic",
                    "address": "Main Road",
                    "pincode": "not-a-pincode",
                    "phone_number": "+919999999999",
                },
            }
        ),
        "must be digits only",
    )
    print("clinic_seed.py: all checks passed")  # noqa: T201


main()
