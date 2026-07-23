// Command care is the terminal entrypoint to the CARE clinic engine - the Go
// replacement for care.sh. It runs the same engine the desktop app uses.
//
//	care setup | start | stop | restart | rebuild-backend | rebuild-frontend |
//	     status | backup-now | list-backups | restore <dump> [files.tar.gz] |
//	     uninstall [--images] [--backups] --yes | mdns [name]
//
// The kit dir defaults to the current directory (override with CARE_DESKTOP_DIR).
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"care-desktop/app/internal/care"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	kit := os.Getenv("CARE_DESKTOP_DIR")
	if kit == "" {
		kit, _ = os.Getwd()
	}
	care.FixPath()
	e := &care.Engine{
		Kit: kit,
		Log: func(s string) { fmt.Println(s) },
	}

	var err error
	switch os.Args[1] {
	case "setup":
		err = e.Setup()
	case "start":
		err = e.Start()
	case "stop":
		err = e.Stop()
	case "restart":
		err = e.Restart()
	case "rebuild-backend":
		err = e.RebuildBackend()
	case "rebuild-frontend":
		err = e.RebuildFrontend()
	case "status":
		var out string
		if out, err = e.Status(); err == nil {
			fmt.Print(out)
		}
	case "backup-now":
		err = e.BackupNow()
	case "list-backups":
		err = listBackups(e)
	case "restore":
		err = restore(e, os.Args[2:])
	case "uninstall":
		err = uninstall(e, os.Args[2:])
	case "mdns":
		err = mdnsServe(e, os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// listBackups prints the restorable points, newest first.
func listBackups(e *care.Engine) error {
	backups, err := e.ListBackups()
	if err != nil {
		return err
	}
	if len(backups) == 0 {
		fmt.Println("No backups found. Run `care backup-now` or wait for the daily backup.")
		return nil
	}
	fmt.Println("Restore one with: care restore <dump>")
	for _, b := range backups {
		fmt.Printf("  %-32s  %s\n", b.DBDump, b.Label)
	}
	return nil
}

// restore replays a backup. The files archive is optional: if omitted, the
// same-timestamp files-*.tar.gz is paired automatically when one exists.
func restore(e *care.Engine, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: care restore <care-*.dump> [files-*.tar.gz]")
	}
	dump, files := args[0], ""
	if len(args) >= 2 {
		files = args[1]
	} else if backups, err := e.ListBackups(); err == nil {
		for _, b := range backups {
			if b.DBDump == dump {
				files = b.FilesArchive
			}
		}
	}
	// Encrypted backups need the backup password; from the CLI it comes via the
	// CARE_BACKUP_PASSWORD env var (the GUI reads it from the keychain instead).
	return e.Restore(dump, files, os.Getenv("CARE_BACKUP_PASSWORD"))
}

// uninstall tears the install down. It removes the containers and ALL data
// volumes, so it refuses to run without an explicit --yes (there's no prompt).
func uninstall(e *care.Engine, args []string) error {
	opts := care.UninstallOptions{RemoveKit: true} // the source-repo guard protects a dev checkout
	yes := false
	for _, a := range args {
		switch a {
		case "--images":
			opts.RemoveImages = true
		case "--backups":
			opts.RemoveBackups = true
		case "--yes", "-y":
			yes = true
		default:
			return fmt.Errorf("unknown flag %q - usage: care uninstall [--images] [--backups] --yes", a)
		}
	}
	if !yes {
		fmt.Println("This removes CARE's containers and ALL data volumes (patient records + files).")
		fmt.Println("It also removes the installed files and downloaded source.")
		if opts.RemoveImages {
			fmt.Println("  + Docker images")
		}
		if opts.RemoveBackups {
			fmt.Println("  + the backup folder (recovery data - no way back after this)")
		} else {
			fmt.Println("  (backups are kept - pass --backups to remove them too)")
		}
		fmt.Println("\nRe-run with --yes to proceed.")
		return nil
	}
	return e.Uninstall(opts)
}

// mdnsServe advertises <name>.local on the LAN and blocks until interrupted. For
// headless servers, run it under systemd so care.local resolves without renaming
// the host. The desktop app does this on its own while it's open.
func mdnsServe(e *care.Engine, args []string) error {
	name := e.MDNSName()
	if len(args) >= 1 && args[0] != "" {
		name = args[0]
	}
	adv, err := care.Advertise(name)
	if err != nil {
		return err
	}
	defer adv.Stop()
	fmt.Printf("Advertising %s.local on the LAN - press Ctrl-C to stop.\n", adv.Name())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	fmt.Println("\nStopped advertising.")
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: care [setup|start|stop|restart|rebuild-backend|rebuild-frontend|status|backup-now|list-backups|restore <dump> [files.tar.gz]|uninstall [--images] [--backups] --yes|mdns [name]]")
}
