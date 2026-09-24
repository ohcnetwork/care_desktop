package prereq

import "testing"

func TestStripRancherBlock(t *testing.T) {
	block := rancherRCStart + "\nexport PATH=\"$HOME/.rd/bin:$PATH\"\n" + rancherRCEnd + "\n"
	cases := []struct {
		name, in, want string
		changed        bool
	}{
		{"removes only the managed block", "alias ll='ls -l'\n" + block + "export EDITOR=vim\n", "alias ll='ls -l'\nexport EDITOR=vim\n", true},
		{"no block leaves file alone", "export EDITOR=vim\n", "export EDITOR=vim\n", false},
		{"unterminated block is kept", "a\n" + rancherRCStart + "\nb\n", "a\n" + rancherRCStart + "\nb\n", false},
		{"block without trailing newline", "a\n" + rancherRCStart + "\nx\n" + rancherRCEnd, "a\n", true},
	}
	for _, c := range cases {
		got, changed := stripRancherBlock(c.in)
		if got != c.want || changed != c.changed {
			t.Errorf("%s: got %q (%v), want %q (%v)", c.name, got, changed, c.want, c.changed)
		}
	}
}
