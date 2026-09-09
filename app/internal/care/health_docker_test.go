package care

import (
	"runtime"
	"strings"
	"testing"
)

func TestDockerAdviceIsPlatformSpecific(t *testing.T) {
	missing, stopped := dockerAdvice()
	for _, s := range []string{missing, stopped} {
		if strings.TrimSpace(s) == "" {
			t.Fatal("advice must never be empty - it is the whole point of the check")
		}
	}
	// Windows is the one platform where a specific product is required; the
	// others must not name Docker Desktop as though it were the only option.
	if runtime.GOOS == "windows" {
		if !strings.Contains(missing, "Docker Desktop") {
			t.Fatalf("Windows advice should point at Docker Desktop, got %q", missing)
		}
		return
	}
	if strings.HasPrefix(missing, "Docker Desktop is not installed") {
		t.Fatalf("%s advice treats Docker Desktop as mandatory: %q", runtime.GOOS, missing)
	}
}

func TestWrongContainerOS(t *testing.T) {
	if got := wrongContainerOS("linux"); got != "" {
		t.Fatalf("linux containers are what we want, got %q", got)
	}
	// An engine too old to report Server.Os must not fail the check.
	if got := wrongContainerOS(""); got != "" {
		t.Fatalf("unknown server OS should pass, got %q", got)
	}
	got := wrongContainerOS("windows")
	if !strings.Contains(got, "Switch to Linux containers") {
		t.Fatalf("want the actionable fix, got %q", got)
	}
}

func TestComposeAdviceMentionsAFix(t *testing.T) {
	got := composeAdvice()
	if !strings.Contains(got, "Compose") {
		t.Fatalf("compose advice should name Compose, got %q", got)
	}
	if runtime.GOOS == "linux" && !strings.Contains(got, "docker-compose-plugin") {
		t.Fatalf("linux advice should name the package, got %q", got)
	}
}
