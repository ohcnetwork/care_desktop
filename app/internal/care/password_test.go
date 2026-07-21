package care

import "testing"

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name    string
		pw      string
		wantErr bool
	}{
		{"empty", "", true},
		{"too short", "Ab3xy", true},
		{"too long", "Abcdefghij0123456789X", true}, // 21 chars
		{"no uppercase", "abcdef12", true},
		{"no lowercase", "ABCDEF12", true},
		{"no number", "Abcdefgh", true},
		{"ok min", "Abcdef12", false},            // 8 chars, has all three
		{"ok max", "Abcdefghij012345678", false}, // 19 chars
		{"ok exactly 20", "Abcdefghij0123456789", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePassword(c.pw)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidatePassword(%q) err=%v, wantErr=%v", c.pw, err, c.wantErr)
			}
		})
	}
}
