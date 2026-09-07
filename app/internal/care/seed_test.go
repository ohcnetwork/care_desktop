package care

import "testing"

func TestClinicSeedValidate(t *testing.T) {
	ok := ClinicSeed{
		GeoOrganization: "Ernakulam",
		Facility:        SeedFacility{Name: "Town Clinic"},
		Members:         []SeedMember{{Username: "asha", Role: "Nurse"}},
	}
	if err := ok.validate(); err != nil {
		t.Fatalf("valid seed rejected: %v", err)
	}

	bad := []struct {
		name string
		seed ClinicSeed
	}{
		{"no region", ClinicSeed{Facility: SeedFacility{Name: "Town Clinic"}}},
		{"no facility", ClinicSeed{GeoOrganization: "Ernakulam"}},
		{"blank facility", ClinicSeed{GeoOrganization: "Ernakulam", Facility: SeedFacility{Name: "   "}}},
		{"member without role", ClinicSeed{
			GeoOrganization: "Ernakulam",
			Facility:        SeedFacility{Name: "Town Clinic"},
			Members:         []SeedMember{{Username: "asha"}},
		}},
		{"member without username", ClinicSeed{
			GeoOrganization: "Ernakulam",
			Facility:        SeedFacility{Name: "Town Clinic"},
			Members:         []SeedMember{{Role: "Nurse"}},
		}},
	}
	for _, tc := range bad {
		if err := tc.seed.validate(); err == nil {
			t.Errorf("%s: expected an error, got none", tc.name)
		}
	}
}

func TestParseSeedResult(t *testing.T) {
	res, err := parseSeedResult(`{"facility":{"id":"abc","name":"Town Clinic"},"members":[{"username":"asha","role":"Nurse"}]}`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if res.Facility.Name != "Town Clinic" || len(res.Members) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}

	// An empty marker means the script never reported - that must not read as success.
	if _, err := parseSeedResult(""); err == nil {
		t.Error("expected an error for a missing result line")
	}
	if _, err := parseSeedResult("not json"); err == nil {
		t.Error("expected an error for a malformed result line")
	}
}
