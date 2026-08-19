package main

import "testing"

// joinSuffixOnce must append the suffix once, and not double it when base
// already ends with it - the exact bug that produced a nested
// ".../care-db-backups/care-db-backups" path on a setup retry.
func TestJoinSuffixOnce(t *testing.T) {
	cases := []struct {
		base, suffix, want string
	}{
		{"/Users/mathew/Desktop/care_fe", "care-db-backups", "/Users/mathew/Desktop/care_fe/care-db-backups"},
		{"/Users/mathew/Desktop/care_fe/care-db-backups", "care-db-backups", "/Users/mathew/Desktop/care_fe/care-db-backups"},
		{"/Users/mathew/Desktop/care_fe/care-db-backups/", "care-db-backups", "/Users/mathew/Desktop/care_fe/care-db-backups"},
		{"/Volumes/Data", "CARE Desktop", "/Volumes/Data/CARE Desktop"},
		{"/Volumes/Data/CARE Desktop", "CARE Desktop", "/Volumes/Data/CARE Desktop"},
	}
	for _, c := range cases {
		if got := joinSuffixOnce(c.base, c.suffix); got != c.want {
			t.Errorf("joinSuffixOnce(%q, %q) = %q, want %q", c.base, c.suffix, got, c.want)
		}
	}
}
