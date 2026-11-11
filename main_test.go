package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureUpdater(t *testing.T, body string) (*Updater, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return &Updater{Config: Config{RecipeRevision: 1}, Client: server.Client(), GithubAPI: server.URL, CloudsmithAPI: server.URL}, server
}
func TestGitHubStableOnly(t *testing.T) {
	for _, test := range []struct {
		tag                     string
		draft, prerelease, fail bool
	}{
		{"v2.5.12", false, false, false}, {"v2.6.0-rc1", false, true, true}, {"v2.6.0", true, false, true}, {"v2.6.0/../../oops", false, false, true},
	} {
		t.Run(test.tag, func(t *testing.T) {
			b, _ := json.Marshal(map[string]any{"tag_name": test.tag, "draft": test.draft, "prerelease": test.prerelease})
			u, _ := fixtureUpdater(t, string(b))
			r, err := u.discover(context.Background(), "onedrive-abraunegg")
			if (err != nil) != test.fail {
				t.Fatalf("release=%+v err=%v", r, err)
			}
			if !test.fail && r.Version != "2.5.12" {
				t.Fatal(r)
			}
		})
	}
}
func rpm(version, arch string) map[string]any {
	return map[string]any{"name": "RemoteDesktopManager", "version": version, "format": "rpm", "status_str": "Completed", "is_downloadable": true, "cdn_url": "https://dl.cloudsmith.io/public/devolutions/rdm/package.rpm", "checksum_sha256": strings.Repeat("a", 64), "signature_url": "https://dl.cloudsmith.io/public/devolutions/rdm/package.asc", "identifiers": map[string]string{"architecture": arch}}
}
func TestCloudsmithSelectionAndTrust(t *testing.T) {
	hidden := rpm("9999.1-1", "x86_64")
	hidden["is_hidden"] = true
	b, _ := json.Marshal([]any{rpm("2026.2.9-1", "x86_64"), rpm("2026.3.2.1-1", "x86_64"), rpm("9999.1-1", "aarch64"), rpm("2027.1-beta-1", "x86_64"), hidden})
	u, _ := fixtureUpdater(t, string(b))
	r, err := u.discover(context.Background(), "remote-desktop-manager")
	if err != nil || r.fullVersion() != "2026.3.2.1-1.1" {
		t.Fatalf("%+v %v", r, err)
	}
	for _, field := range []string{"cdn_url", "checksum_sha256"} {
		p := rpm("2026.3.2.1-1", "x86_64")
		p[field] = "https://attacker.invalid/payload"
		b, _ = json.Marshal([]any{p})
		u, _ = fixtureUpdater(t, string(b))
		if _, err = u.discover(context.Background(), "remote-desktop-manager"); err == nil {
			t.Fatalf("accepted invalid %s", field)
		}
	}
	u, _ = fixtureUpdater(t, `[]`)
	if _, err = u.discover(context.Background(), "remote-desktop-manager"); err == nil {
		t.Fatal("accepted missing package")
	}
}
func TestVersionComparison(t *testing.T) {
	for _, v := range [][3]string{{"2.10.0-1", "2.9.0-1", "1"}, {"2026.3.2.1-1.2", "2026.3.2.1-1.1", "1"}, {"2.5.0-1", "2.5-1", "1"}, {"2.5.12-1", "2.5.12-1", "0"}} {
		got := compare(v[0], v[1])
		want := 0
		if v[2] == "1" {
			want = 1
		} else if v[2] == "-1" {
			want = -1
		}
		if got != want {
			t.Fatalf("compare(%s,%s)=%d", v[0], v[1], got)
		}
	}
}
func TestDownloadRejectsChecksumAndHTTPFailure(t *testing.T) {
	u, s := fixtureUpdater(t, `payload`)
	if _, err := u.download(context.Background(), s.URL, filepath.Join(t.TempDir(), "source"), strings.Repeat("a", 64)); err == nil {
		t.Fatal("accepted incorrect checksum")
	}
	s.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) })
	if _, err := u.download(context.Background(), s.URL, filepath.Join(t.TempDir(), "source"), ""); err == nil {
		t.Fatal("accepted HTTP 503")
	}
}
func TestPublishFailurePreservesLiveRepository(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "old")
	os.Mkdir(old, 0755)
	os.WriteFile(filepath.Join(old, "state.json"), []byte(`{}`), 0644)
	os.Symlink("old", filepath.Join(root, "current"))
	u := Updater{Config: Config{RepoDir: root, RepoName: "nier"}, Command: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("simulated repo-add failure")
	}}
	if err := u.publish(context.Background(), State{}, nil); err == nil {
		t.Fatal("publication unexpectedly succeeded")
	}
	target, _ := os.Readlink(filepath.Join(root, "current"))
	if target != "old" {
		t.Fatal("live pointer changed on failure")
	}
	dirs, _ := os.ReadDir(filepath.Join(root, "releases"))
	if len(dirs) != 0 {
		t.Fatal("failed staging directory retained")
	}
}
func TestNoopDoesNotBuildOrAdvanceState(t *testing.T) {
	u, _ := fixtureUpdater(t, `{"tag_name":"v2.5.12"}`)
	root := t.TempDir()
	u.Config.StateDir = filepath.Join(root, "state")
	u.Config.RepoDir = filepath.Join(root, "repo")
	u.Config.Packages = []string{"onedrive-abraunegg"}
	current := filepath.Join(u.Config.RepoDir, "current")
	os.MkdirAll(current, 0755)
	initial := `{"onedrive-abraunegg":{"name":"onedrive-abraunegg","version":"2.5.12","pkgrel":"1"}}`
	os.WriteFile(filepath.Join(current, "state.json"), []byte(initial), 0644)
	u.Command = func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("no-op invoked build/publication command")
		return nil, nil
	}
	if err := u.update(context.Background(), false, false, ""); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(current, "state.json"))
	if string(got) != initial {
		t.Fatal("no-op changed state")
	}
}

func TestQuickshellStableReleaseAndRemote(t *testing.T) {
	for _, test := range []struct {
		tag                     string
		draft, prerelease, fail bool
	}{
		{"v1.6.0", false, false, false}, {"1.6.1", false, false, false},
		{"v1.7.0-rc1", false, true, true}, {"v1.7.0", true, false, true},
		{"v1.7.0/../../oops", false, false, true},
	} {
		t.Run(test.tag, func(t *testing.T) {
			b, _ := json.Marshal(map[string]any{"tag_name": test.tag, "draft": test.draft, "prerelease": test.prerelease})
			u, server := fixtureUpdater(t, string(b))
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/0byt3/onedrive-quickshell/releases/latest" {
					t.Errorf("wrong remote: %s", r.URL.Path)
				}
				w.Write(b)
			})
			release, err := u.discover(context.Background(), "onedrive-quickshell")
			if (err != nil) != test.fail {
				t.Fatalf("%+v: %v", release, err)
			}
			if !test.fail && release.URL != "https://github.com/0byt3/onedrive-quickshell/archive/refs/tags/"+test.tag+".tar.gz" {
				t.Fatal(release)
			}
			if !test.fail && !strings.HasPrefix(release.URL, "https://github.com/") {
				t.Fatalf("GitHub source must use HTTPS: %s", release.URL)
			}
		})
	}
}

func TestTeamsStableRPMAsset(t *testing.T) {
	const version = "2.28.0"
	const filename = "teams-for-linux-" + version + ".x86_64.rpm"
	const download = "https://github.com/IsmaelMartinez/teams-for-linux/releases/download/v" + version + "/" + filename
	valid := map[string]any{
		"tag_name": "v" + version, "draft": false, "prerelease": false,
		"assets": []any{map[string]any{
			"name": filename, "browser_download_url": download,
			"digest": "sha256:" + strings.Repeat("a", 64), "size": 92069441,
		}},
	}
	b, _ := json.Marshal(valid)
	u, server := fixtureUpdater(t, string(b))
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/IsmaelMartinez/teams-for-linux/releases/latest" {
			t.Errorf("wrong remote: %s", r.URL.Path)
		}
		w.Write(b)
	})
	release, err := u.discover(context.Background(), "teams-for-linux")
	if err != nil || release.Version != version || release.URL != download || release.SHA256 != strings.Repeat("a", 64) {
		t.Fatalf("release=%+v err=%v", release, err)
	}

	for _, mutate := range []func(map[string]any){
		func(v map[string]any) { v["prerelease"] = true },
		func(v map[string]any) { v["tag_name"] = "v2.29.0-rc1" },
		func(v map[string]any) { v["assets"] = []any{} },
		func(v map[string]any) {
			asset := v["assets"].([]any)[0].(map[string]any)
			asset["browser_download_url"] = "https://attacker.invalid/teams.rpm"
		},
		func(v map[string]any) {
			asset := v["assets"].([]any)[0].(map[string]any)
			asset["digest"] = ""
		},
	} {
		var candidate map[string]any
		if err := json.Unmarshal(b, &candidate); err != nil {
			t.Fatal(err)
		}
		mutate(candidate)
		bad, _ := json.Marshal(candidate)
		u, _ := fixtureUpdater(t, string(bad))
		if _, err := u.discover(context.Background(), "teams-for-linux"); err == nil {
			t.Fatalf("accepted invalid release: %s", bad)
		}
	}
}

func TestQuickshellRequiresClientInConfig(t *testing.T) {
	data, err := os.ReadFile("config.json")
	if err != nil {
		t.Fatal(err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	for _, packages := range [][]string{{"onedrive-quickshell", "onedrive-abraunegg"}, {"onedrive-quickshell"}} {
		config.Packages = packages
		data, _ = json.Marshal(config)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		_, err := loadConfig(path)
		if (err == nil) != (len(packages) == 2) {
			t.Fatalf("packages=%v err=%v", packages, err)
		}
	}
}

func TestNeovimNightly(t *testing.T) {
	const download = "https://github.com/neovim/neovim/releases/download/nightly/nvim-linux-x86_64.tar.gz"
	valid := map[string]any{
		"tag_name": "nightly", "prerelease": true,
		"assets": []any{map[string]any{
			"name": "nvim-linux-x86_64.tar.gz", "browser_download_url": download,
			"digest": "sha256:" + strings.Repeat("a", 64), "size": 12345,
			"id": 123, "updated_at": "2026-11-04T05:17:17Z",
		}},
	}
	b, _ := json.Marshal(valid)
	u, server := fixtureUpdater(t, string(b))
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/neovim/neovim/releases/tags/nightly" {
			t.Errorf("wrong remote: %s", r.URL.Path)
		}
		w.Write(b)
	})
	r, err := u.discover(context.Background(), "neovim-nightly")
	if err != nil || r.Version != "20261104051717.123" || r.URL != download || r.SHA256 != strings.Repeat("a", 64) {
		t.Fatalf("release=%+v err=%v", r, err)
	}
	for _, mutate := range []func(map[string]any){
		func(v map[string]any) { v["draft"] = true },
		func(v map[string]any) { v["tag_name"] = "stable" },
		func(v map[string]any) { v["assets"] = []any{} },
		func(v map[string]any) { v["assets"] = append(v["assets"].([]any), v["assets"].([]any)[0]) },
		func(v map[string]any) { v["assets"].([]any)[0].(map[string]any)["digest"] = "" },
		func(v map[string]any) {
			v["assets"].([]any)[0].(map[string]any)["browser_download_url"] = "https://attacker.invalid/nvim.tar.gz"
		},
		func(v map[string]any) { v["assets"].([]any)[0].(map[string]any)["updated_at"] = "" },
		func(v map[string]any) { v["assets"].([]any)[0].(map[string]any)["id"] = 0 },
	} {
		var candidate map[string]any
		json.Unmarshal(b, &candidate)
		mutate(candidate)
		bad, _ := json.Marshal(candidate)
		u, _ := fixtureUpdater(t, string(bad))
		if _, err := u.discover(context.Background(), "neovim-nightly"); err == nil {
			t.Fatalf("accepted invalid release: %s", bad)
		}
	}
}

func TestPowerShellStableArchive(t *testing.T) {
	const version = "7.6.9"
	const filename = "powershell-" + version + "-linux-x64.tar.gz"
	const download = "https://github.com/PowerShell/PowerShell/releases/download/v" + version + "/" + filename
	valid := map[string]any{
		"tag_name": "v" + version, "draft": false, "prerelease": false,
		"assets": []any{map[string]any{
			"name": filename, "browser_download_url": download,
			"digest": "sha256:" + strings.Repeat("a", 64), "size": 92069441,
		}},
	}
	b, _ := json.Marshal(valid)
	u, server := fixtureUpdater(t, string(b))
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/PowerShell/PowerShell/releases/latest" {
			t.Errorf("wrong remote: %s", r.URL.Path)
		}
		w.Write(b)
	})
	release, err := u.discover(context.Background(), "powershell")
	if err != nil || release.Version != version || release.URL != download || release.SHA256 != strings.Repeat("a", 64) {
		t.Fatalf("release=%+v err=%v", release, err)
	}

	for _, mutate := range []func(map[string]any){
		func(v map[string]any) { v["prerelease"] = true },
		func(v map[string]any) { v["draft"] = true },
		func(v map[string]any) { v["assets"] = append(v["assets"].([]any), v["assets"].([]any)[0]) },
		func(v map[string]any) { v["assets"].([]any)[0].(map[string]any)["size"] = 0 },
		func(v map[string]any) {
			v["assets"].([]any)[0].(map[string]any)["name"] = "powershell-7.6.9-linux-arm64.tar.gz"
		},
		func(v map[string]any) {
			v["assets"].([]any)[0].(map[string]any)["browser_download_url"] = download + "?redirect=1"
		},
		func(v map[string]any) { v["tag_name"] = "v2.29.0-rc1" },
		func(v map[string]any) { v["assets"] = []any{} },
		func(v map[string]any) {
			asset := v["assets"].([]any)[0].(map[string]any)
			asset["browser_download_url"] = "https://attacker.invalid/teams.rpm"
		},
		func(v map[string]any) {
			asset := v["assets"].([]any)[0].(map[string]any)
			asset["digest"] = ""
		},
	} {
		var candidate map[string]any
		if err := json.Unmarshal(b, &candidate); err != nil {
			t.Fatal(err)
		}
		mutate(candidate)
		bad, _ := json.Marshal(candidate)
		u, _ := fixtureUpdater(t, string(bad))
		if _, err := u.discover(context.Background(), "powershell"); err == nil {
			t.Fatalf("accepted invalid release: %s", bad)
		}
	}
}

func TestMonitorArrangementStableReleaseAndHTTPS(t *testing.T) {
	for _, test := range []struct {
		tag                     string
		draft, prerelease, fail bool
	}{
		{"v0.1.0", false, false, false}, {"0.2.1", false, false, false},
		{"v0.2.0-rc1", false, true, true}, {"v0.2.0", true, false, true},
		{"v0.3.0", false, true, true}, {"main", false, false, true},
		{"v0.1.0/../../oops", false, false, true},
	} {
		t.Run(test.tag, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"tag_name": test.tag, "draft": test.draft, "prerelease": test.prerelease})
			u, server := fixtureUpdater(t, string(body))
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/0byt3/nier-monitor-arrangement/releases/latest" {
					t.Errorf("wrong remote: %s", r.URL.Path)
				}
				w.Write(body)
			})
			release, err := u.discover(context.Background(), "nier-monitor-arrangement")
			if (err != nil) != test.fail {
				t.Fatalf("%+v: %v", release, err)
			}
			if !test.fail {
				expected := "https://github.com/0byt3/nier-monitor-arrangement/archive/refs/tags/" + test.tag + ".tar.gz"
				if release.URL != expected || release.Version != strings.TrimPrefix(test.tag, "v") {
					t.Fatal(release)
				}
			}
		})
	}
}

func TestMonitorArrangementStandaloneConfig(t *testing.T) {
	data, err := os.ReadFile("config.json")
	if err != nil {
		t.Fatal(err)
	}
	var config Config
	if err = json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	for _, packages := range [][]string{{"nier-monitor-arrangement"}, {"nier-monitor-arrangement", "nier-monitor-arrangement"}} {
		config.Packages = packages
		data, _ = json.Marshal(config)
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		_, err := loadConfig(path)
		if (err == nil) != (len(packages) == 1) {
			t.Fatalf("packages=%v err=%v", packages, err)
		}
	}
}

func TestUxPlayDiscovery(t *testing.T) {
	for _, test := range []struct {
		tag                     string
		draft, prerelease, fail bool
	}{
		{"v1.73.7", false, false, false},
		{"1.73.7", false, false, false},
		{"v1.74", false, true, true},
		{"v1.74", true, false, true},
		{"v1.74-rc1", false, false, true},
		{"v1.73.7/../../oops", false, false, true},
	} {
		t.Run(test.tag, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"tag_name": test.tag, "draft": test.draft, "prerelease": test.prerelease})
			u, server := fixtureUpdater(t, string(body))
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/FDH2/UxPlay/releases/latest" {
					t.Errorf("wrong remote: %s", r.URL.Path)
				}
				w.Write(body)
			})
			release, err := u.discover(context.Background(), "uxplay")
			if (err != nil) != test.fail {
				t.Fatalf("%+v: %v", release, err)
			}
			if !test.fail && (release.Version != strings.TrimPrefix(test.tag, "v") ||
				release.URL != "https://github.com/FDH2/UxPlay/archive/refs/tags/"+test.tag+".tar.gz") {
				t.Fatal(release)
			}
		})
	}
}

func TestNufmtDiscovery(t *testing.T) {
	for _, test := range []struct {
		tag                     string
		draft, prerelease, fail bool
	}{
		{"v0.1.6", false, false, false},
		{"0.1.6", false, false, false},
		{"v0.2.0", false, true, true},
		{"v0.2.0", true, false, true},
		{"v0.2.0-rc1", false, false, true},
		{"v0.1.6/../../oops", false, false, true},
	} {
		t.Run(test.tag, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"tag_name": test.tag, "draft": test.draft, "prerelease": test.prerelease})
			u, server := fixtureUpdater(t, string(body))
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/nushell/nufmt/releases/latest" {
					t.Errorf("wrong remote: %s", r.URL.Path)
				}
				w.Write(body)
			})
			release, err := u.discover(context.Background(), "nufmt")
			if (err != nil) != test.fail {
				t.Fatalf("%+v: %v", release, err)
			}
			if !test.fail && (release.Version != strings.TrimPrefix(test.tag, "v") ||
				release.URL != "https://github.com/nushell/nufmt/archive/refs/tags/"+test.tag+".tar.gz") {
				t.Fatal(release)
			}
		})
	}
}

func TestNufmtFetchAndOfflineBuildFailures(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			u, server := fixtureUpdater(t, "source archive")
			u.Config.StateDir = t.TempDir()
			u.Config.RecipeDir = "recipes"
			u.Config.Image = "test-builder"
			if err := os.Mkdir(filepath.Join(u.Config.StateDir, "work"), 0755); err != nil {
				t.Fatal(err)
			}
			calls := 0
			u.Command = func(_ context.Context, name string, args ...string) ([]byte, error) {
				calls++
				if name != "podman" || calls > failAt {
					t.Fatalf("unexpected command after failure: %s %v", name, args)
				}
				joined := strings.Join(args, " ")
				if calls == 1 {
					if !strings.Contains(joined, "--network=pasta") || !strings.Contains(joined, "--nobuild") {
						t.Fatalf("dependency fetch: %s", joined)
					}
				} else if !strings.Contains(joined, "--network=none") || !strings.Contains(joined, "--cleanbuild") {
					t.Fatalf("compilation must be offline: %s", joined)
				}
				if calls == failAt {
					return nil, errors.New("simulated failure")
				}
				return nil, nil
			}
			_, _, err := u.build(context.Background(), Release{Name: "nufmt", Version: "0.1.6", Pkgrel: "14", URL: server.URL})
			if err == nil || calls != failAt {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}
