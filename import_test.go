package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalImportPreservesAnyArchitectureAcrossGenerations(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	os.Mkdir(input, 0755)
	filename := "local-widget-1.0-1-any.pkg.tar.zst"
	archive := filepath.Join(input, filename)
	os.WriteFile(archive, []byte("package content"), 0644)
	u := Updater{Config: Config{StateDir: filepath.Join(root, "state"), RepoDir: filepath.Join(root, "repo"), RepoName: "nier", SigningKey: "test"}}
	failSign := false
	u.Command = func(_ context.Context, name string, args ...string) ([]byte, error) {
		switch name {
		case "bsdtar":
			return []byte("pkgname = local-widget\npkgver = 1.0-1\narch = any\n"), nil
		case "gpg":
			if strings.Contains(strings.Join(args, " "), "--export") {
				return []byte("public key"), nil
			}
			if failSign {
				return nil, fmt.Errorf("signing failure")
			}
			return nil, os.WriteFile(args[len(args)-1]+".sig", []byte("signature"), 0644)
		case "repo-add":
			db := args[3]
			found := false
			for _, p := range args[4:] {
				if filepath.Base(p) == filename {
					found = true
				}
				if _, err := os.Stat(p); err != nil {
					return nil, err
				}
			}
			if !found {
				t.Fatal("local any package lost from publication")
			}
			os.WriteFile(db, []byte("db"), 0644)
			return nil, os.WriteFile(db+".sig", []byte("signature"), 0644)
		}
		return nil, fmt.Errorf("unexpected command: %s", name)
	}
	if err := u.importPackages(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	original, _ := os.Readlink(filepath.Join(u.Config.RepoDir, "current"))
	if err := u.importPackages(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	repeat, _ := os.Readlink(filepath.Join(u.Config.RepoDir, "current"))
	if original != repeat {
		t.Fatal("same-package import was not a no-op")
	}
	state, err := u.state()
	if err != nil {
		t.Fatal(err)
	}
	if state["local-widget"].Filename != filename {
		t.Fatal("lost arch=any filename")
	}
	if err := u.publish(context.Background(), state, nil); err != nil {
		t.Fatal(err)
	}
	current, _ := os.Readlink(filepath.Join(u.Config.RepoDir, "current"))
	os.WriteFile(archive, []byte("changed same version"), 0644)
	if err := u.importPackages(context.Background(), input); err == nil {
		t.Fatal("accepted changed same-version archive")
	}
	after, _ := os.Readlink(filepath.Join(u.Config.RepoDir, "current"))
	if current != after {
		t.Fatal("failed import changed repository")
	}
	// Fresh repository: a signing failure must not create a current generation.
	u.Config.RepoDir = filepath.Join(root, "failed-repo")
	failSign = true
	if err := u.importPackages(context.Background(), input); err == nil {
		t.Fatal("signing failure ignored")
	}
	if _, err := os.Lstat(filepath.Join(u.Config.RepoDir, "current")); !os.IsNotExist(err) {
		t.Fatal("failed signing published a generation")
	}
}

func TestVendorKeyringDoesNotUseDeepPackageWorkDirectory(t *testing.T) {
	work := filepath.Join(t.TempDir(), strings.Repeat("long-work-directory-", 12))
	os.MkdirAll(work, 0755)
	var home string
	u := Updater{Config: Config{VendorKey: "vendor.asc", VendorFingerprint: strings.Repeat("A", 40)}}
	u.Command = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "gpg" {
			home = args[1]
			if strings.HasPrefix(home, work) {
				t.Fatal("GnuPG keyring remains under deep package work path")
			}
			if _, err := os.Stat(home); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(args, " "), "--no-autostart") {
				t.Fatal("public key inspection starts an agent")
			}
			if strings.Contains(strings.Join(args, " "), "--fingerprint") {
				return []byte("pub::::::::::\nfpr:::::::::" + u.Config.VendorFingerprint + ":\n"), nil
			}
		}
		return nil, nil
	}
	if err := u.verifyVendor(context.Background(), work, "vendor.rpm"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("temporary public keyring not removed")
	}
}
