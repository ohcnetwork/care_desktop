package clinic

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Faker is one of CARE's dev-packages, so the production backend image ships
// without it - but care/fixtures/base.py imports it at module level. Installing
// it into the running container is the whole difference between the image CARE
// publishes and the one its own fixtures expect.
// ponytail: pinned to the version in CARE's Pipfile; bump it if the fixtures need a newer one.
const fakerPin = "Faker==38.2.0"

// care_fixture_context() resets the admin password to "admin" on its way in,
// which would undo the password chosen during install. Put it back afterwards.
const restoreAdminScript = `import os
from django.contrib.auth import get_user_model
user = get_user_model().objects.get(username="admin")
user.set_password(os.environ["CARE_ADMIN_PASSWORD"])
user.save()
`

// LoadDemoData runs CARE's own bundled fixture script inside the backend
// container: `manage.py load_fixtures` with no --path loads
// care/fixtures/scripts/default_fixtures.py. No customisation on purpose.
func (e *Clinic) LoadDemoData() error {
	if e.AdminPassword == "" {
		return errors.New("the admin password is needed before demo data can be loaded")
	}
	e.logln("Installing the fixture helper (Faker) into the backend...")
	if err := e.dc("exec", "-T", "backend",
		"pip", "install", "--quiet", "--disable-pip-version-check", fakerPin); err != nil {
		return fmt.Errorf("the fixture helper could not be installed in the backend: %w", err)
	}

	e.logln("Loading demo data. This takes a few minutes...")
	// load_fixtures refuses to run when IS_PRODUCTION, and care_fixture_context()
	// refuses unless DEBUG. The deployment settings inherit base's
	// IS_PRODUCTION=False, and DJANGO_DEBUG here only affects this one exec -
	// the gunicorn server in the same container keeps its own environment.
	if err := e.dc("exec", "-T",
		"-e", "DJANGO_SETTINGS_MODULE=config.settings.deployment",
		"-e", "DJANGO_DEBUG=True",
		"backend", "python", "manage.py", "load_fixtures"); err != nil {
		return fmt.Errorf("the demo data could not be loaded: %w", err)
	}

	if err := e.restoreAdminPassword(); err != nil {
		return err
	}
	e.logln("Demo data loaded.")
	return nil
}

func (e *Clinic) restoreAdminPassword() error {
	cmd := proc.Command("docker", "compose", "exec", "-T",
		"-e", "CARE_ADMIN_PASSWORD",
		"backend", "python", "manage.py", "shell", "-c", restoreAdminScript)
	cmd.Env = append(e.baseEnv(), "CARE_ADMIN_PASSWORD="+e.AdminPassword)
	cmd.Dir = e.workdir()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"demo data was loaded, but the admin password is now \"admin\" and could not be reset: %w: %s",
			err, strings.TrimSpace(string(out)))
	}
	return nil
}
