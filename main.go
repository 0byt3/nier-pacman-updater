package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"text/template"
	"time"
)

type Config struct {
	StateDir          string   `json:"state_dir"`
	RepoDir           string   `json:"repo_dir"`
	RepoName          string   `json:"repo_name"`
	RecipeDir         string   `json:"recipe_dir"`
	Image             string   `json:"image"`
	SigningKey        string   `json:"signing_key"`
	VendorKey         string   `json:"vendor_key"`
	VendorFingerprint string   `json:"vendor_fingerprint"`
	Packages          []string `json:"packages"`
	RecipeRevision    int      `json:"recipe_revision"`
}
type Release struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Pkgrel   string `json:"pkgrel"`
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	Filename string `json:"filename,omitempty"`
}

func (r Release) fullVersion() string { return r.Version + "-" + r.Pkgrel }

type State map[string]Release
type commandFunc func(context.Context, string, ...string) ([]byte, error)
type Updater struct {
	Config                   Config
	Client                   *http.Client
	GithubAPI, CloudsmithAPI string
	Command                  commandFunc
}

var stable = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*$`)
var digest = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

func command(ctx context.Context, name string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, name, args...)
	c.Stderr = os.Stderr
	out, err := c.Output()
	if err != nil {
		return out, fmt.Errorf("%s failed: %w", name, err)
	}
	return out, nil
}
func (u *Updater) run(ctx context.Context, name string, args ...string) error {
	out, err := u.Command(ctx, name, args...)
	if len(out) > 0 {
		log.Print(string(out))
	}
	return err
}
func (u *Updater) get(ctx context.Context, address string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "nier-pacman-updater/1")
	if req.URL.Host == "api.github.com" && os.Getenv("GITHUB_TOKEN") != "" {
		req.Header.Set("Authorization", "Bearer "+os.Getenv("GITHUB_TOKEN"))
	}
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: HTTP %d", address, resp.StatusCode)
	}
	return resp, nil
}
func (u *Updater) jsonGet(ctx context.Context, address string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	resp, err := u.get(ctx, address)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(v)
}
func vendorURL(s string) bool {
	p, err := url.Parse(s)
	return err == nil && p.Scheme == "https" && p.Host == "dl.cloudsmith.io" && p.User == nil && strings.HasPrefix(p.Path, "/public/devolutions/rdm/")
}

// Versions accepted by the recipes are numeric stable releases only.
func comparePart(a, b string) int {
	aa, bb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < min(len(aa), len(bb)); i++ {
		x, y := strings.TrimLeft(aa[i], "0"), strings.TrimLeft(bb[i], "0")
		if len(x) != len(y) {
			if len(x) > len(y) {
				return 1
			}
			return -1
		}
		if c := strings.Compare(x, y); c != 0 {
			return c
		}
	}
	if len(aa) > len(bb) {
		return 1
	}
	if len(aa) < len(bb) {
		return -1
	}
	return 0
}
func compare(a, b string) int {
	av, ar, _ := strings.Cut(a, "-")
	bv, br, _ := strings.Cut(b, "-")
	if c := comparePart(av, bv); c != 0 {
		return c
	}
	return comparePart(ar, br)
}

func (u *Updater) discover(ctx context.Context, name string) (Release, error) {
	r := Release{Name: name, Pkgrel: fmt.Sprint(u.Config.RecipeRevision)}
	switch name {
	case "onedrive-abraunegg", "onedrive-quickshell", "nier-monitor-arrangement", "teams-for-linux", "powershell":
		repository := "abraunegg/onedrive"
		if name == "onedrive-quickshell" {
			repository = "0byt3/onedrive-quickshell"
		} else if name == "nier-monitor-arrangement" {
			repository = "0byt3/nier-monitor-arrangement"
		} else if name == "teams-for-linux" {
			repository = "IsmaelMartinez/teams-for-linux"
		} else if name == "powershell" {
			repository = "PowerShell/PowerShell"
		}
		var v struct {
			Tag               string `json:"tag_name"`
			Draft, Prerelease bool
			Assets            []struct {
				Name   string `json:"name"`
				URL    string `json:"browser_download_url"`
				Digest string `json:"digest"`
				Size   int64  `json:"size"`
			} `json:"assets"`
		}
		if err := u.jsonGet(ctx, u.GithubAPI+"/repos/"+repository+"/releases/latest", &v); err != nil {
			return r, err
		}
		r.Version = strings.TrimPrefix(v.Tag, "v")
		if v.Draft || v.Prerelease || !stable.MatchString(r.Version) {
			return r, fmt.Errorf("not a numeric stable %s release: %q", name, v.Tag)
		}
		if name == "teams-for-linux" || name == "powershell" {
			filename := "teams-for-linux-" + r.Version + ".x86_64.rpm"
			if name == "powershell" {
				filename = "powershell-" + r.Version + "-linux-x64.tar.gz"
			}
			matches := 0
			for _, asset := range v.Assets {
				if asset.Name != filename {
					continue
				}
				matches++
				sum, ok := strings.CutPrefix(asset.Digest, "sha256:")
				if !ok || !digest.MatchString(sum) || asset.Size <= 0 || asset.Size > 2<<30 || asset.URL != "https://github.com/"+repository+"/releases/download/"+v.Tag+"/"+filename {
					return r, fmt.Errorf("invalid %s x86_64 asset", name)
				}
				r.URL, r.SHA256 = asset.URL, strings.ToLower(sum)
			}
			if matches != 1 {
				return r, fmt.Errorf("expected one %s x86_64 asset, got %d", name, matches)
			}
			break
		}
		r.URL = "https://github.com/" + repository + "/archive/refs/tags/" + v.Tag + ".tar.gz"
	case "neovim-nightly":
		var v struct {
			Tag    string `json:"tag_name"`
			Draft  bool
			Assets []struct {
				Name    string    `json:"name"`
				URL     string    `json:"browser_download_url"`
				Digest  string    `json:"digest"`
				Updated time.Time `json:"updated_at"`
				ID      int64     `json:"id"`
				Size    int64     `json:"size"`
			} `json:"assets"`
		}
		if err := u.jsonGet(ctx, u.GithubAPI+"/repos/neovim/neovim/releases/tags/nightly", &v); err != nil {
			return r, err
		}
		if v.Tag != "nightly" || v.Draft {
			return r, errors.New("invalid Neovim nightly release")
		}
		matches := 0
		for _, a := range v.Assets {
			if a.Name != "nvim-linux-x86_64.tar.gz" {
				continue
			}
			matches++
			sum, ok := strings.CutPrefix(a.Digest, "sha256:")
			if !ok || !digest.MatchString(sum) || a.ID <= 0 || a.Updated.IsZero() || a.Size <= 0 || a.Size > 2<<30 ||
				a.URL != "https://github.com/neovim/neovim/releases/download/nightly/nvim-linux-x86_64.tar.gz" {
				return r, errors.New("invalid Neovim nightly x86_64 asset")
			}
			r.Version = a.Updated.UTC().Format("20060102150405") + "." + fmt.Sprint(a.ID)
			r.URL, r.SHA256 = a.URL, strings.ToLower(sum)
		}
		if matches != 1 {
			return r, fmt.Errorf("expected one Neovim nightly x86_64 asset, got %d", matches)
		}
	case "remote-desktop-manager":
		type pkg struct {
			Name, Version, Format string
			Status                string                        `json:"status_str"`
			Downloadable          bool                          `json:"is_downloadable"`
			Hidden                bool                          `json:"is_hidden"`
			Quarantined           bool                          `json:"is_quarantined"`
			URL                   string                        `json:"cdn_url"`
			SHA256                string                        `json:"checksum_sha256"`
			Signature             string                        `json:"signature_url"`
			Identifiers           struct{ Architecture string } `json:"identifiers"`
		}
		// Paginate rather than assuming the most recently uploaded file is newest.
		for page := 1; page <= 20; page++ {
			var pkgs []pkg
			address := fmt.Sprintf("%s/v1/packages/devolutions/rdm/?page_size=100&page=%d&query=%s", u.CloudsmithAPI, page, url.QueryEscape("format:rpm architecture:x86_64 name:RemoteDesktopManager"))
			if err := u.jsonGet(ctx, address, &pkgs); err != nil {
				return r, err
			}
			for _, p := range pkgs {
				v, rel, ok := strings.Cut(p.Version, "-")
				if !ok || !stable.MatchString(v) || !stable.MatchString(rel) || p.Name != "RemoteDesktopManager" || p.Format != "rpm" || p.Identifiers.Architecture != "x86_64" || p.Status != "Completed" || !p.Downloadable || p.Hidden || p.Quarantined {
					continue
				}
				if !vendorURL(p.URL) || !digest.MatchString(p.SHA256) {
					return r, errors.New("invalid Cloudsmith artifact URL or checksum")
				}
				candidate := Release{Name: name, Version: v, Pkgrel: rel + "." + fmt.Sprint(u.Config.RecipeRevision), URL: p.URL, SHA256: strings.ToLower(p.SHA256)}
				if r.Version == "" || compare(candidate.fullVersion(), r.fullVersion()) > 0 {
					r = candidate
				}
			}
			if len(pkgs) < 100 {
				break
			}
			if page == 20 {
				return r, errors.New("Cloudsmith pagination limit exceeded; refusing incomplete selection")
			}
		}
		if r.Version == "" {
			return r, errors.New("no completed stable x86_64 RemoteDesktopManager RPM found")
		}
	default:
		return r, fmt.Errorf("unsupported package %q", name)
	}
	return r, nil
}
func (u *Updater) download(ctx context.Context, address, path, expected string) (string, error) {
	resp, err := u.get(ctx, address)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, (2<<30)+1))
	closeErr := f.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if n > 2<<30 {
		return "", errors.New("download exceeds 2 GiB limit")
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if expected != "" && !strings.EqualFold(sum, expected) {
		return "", errors.New("download checksum mismatch")
	}
	return sum, nil
}
func (u *Updater) verifyVendor(ctx context.Context, work, artifact string) error {
	// GnuPG constructs agent socket names even with --no-autostart. Keep the
	// temporary public keyring outside potentially long package work paths.
	home, err := os.MkdirTemp("", "nier-vendor-keyring-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(home)
	// Public-key inspection needs no agent. Avoid spawning one with a socket
	// path exceeding Unix's limit in deeply nested staging directories.
	if err := u.run(ctx, "gpg", "--homedir", home, "--no-autostart", "--batch", "--import", u.Config.VendorKey); err != nil {
		return err
	}
	out, err := u.Command(ctx, "gpg", "--homedir", home, "--no-autostart", "--batch", "--with-colons", "--fingerprint")
	if err != nil {
		return err
	}
	var fingerprints []string
	primaryKeys := 0
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Split(line, ":")
		if len(f) > 0 && f[0] == "pub" {
			primaryKeys++
		}
		if len(f) > 9 && f[0] == "fpr" {
			fingerprints = append(fingerprints, f[9])
		}
	}
	if primaryKeys != 1 || len(fingerprints) == 0 || fingerprints[0] != u.Config.VendorFingerprint {
		return errors.New("vendor signing key fingerprint mismatch")
	}
	rpmdb := filepath.Join(work, "rpmdb")
	if err := os.Mkdir(rpmdb, 0700); err != nil {
		return err
	}
	if err := u.run(ctx, "rpmkeys", "--dbpath", rpmdb, "--import", u.Config.VendorKey); err != nil {
		return err
	}
	return u.run(ctx, "rpmkeys", "--dbpath", rpmdb, "--define", "_pkgverify_level all", "--checksig", "--verbose", artifact)

}
func (u *Updater) build(ctx context.Context, r Release) (Release, string, error) {
	work, err := os.MkdirTemp(filepath.Join(u.Config.StateDir, "work"), r.Name+"-")
	if err != nil {
		return r, "", err
	}
	// Keep failed work and logs for diagnosis; remove successful work after publication.
	filename := "source.tar.gz"
	if r.Name == "remote-desktop-manager" || r.Name == "teams-for-linux" {
		filename = "vendor.rpm"
	}
	artifact := filepath.Join(work, filename)
	r.SHA256, err = u.download(ctx, r.URL, artifact, r.SHA256)
	if err != nil {
		return r, "", err
	}
	if r.Name == "remote-desktop-manager" {
		if err = u.verifyVendor(ctx, work, artifact); err != nil {
			return r, "", err
		}
	}

	// Pacman install scripts must exist beside PKGBUILD before makepkg starts.
	installName := r.Name + ".install"
	installScript, err := os.ReadFile(filepath.Join(u.Config.RecipeDir, installName))
	if err == nil {
		if err = os.WriteFile(filepath.Join(work, installName), installScript, 0644); err != nil {
			return r, "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return r, "", err
	}

	tpl, err := template.ParseFiles(filepath.Join(u.Config.RecipeDir, r.Name+".PKGBUILD"))
	if err != nil {
		return r, "", err
	}
	f, err := os.Create(filepath.Join(work, "PKGBUILD"))
	if err != nil {
		return r, "", err
	}
	err = tpl.Execute(f, r)
	closeErr := f.Close()
	if err != nil {
		return r, "", err
	}
	if closeErr != nil {
		return r, "", closeErr
	}
	if err = u.run(ctx, "podman", "--cgroup-manager=cgroupfs", "run", "--rm", "--cgroups=disabled", "--userns=keep-id:uid=1000,gid=1000", "--network=none", "--volume", work+":/work:rw", "--workdir", "/work", u.Config.Image, "makepkg", "--nodeps", "--cleanbuild", "--clean", "--noconfirm"); err != nil {
		return r, "", err
	}
	files, err := filepath.Glob(filepath.Join(work, "*.pkg.tar.zst"))
	if err != nil || len(files) != 1 {
		return r, "", fmt.Errorf("expected one package, got %d", len(files))
	}
	info, err := u.Command(ctx, "bsdtar", "-xOf", files[0], ".PKGINFO")
	if err != nil {
		return r, "", err
	}
	metadata := map[string]string{}
	for _, line := range strings.Split(string(info), "\n") {
		k, v, ok := strings.Cut(line, " = ")
		if ok {
			metadata[k] = v
		}
	}
	if metadata["pkgname"] != r.Name || metadata["pkgver"] != r.fullVersion() || metadata["arch"] != "x86_64" {
		return r, "", errors.New("built package metadata does not match requested release")
	}
	if err = u.run(ctx, "gpg", "--homedir", filepath.Join(u.Config.StateDir, "gnupg"), "--batch", "--yes", "--local-user", u.Config.SigningKey, "--detach-sign", files[0]); err != nil {
		return r, "", err
	}
	return r, files[0], nil
}
func (u *Updater) state() (State, error) {
	data, err := os.ReadFile(filepath.Join(u.Config.RepoDir, "current", "state.json"))
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	err = json.Unmarshal(data, &s)
	if s == nil && err == nil {
		err = errors.New("invalid null repository state")
	}
	return s, err
}
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func (u *Updater) publish(ctx context.Context, state State, packages []string) error {
	releases := filepath.Join(u.Config.RepoDir, "releases")
	if err := os.MkdirAll(releases, 0755); err != nil {
		return err
	}
	next, err := os.MkdirTemp(releases, "release-")
	if err != nil {
		return err
	}
	if err = os.Chmod(next, 0755); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			os.RemoveAll(next)
		}
	}()
	current := filepath.Join(u.Config.RepoDir, "current")
	// Keep previous package filenames available to clients which fetched an older DB.
	old, err := filepath.Glob(filepath.Join(current, "*.pkg.tar.zst*"))
	if err != nil {
		return err
	}
	for _, p := range old {
		if err = os.Link(p, filepath.Join(next, filepath.Base(p))); err != nil {
			return err
		}
	}
	for _, p := range packages {
		for _, src := range []string{p, p + ".sig"} {
			dst := filepath.Join(next, filepath.Base(src))
			if err = os.Remove(dst); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err = copyFile(src, dst); err != nil {
				return err
			}
		}
	}
	// Rebuild the database from exactly the latest package recorded for each name.
	var latest []string
	for _, r := range state {
		filename := r.Filename
		if filename == "" {
			filename = fmt.Sprintf("%s-%s-x86_64.pkg.tar.zst", r.Name, r.fullVersion())
		}
		if filepath.Base(filename) != filename || !strings.HasSuffix(filename, ".pkg.tar.zst") {
			return errors.New("invalid package filename in repository state")
		}
		latest = append(latest, filepath.Join(next, filename))
	}
	slices.Sort(latest)
	args := []string{"--sign", "--key", u.Config.SigningKey, filepath.Join(next, u.Config.RepoName+".db.tar.gz")}
	args = append(args, latest...)
	// repo-add uses GNUPGHOME inherited from the service/installer environment.
	if err = u.run(ctx, "repo-add", args...); err != nil {
		return err
	}
	for _, name := range []string{u.Config.RepoName + ".db.tar.gz", u.Config.RepoName + ".db.tar.gz.sig"} {
		if _, err = os.Stat(filepath.Join(next, name)); err != nil {
			return err
		}
	}
	pub, err := u.Command(ctx, "gpg", "--homedir", filepath.Join(u.Config.StateDir, "gnupg"), "--armor", "--export", u.Config.SigningKey)
	if err != nil {
		return err
	}
	if len(pub) == 0 {
		return errors.New("repository public key export is empty")
	}
	if err = os.WriteFile(filepath.Join(next, "nier-repo.asc"), pub, 0644); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(next, "state.json"), data, 0644); err != nil {
		return err
	}
	link := filepath.Join(u.Config.RepoDir, ".next")
	if err = os.Remove(link); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = os.Symlink(filepath.Join("releases", filepath.Base(next)), link); err != nil {
		return err
	}
	if err = os.Rename(link, current); err != nil {
		return err
	}
	committed = true
	log.Printf("published %d updated package(s): %s", len(packages), next)
	return nil
}
func (u *Updater) update(ctx context.Context, check, rebuild bool, only string) error {
	for _, dir := range []string{u.Config.StateDir, filepath.Join(u.Config.StateDir, "work"), u.Config.RepoDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	lock, err := os.OpenFile(filepath.Join(u.Config.StateDir, "update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("another updater holds the lock: %w", err)
	}
	state, err := u.state()
	if err != nil {
		return err
	}
	var pending []Release
	for _, name := range u.Config.Packages {
		if only != "" && name != only {
			continue
		}
		r, err := u.discover(ctx, name)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		previous := state[name]
		if !rebuild && previous.Version != "" && compare(r.fullVersion(), previous.fullVersion()) <= 0 {
			log.Printf("%s is current (%s)", name, previous.fullVersion())
			continue
		}
		pending = append(pending, r)
		log.Printf("%s: %s -> %s", name, previous.fullVersion(), r.fullVersion())
	}
	if check {
		return json.NewEncoder(os.Stdout).Encode(pending)
	}
	if len(pending) == 0 {
		return nil
	}
	var packages []string
	for _, r := range pending {
		built, path, err := u.build(ctx, r)
		if err != nil {
			return fmt.Errorf("%s build failed (work retained): %w", r.Name, err)
		}
		state[r.Name] = built
		packages = append(packages, path)
	}
	if err = u.publish(ctx, state, packages); err != nil {
		return err
	}
	for _, p := range packages {
		os.RemoveAll(filepath.Dir(p))
	}
	return nil
}
func loadConfig(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	for _, p := range []string{c.StateDir, c.RepoDir, c.RecipeDir, c.VendorKey} {
		if !filepath.IsAbs(p) {
			return c, errors.New("configuration paths must be absolute")
		}
	}
	if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(c.RepoName) || c.SigningKey == "" || c.Image == "" || c.RecipeRevision < 1 || len(c.Packages) == 0 {
		return c, errors.New("invalid repository, signing key, image, revision or package list")
	}
	seen := map[string]bool{}
	for _, p := range c.Packages {
		if (p != "onedrive-abraunegg" && p != "onedrive-quickshell" && p != "nier-monitor-arrangement" && p != "remote-desktop-manager" && p != "teams-for-linux" && p != "neovim-nightly" && p != "powershell") || seen[p] {
			return c, fmt.Errorf("unsupported or duplicate package %q", p)
		}
		seen[p] = true
	}
	if seen["onedrive-quickshell"] && !seen["onedrive-abraunegg"] {
		return c, errors.New("onedrive-quickshell requires onedrive-abraunegg in the package list")
	}
	return c, nil
}
func main() {
	config := flag.String("config", "/etc/nier-pacman-updater/config.json", "configuration file")
	check := flag.Bool("check", false, "check releases without downloading, building or publishing")
	rebuild := flag.Bool("rebuild", false, "rebuild even if already current")
	only := flag.String("package", "", "update only the named configured package")
	importDir := flag.String("import-dir", "", "sign and atomically publish local package archives from this directory")
	flag.Parse()
	c, err := loadConfig(*config)
	if err != nil {
		log.Fatal(err)
	}
	if *only != "" && !slices.Contains(c.Packages, *only) {
		log.Fatal("package is not configured")
	}
	os.Setenv("GNUPGHOME", filepath.Join(c.StateDir, "gnupg"))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()
	ctx, timeout := context.WithTimeout(ctx, 110*time.Minute)
	defer timeout()
	u := Updater{Config: c, Client: &http.Client{Timeout: 20 * time.Minute}, GithubAPI: "https://api.github.com", CloudsmithAPI: "https://api.cloudsmith.io", Command: command}
	if *importDir != "" {
		if *check || *rebuild || *only != "" {
			log.Fatal("-import-dir cannot be combined with update options")
		}
		if err = u.importPackages(ctx, *importDir); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err = u.update(ctx, *check, *rebuild, *only); err != nil {
		log.Fatal(err)
	}
}
