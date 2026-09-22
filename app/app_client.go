package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/trust"
)

func (a *App) ConnectClient(address string) error {
	return a.withJob(func() error {
		cfg := a.loadConfig()
		if cfg.Role != "client" {
			return errors.New("choose Connect to a clinic before connecting this computer")
		}
		clinicURL, err := trust.ClientURL(address)
		if err != nil {
			return err
		}
		if cfg.ClientURL != "" && cfg.ClientURL != clinicURL {
			return errors.New("remove this computer's current clinic access before connecting to another clinic")
		}
		ctx := a.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		networkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		root := cfg.ClientCertificate
		if root == "" {
			root, err = trust.FetchClientCertificate(networkCtx, clinicURL)
			if err != nil {
				return err
			}
		}
		if err := trust.CheckClientConnection(networkCtx, clinicURL, root); err != nil {
			return err
		}
		present, err := trust.ClientCertificatePresent(root)
		if err != nil {
			return err
		}
		cfg.ClientURL = clinicURL
		cfg.ClientCertificate = root
		cfg.ClientCertificateOwned = cfg.ClientCertificateOwned || !present
		// Save ownership before elevation so interrupted setup can be retried or removed.
		if err := a.saveConfig(cfg); err != nil {
			return fmt.Errorf("could not save the clinic connection; no certificate was installed: %w", err)
		}
		if !present {
			if err := trust.InstallClientCertificate(root); err != nil {
				return err
			}
		}
		// The OS prompt can take longer than the download timeout.
		if err := trust.CheckClientConnection(ctx, clinicURL, ""); err != nil {
			return err
		}
		a.logln("This client can open " + clinicURL)
		a.OpenURL(clinicURL)
		return nil
	})
}

func (a *App) DisconnectClient() error {
	return a.withJob(func() error {
		cfg := a.loadConfig()
		if cfg.Role != "client" {
			return errors.New("clinic access removal is only available on a client computer")
		}
		if cfg.ClientCertificateOwned {
			if err := trust.RemoveClientCertificate(cfg.ClientCertificate); err != nil {
				return err
			}
		}
		if err := a.resetConfigAfterUninstall(); err != nil {
			return fmt.Errorf("could not clear the saved clinic connection; try removing clinic access again: %w", err)
		}
		a.logln("Removed this client's saved clinic access and role. No clinic data was changed.")
		return nil
	})
}
