package compose

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ohcnetwork/care_desktop/app/internal/release"
)

func TestImageKeyFollowsTheBranchHead(t *testing.T) {
	b := builderFixture(t)
	repo := filepath.Join(t.TempDir(), "remote")
	run := gitFixture(t, repo)
	b.set.BeRepo = repo

	first := commitFixture(t, run, "first")
	before := builderAt(t, b).backendBuiltFrom("")
	second := commitFixture(t, run, "second")
	after := builderAt(t, b).backendBuiltFrom("")

	if first == second {
		t.Fatal("fixture did not move the branch")
	}
	if before == after {
		t.Fatal("the image key ignored a new commit on the tracked branch - the clinic would never update")
	}
	for ref, key := range map[string]string{first: before, second: after} {
		if !contains(key, ref) {
			t.Errorf("image key %q does not name the commit %s it was built from", key, ref)
		}
	}
}

func TestStartupUsesTheRecordedCommitAndBackgroundChecksLookAhead(t *testing.T) {
	b := builderFixture(t)
	repo := filepath.Join(t.TempDir(), "remote")
	run := gitFixture(t, repo)
	b.set.BeRepo = repo
	installed := commitFixture(t, run, "installed")
	if err := WriteLock(b.dir, Lock{Backend: Channel{Ref: "develop", Current: installed}}); err != nil {
		t.Fatal(err)
	}
	upstream := commitFixture(t, run, "upstream")

	if got := builderAt(t, b).BackendRef(); got != installed {
		t.Errorf("startup resolved %s, want the installed commit %s", got, installed)
	}
	if got := builderAt(t, b).Pending().BackendRef(); got != upstream {
		t.Errorf("the background check resolved %s, want the branch head %s", got, upstream)
	}
}

func TestUnreachableRemoteKeepsTheInstalledCommit(t *testing.T) {
	b := builderFixture(t)
	installed := "a749b92794ac175db8839d3d75ca36402a196282"
	if err := WriteLock(b.dir, Lock{Backend: Channel{Ref: "develop", Current: installed}}); err != nil {
		t.Fatal(err)
	}
	b.set.BeRepo = filepath.Join(b.dir, "no-such-repository")
	if got := builderAt(t, b).Pending().BackendRef(); got != installed {
		t.Errorf("offline check resolved %q, want the installed commit", got)
	}
}

func TestPinnedRefIsUsedVerbatim(t *testing.T) {
	b := builderFixture(t)
	pinned := "a749b92794ac175db8839d3d75ca36402a196282"
	b.set.BeRef = pinned
	b.set.BeRepo = filepath.Join(b.dir, "no-such-repository")
	if got := b.BackendRef(); got != pinned {
		t.Errorf("pinned ref resolved to %q, want %q", got, pinned)
	}
	if !release.IsCommitRef(b.Pending().BackendRef()) {
		t.Error("a pinned ref was not honoured by the background check")
	}
}

func TestLockSurvivesRoundTripAndCorruption(t *testing.T) {
	dir := t.TempDir()
	want := Lock{
		Backend:  Channel{Ref: "develop", Current: "aaa", Next: "bbb", Declined: "bbb"},
		Frontend: Channel{Ref: "develop", Current: "ccc"},
	}
	if err := WriteLock(dir, want); err != nil {
		t.Fatal(err)
	}
	if got := ReadLock(dir); got != want {
		t.Errorf("lock round-trip = %+v, want %+v", got, want)
	}
	if err := os.WriteFile(filepath.Join(dir, LockFile), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := ReadLock(dir); got != (Lock{}) {
		t.Errorf("a corrupt lock was trusted: %+v", got)
	}
	if got := ReadLock(filepath.Join(dir, "absent")); got != (Lock{}) {
		t.Errorf("a missing lock was not empty: %+v", got)
	}
}

func builderAt(t *testing.T, b *Builder) *Builder {
	t.Helper()
	return NewBuilder(b.run, b.dir, b.set, nil)
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		(haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
