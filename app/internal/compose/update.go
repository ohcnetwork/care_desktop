package compose

import (
	"github.com/ohcnetwork/care_desktop/app/internal/release"
)

type Update struct {
	Backend  string `json:"backend"`
	Frontend string `json:"frontend"`
}

func (u Update) Any() bool { return u.Backend != "" || u.Frontend != "" }

func Short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func (b *Builder) PrepareUpdate() (Update, error) {
	if b.stopped() {
		return Update{}, nil
	}
	next := b.Pending()
	lock := ReadLock(b.dir)
	var u Update
	if !release.IsCommitRef(b.set.BeRef) && wanted(lock.Backend, next.BackendRef()) {
		u.Backend = next.BackendRef()
	}
	if !release.IsCommitRef(b.set.FeRef) && wanted(lock.Frontend, next.FrontendRef()) {
		u.Frontend = next.FrontendRef()
	}
	if !u.Any() {
		return u, nil
	}
	if u.Frontend != "" {
		if next.stopped() {
			return Update{}, nil
		}
		b.logln("A newer CARE frontend is available (" + Short(u.Frontend) + ") - building it in the background.")
		if err := next.EnsureFrontendImage(); err != nil {
			return Update{}, err
		}
		if next.stopped() {
			return Update{}, nil
		}
		if err := next.recordBuilt(Frontend, b.set.FeRef, u.Frontend); err != nil {
			return Update{}, err
		}
	}
	if u.Backend != "" {
		if next.stopped() {
			return Update{}, nil
		}
		b.logln("A newer CARE backend is available (" + Short(u.Backend) + ") - building it in the background.")
		if err := next.EnsureBackendImage(); err != nil {
			return Update{}, err
		}
		if next.stopped() {
			return Update{}, nil
		}
		if err := next.recordBuilt(Backend, b.set.BeRef, u.Backend); err != nil {
			return Update{}, err
		}
	}
	return u, nil
}

func wanted(c Channel, sha string) bool {
	return sha != "" && sha != c.Current && sha != c.Declined
}

func (b *Builder) Waiting() Update {
	lock := ReadLock(b.dir)
	return Update{Backend: lock.Backend.Next, Frontend: lock.Frontend.Next}
}

func (b *Builder) ApplyPending() (Update, error) {
	var applied Update
	err := ModifyLock(b.dir, func(lock *Lock) error {
		return b.applyPending(lock, &applied)
	})
	return applied, err
}

func (b *Builder) applyPending(lock *Lock, applied *Update) error {
	for _, s := range []struct {
		service string
		image   string
		dst     *string
	}{
		{Backend, b.set.BackendImage, &applied.Backend},
		{Frontend, b.set.FrontendImage, &applied.Frontend},
	} {
		c := lock.Get(s.service)
		if c.Next == "" {
			continue
		}
		staged := s.image + "-next"
		_, ok, err := b.builtFrom(staged)
		if err != nil {
			return err
		}
		if !ok {
			c.Next, c.Declined = "", ""
			lock.Set(s.service, c)
			continue
		}
		if err := b.run.Run("docker", "image", "tag", staged, s.image); err != nil {
			return err
		}
		b.logln("CARE " + s.service + " updated to " + Short(c.Next) + ".")
		*s.dst = c.Next
		c.Current, c.Next, c.Declined = c.Next, "", ""
		lock.Set(s.service, c)
	}
	return nil
}

func (b *Builder) DeclineUpdate() error {
	return ModifyLock(b.dir, func(lock *Lock) error {
		for _, service := range []string{Backend, Frontend} {
			c := lock.Get(service)
			if c.Next != "" {
				c.Declined = c.Next
			}
			lock.Set(service, c)
		}
		return nil
	})
}

func (b *Builder) PruneDangling() { b.pruneDangling() }
