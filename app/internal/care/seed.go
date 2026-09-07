package care

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

// seedScript is where docker-compose.yml mounts scripts/clinic_seed.py.
const seedScript = "/clinic_seed.py"

// resultMarker prefixes the one line of clinic_seed.py output we parse. The
// management command prints its own banner around the script, so the reply needs
// a marker rather than "the last line".
const resultMarker = "CLINIC_SEED_RESULT"

// ClinicSeed is the clinic's starting data, as collected by the setup screen.
type ClinicSeed struct {
	// GeoOrganization is the region the facility sits in (a "govt" organization).
	GeoOrganization string       `json:"geo_organization"`
	Facility        SeedFacility `json:"facility"`
	Members         []SeedMember `json:"members"`
}

type SeedFacility struct {
	Name         string `json:"name"`
	FacilityType string `json:"facility_type"`
	Address      string `json:"address"`
	Pincode      string `json:"pincode"`
	PhoneNumber  string `json:"phone_number"`
	Description  string `json:"description"`
}

type SeedMember struct {
	Username    string `json:"username"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
	Gender      string `json:"gender"`
	Role        string `json:"role"`
	Password    string `json:"password"`
}

// SeedResult is what was created, for the screen to confirm back.
type SeedResult struct {
	Facility struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"facility"`
	Members []struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	} `json:"members"`
	// Counts of the content CARE ships that the seed loads alongside the clinic:
	// questionnaires to fill in during an encounter, and printable report templates.
	Questionnaires int      `json:"questionnaires"`
	Templates      int      `json:"templates"`
	Roles          []string `json:"roles"`
	FacilityTypes  []string `json:"facility_types"`
	Error          string   `json:"error"`
}

// SeedOptions is what the clinic details form's pickers offer. Read from the
// running backend rather than hardcoded: roles come from CARE's
// sync_permissions_roles, and facility types are validated against an exact list
// that moves with the backend version.
type SeedOptions struct {
	Roles         []string `json:"roles"`
	FacilityTypes []string `json:"facility_types"`
}

// Options lists the roles and facility types this install accepts.
func (e *Engine) Options() (*SeedOptions, error) {
	res, err := e.runSeedScript(map[string]string{"action": "options"})
	if err != nil {
		return nil, err
	}
	return &SeedOptions{Roles: res.Roles, FacilityTypes: res.FacilityTypes}, nil
}

// SeedClinic creates the region, facility, staff and their facility memberships.
// One transaction inside CARE: it either all lands or none of it does, so a
// rejected member leaves nothing behind and the screen can be retried as-is.
// CARE's bundled questionnaires and report templates are loaded afterwards, on
// their own, so a bad entry in one cannot roll back what the operator typed.
func (e *Engine) SeedClinic(seed ClinicSeed) (*SeedResult, error) {
	if err := seed.validate(); err != nil {
		return nil, err
	}
	e.logln("Adding your clinic details...")
	res, err := e.runSeedScript(seed)
	if err != nil {
		return nil, err
	}
	e.logln("Added " + res.Facility.Name + fmt.Sprintf(" with %d staff member(s).", len(res.Members)))
	e.logln(fmt.Sprintf("Loaded %d questionnaires and %d report templates.", res.Questionnaires, res.Templates))
	return res, nil
}

// validate catches the empty fields here, where the message can name the field,
// instead of after a 10-second container round trip. CARE re-checks everything
// (and owns the rules we don't duplicate: uniqueness, valid facility types).
func (s ClinicSeed) validate() error {
	if strings.TrimSpace(s.GeoOrganization) == "" {
		return errors.New("Region name is required.")
	}
	if strings.TrimSpace(s.Facility.Name) == "" {
		return errors.New("Facility name is required.")
	}
	for i, m := range s.Members {
		if strings.TrimSpace(m.Username) == "" || strings.TrimSpace(m.Role) == "" {
			return fmt.Errorf("Member %d needs a username and a role.", i+1)
		}
	}
	return nil
}

// runSeedScript pipes the request to clinic_seed.py as JSON on stdin and parses
// its marker line. Stdin rather than an argument or a temp file: the payload
// holds staff passwords, so it never touches disk or a process list, and there's
// no shell quoting to get wrong on Windows.
func (e *Engine) runSeedScript(request any) (*SeedResult, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	cmd := newCmd("docker", "compose", "exec", "-T", "backend",
		"python", "manage.py", "load_fixtures", "--path", seedScript)
	cmd.Dir = e.workdir()
	cmd.Env = e.baseEnv()
	cmd.Stdin = bytes.NewReader(body)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start docker compose exec: %w", err)
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		marker string
		tail   []string
	)
	scan := func(r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			mu.Lock()
			if rest, ok := strings.CutPrefix(line, resultMarker+" "); ok {
				marker = rest
			} else if strings.TrimSpace(line) != "" {
				// Keep the last few lines so a crash (a traceback, a missing
				// container) is reportable instead of "exit status 1".
				tail = append(tail, line)
				if len(tail) > 8 {
					tail = tail[1:]
				}
			}
			mu.Unlock()
		}
	}
	wg.Add(2)
	go scan(stdout)
	go scan(stderr)
	wg.Wait()
	runErr := cmd.Wait()

	res, parseErr := parseSeedResult(marker)
	switch {
	// The script reports its own failures through the marker, so prefer that
	// message over the exit status it also sets.
	case res != nil && res.Error != "":
		return nil, errors.New(res.Error)
	case runErr != nil:
		return nil, fmt.Errorf("clinic setup failed: %s", strings.Join(tail, " | "))
	case parseErr != nil:
		return nil, parseErr
	}
	return res, nil
}

func parseSeedResult(marker string) (*SeedResult, error) {
	if marker == "" {
		return nil, errors.New("the backend returned no result - is CARE running?")
	}
	var res SeedResult
	if err := json.Unmarshal([]byte(marker), &res); err != nil {
		return nil, fmt.Errorf("could not read the backend's reply: %w", err)
	}
	return &res, nil
}
