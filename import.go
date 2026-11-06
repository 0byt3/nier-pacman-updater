package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
)

// Local packages share the updater lock and atomic generation publisher. Their
// state entries survive future upstream-only updates, including arch=any files.
func (u *Updater) importPackages(ctx context.Context, directory string) error {
	if err := os.MkdirAll(u.Config.StateDir, 0755); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(u.Config.StateDir, "update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("another updater holds the lock: %w", err)
	}
	state, err := u.state()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(directory, "*.pkg.tar.zst"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no package archives to import")
	}
	work, err := os.MkdirTemp(u.Config.StateDir, "local-import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	var packages []string
	seen := map[string]bool{}
	for _, file := range files {
		info, err := u.Command(ctx, "bsdtar", "-xOf", file, ".PKGINFO")
		if err != nil {
			return err
		}
		metadata := map[string]string{}
		for _, line := range strings.Split(string(info), "\n") {
			if k, v, ok := strings.Cut(line, " = "); ok {
				metadata[k] = v
			}
		}
		name, version, arch := metadata["pkgname"], metadata["pkgver"], metadata["arch"]
		if !regexp.MustCompile(`^[a-z0-9][a-z0-9@._+\-]*$`).MatchString(name) || !regexp.MustCompile(`^[A-Za-z0-9_.+:~\-]+$`).MatchString(version) || (arch != "any" && arch != "x86_64") {
			return fmt.Errorf("invalid package metadata: %s", file)
		}
		if slices.Contains(u.Config.Packages, name) {
			return fmt.Errorf("%s is upstream-managed; use the normal updater", name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate package: %s", name)
		}
		seen[name] = true
		split := strings.LastIndex(version, "-")
		if split < 1 || split == len(version)-1 {
			return errors.New("package version has no release")
		}
		filename := name + "-" + version + "-" + arch + ".pkg.tar.zst"
		if filepath.Base(file) != filename {
			return fmt.Errorf("archive filename does not match metadata: %s", file)
		}
		// Copy before hashing/signing so a caller changing its input cannot alter the
		// candidate between validation and publication.
		staged := filepath.Join(work, filename)
		if err := copyFile(file, staged); err != nil {
			return err
		}
		stagedInfo, err := u.Command(ctx, "bsdtar", "-xOf", staged, ".PKGINFO")
		if err != nil {
			return err
		}
		if string(stagedInfo) != string(info) {
			return errors.New("package metadata changed while staging")
		}
		f, err := os.Open(staged)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, err = io.Copy(hash, f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		sum := hex.EncodeToString(hash.Sum(nil))
		previous := state[name]
		if previous.Version != "" && compare(version, previous.fullVersion()) <= 0 {
			if version == previous.fullVersion() && previous.SHA256 == sum {
				continue
			}
			return fmt.Errorf("%s: refusing an older or changed same-version package", name)
		}
		if err := u.run(ctx, "gpg", "--homedir", filepath.Join(u.Config.StateDir, "gnupg"), "--batch", "--yes", "--local-user", u.Config.SigningKey, "--detach-sign", staged); err != nil {
			return err
		}
		state[name] = Release{Name: name, Version: version[:split], Pkgrel: version[split+1:], Filename: filename, SHA256: sum, URL: "local:" + name}
		packages = append(packages, staged)
	}
	if len(packages) == 0 {
		return nil
	}
	return u.publish(ctx, state, packages)
}
