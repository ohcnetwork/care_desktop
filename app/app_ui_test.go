package main

import "testing"

func TestAffirmativeAcceptsTheWindowsAnswer(t *testing.T) {
	for _, tc := range []struct {
		sel, yes string
		want     bool
	}{
		{"Yes", "Remove everything", true},
		{"Remove everything", "Remove everything", true},
		{"No", "Remove everything", false},
		{"Cancel", "Remove everything", false},
		{"", "Remove everything", false},
		{"Error", "Remove everything", false},
	} {
		if got := affirmative(tc.sel, tc.yes); got != tc.want {
			t.Errorf("affirmative(%q, %q) = %v, want %v", tc.sel, tc.yes, got, tc.want)
		}
	}
}

func TestUnsetQuitChoiceAllowsClosing(t *testing.T) {
	var unset quitChoice
	if unset == quitStayOpen {
		t.Fatal("an unset quit choice keeps the window open, so a dropped answer would wedge it shut")
	}
	if unset != quitLeaveClinicRunning {
		t.Fatalf("unset quit choice = %v, want quitLeaveClinicRunning", unset)
	}
}
