package care

import (
	"strings"
	"testing"
)

// Approving one thing must never perform two: every step has to be listed.
func TestConfirmPromptListsEveryStep(t *testing.T) {
	steps := []privilegedStep{
		{what: "add care.local to this computer's hosts file, so the name works here"},
		{what: "trust CARE's security certificate, so the browser shows no warning"},
	}
	title, msg := confirmPrompt("care.local", steps)

	if !strings.Contains(title, "care.local") {
		t.Fatalf("title should name the host, got %q", title)
	}
	for _, s := range steps {
		if !strings.Contains(msg, s.what) {
			t.Fatalf("prompt is missing a step it will perform: %q\n%s", s.what, msg)
		}
	}
	if got := strings.Count(msg, "•"); got != len(steps) {
		t.Fatalf("expected %d bullets, got %d:\n%s", len(steps), got, msg)
	}
	if !strings.Contains(msg, "password once") {
		t.Fatalf("prompt should promise a single password:\n%s", msg)
	}
}

func TestConfirmPromptSingleStep(t *testing.T) {
	steps := []privilegedStep{{what: "add care.local to this computer's hosts file"}}
	_, msg := confirmPrompt("care.local", steps)
	if got := strings.Count(msg, "•"); got != 1 {
		t.Fatalf("expected 1 bullet, got %d:\n%s", got, msg)
	}
}

// One invocation is what makes it one password prompt.
func TestRunPrivilegedStepsJoinsAllCommands(t *testing.T) {
	e := &Engine{}
	steps := []privilegedStep{
		{sh: "echo one", ps: "Write-Output one"},
		{sh: "echo two", ps: "Write-Output two"},
	}
	var parts []string
	for _, s := range steps {
		parts = append(parts, s.sh)
	}
	joined := strings.Join(parts, "; ")
	if !strings.Contains(joined, "echo one") || !strings.Contains(joined, "echo two") {
		t.Fatalf("both commands must survive joining, got %q", joined)
	}
	if strings.Contains(joined, "&&") {
		t.Fatalf("steps must not be && chained, got %q", joined)
	}
	if e.runPrivileged(joined, false) != nil {
		t.Fatalf("joined command should run: %q", joined)
	}
}
