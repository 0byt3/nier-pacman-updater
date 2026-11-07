# nier-pacman-updater

A Go service that checks upstream releases, builds signed Arch Linux packages,
and publishes a pacman repository served by Nginx in a rootless Podman container.
Bash handles installation and packaging recipes. It runs on x86_64 Arch Linux;
VM 107 is the deployment/test server.

## Packages and trusted sources

- **`onedrive-abraunegg`**: latest stable GitHub release of
  [abraunegg/onedrive](https://github.com/abraunegg/onedrive), compiled from its
  release source archive. Drafts, prereleases, and nonnumeric tags are rejected.
  The package provides `onedrive` and supplies the client used by the widget.
- **`onedrive-quickshell`**: latest numeric stable GitHub release of
  [0byt3/onedrive-quickshell](https://github.com/0byt3/onedrive-quickshell),
  built offline from its vendored Go source. Its pacman dependencies include
  `onedrive-abraunegg>=2.5.12` directly, Omarchy 4.0.6 or newer, and Quickshell.
  The widget uses Omarchy's shell components and terminal launcher; Nier Desktop
  is not required. Installation does not enable account services or edit bar
  preferences; the user runs `nier-onedrive-setup` to configure accounts/widgets.
- **`nier-monitor-arrangement`**: latest numeric stable release of
  [0byt3/nier-monitor-arrangement](https://github.com/0byt3/nier-monitor-arrangement).
  Queries `https://api.github.com/repos/0byt3/nier-monitor-arrangement/releases/latest`
  and builds the matching GitHub HTTPS tag archive offline. Drafts, prereleases
  and nonnumeric tags are rejected. The standalone Quickshell editor includes
  its desktop-and-laptop menu icon and optional per-user Hyprland loader.
  Runtime dependencies include Quickshell, Lua and Lua-enabled Hyprland; Lua is
  also installed in the builder to run profile-decoder tests. No SSH key is used
  to fetch releases or source. Installing it does not enable the user hook;
  run `nier-monitor-arrangement enable` as the desktop user.
- **`remote-desktop-manager`**: latest completed numeric stable x86_64 RPM in
  [Devolutions' official Cloudsmith repository](https://cloudsmith.io/~devolutions/repos/rdm/).
  The updater queries Cloudsmith's API, verifies the SHA-256 checksum and embedded
  RPM signature, and repackages the vendor's files for pacman. No AUR recipes
  or packages are fetched or executed.
- **`teams-for-linux`**: latest numeric stable GitHub release from the official
  `IsmaelMartinez/teams-for-linux` repository. The updater selects exactly one
  release asset named for the version and x86_64 RPM architecture, requires
  GitHub's SHA-256 asset digest, and repackages the RPM offline. Drafts,
  prereleases, malformed tags, missing digests, and off-repository URLs are
  rejected. No AUR recipe or package is used.

- **`powershell`**: latest numeric stable release from Microsoft's
  [PowerShell/PowerShell](https://github.com/PowerShell/PowerShell) repository.
  Selects exactly `powershell-VERSION-linux-x64.tar.gz` (self-contained glibc
  build), requiring GitHub's SHA-256 asset digest and exact repository release
  URL. Drafts, prereleases, missing/duplicate assets, and invalid digests are
  rejected. Repackages offline under `/opt/microsoft/powershell/7` and exposes
  `/usr/bin/pwsh`, without changing login shells or shell startup files.
  Includes the bundled .NET runtime; system dependencies are glibc, libgcc,
  libstdc++, ICU, Kerberos, OpenSSL, and zlib. Conflicts with alternate
  `powershell-bin` and `powershell-git` packages. No AUR recipe is used.
- **`neovim-nightly`**: official `neovim/neovim` nightly Linux x86_64
  tarball, repackaged offline with GitHub's required SHA-256 asset digest.
  Tracks the moving nightly tag using asset UTC update time and asset ID as
  `YYYYMMDDhhmmss.ID`. A concurrent upstream replacement fails checksum
  verification; the next hourly run retries discovery. Runtime and optional
  dependencies match Arch extra/neovim as of 2026-11-04, including its library
  ABI constraints; review these when Arch changes Neovim dependencies.
  Upstream bundles some libraries, so matching dependencies can be redundant.
  Provides `neovim=<upstream API version>` detected from the bundled binary and
  conflicts with `neovim`; installing it
  prompts to remove stable Neovim. It does not automatically replace stable
  installations during system upgrades. The versioned provide satisfies Omarchy
  4.5's `omarchy-nvim` dependency on `neovim>=0.9.0` without treating the
  date-based package version as a Neovim API version. Recipe revision 7 adds this.

The plugin's source must first be pushed to its origin and published as a stable
GitHub release (initially `v1.6.0`, including `vendor/`). Tags alone do not count.
Until that release exists, leave it out of a deployed configuration: a missing
upstream release fails the update and preserves the existing repository.
The Cloudsmith key is pinned to `880E66407780AD737F46CBADFE7407ECB26FD2FE`, from
[Devolutions' installation instructions](https://docs.devolutions.net/rdm/installation/client/?tab=linux).
Key rotation requires review. OneDrive's source archive is fetched over HTTPS;
its computed checksum is recorded, but it is not a vendor-signed source artifact.

## Installation

On the Arch server, from a copy of this repository:

```sh
sudo bash requirements.sh
```

This performs a full Arch upgrade, installs official dependencies, builds the Go
executable, creates the `nier-updater` account and subordinate UID/GID mappings,
generates a dedicated repository signing key, builds the rootless builder image,
pulls the official `docker.io/library/nginx:stable-alpine` image, and installs/enables:

- `nier-pacman-updater.service`: release checks, builds, signing, and publication.
- `nier-pacman-updater.timer`: hourly, with up to five minutes of jitter;
  missed checks run when the server returns.
- `nier-pacman-repo.service`: Nginx in a rootless Podman container on **8080**,
  managed by the dedicated `nier-repo-web` system account.

The first update starts asynchronously. `requirements.sh --no-start` installs
without enabling or starting units. Reinstallation preserves configuration and
keys, refreshes the builder/Nginx images and program, and reinstalls units/recipes.
Reinstallation restarts the repository container to load its image/configuration.
Restart the updater service separately if a previous update is still running.
The dedicated signing key is unencrypted for unattended use, in a mode-0700
service-owned directory. Back it up securely; replacing it requires distributing
a new public key. Private keys never enter build containers or source control.

## Configuration and operation

`config.json` installs to `/etc/nier-pacman-updater/config.json`. The installer
provisions the default paths; custom paths need matching ownership and unit
changes. Configurable values include the approved package list, builder image,
repository name, signing key, and pinned vendor key. Increment `recipe_revision`
after recipe changes to rebuild an already published upstream release.

The default configuration includes `onedrive-quickshell`, `teams-for-linux`,
`nier-monitor-arrangement`, `neovim-nightly`, `powershell`, and recipe revision 11. Reinstallation preserves an existing package selection, so
add newly adopted packages to the deployed `packages` list and update
`recipe_revision` explicitly before running them. Rebuild the builder image with
`requirements.sh` after build dependency changes. GitHub source and asset URLs
are pinned to their respective repository origins; the updater does not depend
on sibling checkouts or AUR recipes.

- `/var/lib/nier-pacman-updater/`: container storage, work, and GPG home.
- `/srv/nier-pacman-repo/releases/`: published repository generations.
- `/srv/nier-pacman-repo/current`: atomically switched live-generation symlink.
- `/usr/share/nier/bin/nier-pacman-updater`: installed executable, with a
  compatibility link at `/usr/local/bin/nier-pacman-updater`.
- `/usr/share/nier/pacman-updater/recipes/`: installed recipe templates and pacman install scripts;
  its parent directory also holds `Containerfile`.
- `/etc/nier-pacman-updater/nginx.conf`: Nginx configuration.
- `/var/lib/nier-pacman-repo/`: separate web account's rootless image storage.

On reinstallation, the installer migrates the former default recipe directory
`/usr/local/lib/nier-pacman-updater/recipes` to the new configured path and
increments the existing recipe revision once. Custom recipe paths and all
other settings are retained. Existing recipe copies at the old directory are
left alone; migrated configurations use the new directory. This source change
has not been deployed to the live updater service.

The `nier-pacman-repo` container runs as UID/GID 101 with all capabilities
dropped, no new privileges, a read-only root filesystem, and a temporary `/tmp`.
Only the repository parent and Nginx configuration are mounted, both read-only.
Nginx serves `/repo/current` with cache revalidation, autoindex, GET/HEAD only,
and byte-range downloads. Mounting the parent directory lets it follow atomic
publication symlink changes without restarting. The signing key is not mounted.
The Go updater no longer includes an HTTP serving mode.

```sh
sudo systemctl start nier-pacman-updater.service
sudo journalctl -u nier-pacman-updater.service -n 100 --no-pager
systemctl list-timers nier-pacman-updater.timer

# Metadata only: no artifact downloads, builds, or publication
sudo -u nier-updater /usr/local/bin/nier-pacman-updater -check

# Select one package, optionally forcing a rebuild
sudo -u nier-updater env XDG_RUNTIME_DIR=/run/nier-pacman-updater \
  /usr/local/bin/nier-pacman-updater -package onedrive-abraunegg
sudo -u nier-updater env XDG_RUNTIME_DIR=/run/nier-pacman-updater \
  /usr/local/bin/nier-pacman-updater -package remote-desktop-manager -rebuild
sudo -u nier-updater env XDG_RUNTIME_DIR=/run/nier-pacman-updater \
  /usr/local/bin/nier-pacman-updater -package teams-for-linux
```

Optional GitHub credentials go in root-only
`/etc/nier-pacman-updater/environment` as `GITHUB_TOKEN=...`. The token is used
only for GitHub API requests; anonymous hourly checks work at this scale.
RDM's RPM release and recipe revision become Arch `pkgrel` (e.g. `1.1`);
OneDrive uses the recipe revision directly (`2.5.12-1`). Older upstream versions
never automatically replace newer published versions.

## Build and failure behavior

Published `state.json` is the version record. A lock prevents overlapping runs.
Sources are downloaded and verified before `makepkg` runs unprivileged in a
rootless container with networking disabled and only its work directory mounted.
The image contains build dependencies. `makepkg --nodeps` avoids needing GUI
runtime dependencies merely to repackage an RPM; those dependencies remain in
the package metadata for pacman clients.

Package metadata must match the requested name/version/architecture. Packages
and databases are signed. All selected updates must build before publication.
A complete new generation is assembled before switching the live symlink.
Failures leave the previous live repository and state untouched, fail the
systemd service, and are retried by the next timer. See the journal for errors.

Previous package filenames remain available for clients with older databases.
Old generations and failed work are retained for diagnosis; monitor disk usage
and remove obsolete generations/work manually. Never delete the live generation
or its packages. Automatic garbage collection and notifications are not implemented.
The builder image follows rolling Arch rather than a pinned reproducible toolchain.
The hourly job validates metadata/signatures; it does not launch a graphical RDM
session or authenticate OneDrive. Runtime installation is tested separately.

## Pacman clients

The VM currently serves `http://10.0.2.36:8080/` over the LAN (DHCP-assigned).
Verify the repository signing fingerprint through the administrator, then:

```sh
curl -f http://10.0.2.36:8080/nier-repo.asc -o /tmp/nier-repo.asc
sudo pacman-key --add /tmp/nier-repo.asc
sudo pacman-key --lsign-key REPLACE_WITH_VERIFIED_REPOSITORY_KEY_FINGERPRINT
```

Add to `/etc/pacman.conf`:

```ini
[nier]
SigLevel = Required DatabaseRequired
Server = http://10.0.2.36:8080
```

Then run `sudo pacman -Syu onedrive-quickshell remote-desktop-manager teams-for-linux` on Omarchy.
Pacman installs `onedrive-abraunegg` as the widget's dependency. For a client
without Omarchy, install `onedrive-abraunegg` directly instead.
Package/database signatures remain required over HTTP. Set a DHCP reservation
or static address before relying on the URL across machines. The updater
maintains the repository; it does not upgrade installed client packages.

## Development

```sh
go test -race ./...
go vet ./...
go build .
bash -n requirements.sh
```

Tests cover stable filtering, Cloudsmith selection/trust checks, failed downloads,
version comparison, no-op runs, and preservation of the live repository on failure.
Real rootless builds and signed pacman installation are tested on the Arch VM.

For the plugin's offline package build and helper smoke checks on VM 107:

```sh
sudo bash tests/quickshell-build.sh /path/to/onedrive-quickshell
```

This builds a separate test image and retains its artifacts, without publishing
or modifying the production image/configuration. It verifies the direct client
dependency and installs the plugin into a disposable container with dependency
checks skipped. It does not validate dependency resolution or graphical behavior.
Run `bash tests/qml-smoke.sh` in the plugin checkout on an Omarchy Wayland
desktop to load its QML against the installed Omarchy components.

On VM 107, after installation and the first successful publication, run:

```sh
sudo bash tests/nginx.sh
sudo bash tests/vm-integration.sh
```

This checks no-op and failed-build behavior, then uses a disposable rootless
client container to import the verified public key, install both packages
through the HTTP repository with signatures required, run `onedrive --version`,
and check RDM startup for 20 seconds under Xvfb. The client shares the VM's
network to reach its repository; it does not install the applications on the
VM host. OneDrive authentication/sync and RDM remote connections are not tested.

## Verified VM deployment (2026-11-04)

- Repository: `http://10.0.2.36:8080/`.
- Published: `onedrive-abraunegg 2.5.12-4`,
  `remote-desktop-manager 2026.3.2.1-1.4`, and
  `teams-for-linux 2.28.0-4`.
- Signing fingerprint: `D08DCD6B97C9885E62968C927DEEC45C73A8FB34`.
- Nginx Podman HTTP service and hourly timer are enabled and running.
- Real rootless builds, signed pacman installs, startup smoke tests, no-op
  behavior, and failure preservation passed. RPM verification accepted the
  signed vendor RPM and rejected a copy with its signature removed.
- The Teams for Linux package was installed from the signed repository in a
  disposable client, and its version command passed as an unprivileged user.

### Neovim nightly validation (2026-11-04)

On VM 107, built the official nightly asset with digest verification in the
existing builder image, without publishing or changing deployed configuration.
Package: `neovim-nightly 20261104051717.708331109-5`.
The extracted package passed `nvim --version`
(`v0.13.0-dev-2012+gb4d549f439`) and headless startup with its packaged runtime.
Build dependencies were skipped for this binary repackaging test; a full pacman
dependency-resolution/install transaction was not tested. Test artifacts remain
at `/tmp/nier-neovim-test.mOyGXH` on VM 107.

### PowerShell stable validation (2026-11-04)

On VM 107, built `powershell 7.6.9-6` from Microsoft's Linux x64 archive
with GitHub's SHA-256 digest verified. A disposable Arch container completed
a full system upgrade and installed the archive through `pacman -U`, resolving
all runtime dependencies. `pacman -Qkk powershell` reported 627 files with none
altered. Noninteractive `pwsh -NoProfile` tests passed for version reporting,
pipelines, JSON, ICU culture lookup, and SHA-256 cryptography.
Artifacts and the smoke script remain at `/tmp/nier-powershell-test.GZ30Bh`
on VM 107. No live repository publication or deployed config change was made.
To enable tracking in an existing deployment, install the updated updater and
recipes, add `powershell` to its package list, and set recipe revision to 6.

### PowerShell deployment (2026-11-04)

Deployed the updated executable and recipe templates to VM 107, preserving
existing settings and adding only `powershell` to the enabled package list.
Recipe revision is 6. Neovim nightly and the OneDrive widget remain disabled.
The PowerShell-only update signed and published `powershell 7.6.9-6` in
`releases/release-3156174935`. A disposable client installed it from the live
HTTP repository with package/database signatures required, after checking the
repository key against fingerprint
`D08DCD6B97C9885E62968C927DEEC45C73A8FB34`. Integrity and PowerShell smoke tests
passed. A subsequent discovery check reported the release current.
The hourly updater timer and repository service remain active.
The global revision increase also schedules the other enabled packages for
rebuild on their next update run.

Previous binary, config, recipes, state, repository generation pointer, and
source checkout are backed up under
`/var/backups/nier-pacman-updater/powershell-20261104T150954Z/`.
Signed-client validation log: `/tmp/nier-powershell-signed-client.log`.
VM 106 was not changed.

## Windows VM images (outside pacman)

Nginx serves `/images/` from `/srv/nier-pacman-repo/images/`, independently of
`current` and pacman generations. These are qcow2 artifacts, not Arch packages.
The Windows reference is built/updated/cleaned/Sysprepped on VM 106; see
`../nier-desktop/windows/README.md`. A versioned qcow2 and a clearsigned JSON
manifest are published under `images/windows11-pro/releases/<version>/`.
`images/windows11-pro/latest.json.asc` atomically selects the signed descriptor.
Clients authenticate it with the pinned repository public key and check the
image's SHA-256 before cloning an independent writable disk.

After installing `qemu-img` (included by `requirements.sh`) and preparing readable,
sanitized input files, invoke as `nier-updater`:

```sh
bash /usr/share/nier/pacman-updater/tools/publish-windows-image.sh IMAGE.qcow2 VERSION BUILD-REPORT.json
```

The tool rejects backing files, encrypted images, image check errors, unfinished
updates or missing sanitization/generalization attestations. It preserves the
previous latest pointer on failures before publication. The signing key stays
in the service-owned GnuPG directory; only public artifacts are exposed by Nginx.
No timer automatically rebuilds or publishes Windows images. Source/account
credentials, answer ISOs, TPM and NVRAM files must never be uploaded.

For pre-release validation, set `NIER_WINDOWS_IMAGE_DIR` to
`/srv/nier-pacman-repo/images/windows11-pro-candidate` when invoking the publisher.
Point the real wizard at that channel's `latest.json.asc`. Publish the validated
artifact to the default channel only after the fresh personalization test passes;
the default latest pointer then advances atomically.

On 2026-11-04, version `2026.11.04-26300.9711.2` was validated on VM 106 and
published at `http://10.0.2.36:8080/images/windows11-pro/latest.json.asc`.
Its signed build report includes the fresh installation/cold-boot checks.
The image is 14,025,553,408 bytes, SHA-256
`58ffcd35de8b77ffd870b87c6fee133672ac449cb3feff3e33b3b356bbd51bfb`.
No desktop pacman package or pacman repository generation was published for this change.

Large-image writes stalled VM 107's original Btrfs/system-disk backing during
the first transfer. Image storage now uses a separate 128 GiB NVMe-backed
Proxmox disk (`scsi1`, serial `nier-win-images`), formatted ext4 and mounted at
`/var/lib/nier-windows-images`. Its `published/` directory is bind-mounted at
`/srv/nier-pacman-repo/images`; private staging is in `incoming/` (mode 0700,
owned by `nier-updater`). Both mounts are in `/etc/fstab`, whose prior version
is `/etc/fstab.nier-windows-images.before`. The web unit requires the image
mount before starting. Runtime mount/HTTP/range checks passed; an outer-server
reboot was not required or tested. The hourly package updater remains enabled.

## Publishing locally maintained packages

The updater can sign and atomically publish already-built archives:

```sh
nier-pacman-updater -config /etc/nier-pacman-updater/config.json \
  -import-dir /path/to/package-archives
```

Run as the repository service account with its normal environment. The import
shares the hourly updater's lock, validates archive metadata and filenames,
rejects duplicate names and downgrades/changed same-version archives, and stages
copies before signing. An identical import is a no-op. Upstream-managed names
must use their normal recipes instead. Keep private signing keys outside build
containers.

Imported packages remain in repository state across subsequent upstream updates.
The optional `filename` field preserves their original `any` or `x86_64` archive
name; existing upstream state without that field remains compatible. Desktop,
ble.sh and workspace packages use this path. Plugins and themes install
read-only assets; activation remains an explicit user configuration action.

## Complete package publication (2026-11-06)

The production repository at `http://10.0.2.36:8080/` now contains ten packages:
`nier-desktop 3.0.0-20`, `blesh 0.4.0_devel3-2`,
`nier-workspaces-dropdown 1.0.0-1`, `nier-theme-eag 1.0.0-1`,
`onedrive-abraunegg 2.5.12-7`, `onedrive-quickshell 1.6.0-7`,
`neovim-nightly 20261106213034.718630294-7`, `powershell 7.6.9-7`,
`remote-desktop-manager 2026.3.2.1-1.7`, and `teams-for-linux 2.28.0-7`.
OneDrive's complete source was released as
[v1.6.0](https://github.com/0byt3/onedrive-quickshell/releases/tag/v1.6.0).

All six upstream recipes are enabled on VM107 at recipe revision 7. The four
locally maintained packages were imported atomically and remain in state across
future upstream publications, including the original `any` filenames. The
public key and existing signature requirements were retained. Production's
previous package filenames remain available for clients holding the old database.

A separate signed candidate generation was installed in a disposable rootless
Arch/Omarchy client with package/database signatures required. All ten packages
passed integrity checks with zero altered files. OneDrive, its setup/monitor
commands, PowerShell and Neovim passed executable smoke tests. The desktop's
real upgrade hook passed with Omarchy already installed. Both packaged widgets
loaded against Omarchy 4.5 components in a headless Wayland session. Container
validation included documentation and masked only bootloader/initramfs hooks
that require a real EFI system partition. No account authentication, real sync,
RDM remote session or Teams login was attempted.

Promotion switched `current` atomically to
`releases/release-all-packages-r7pmkuFl`. A subsequent production updater run
reported all six upstream packages current and completed successfully. The
hourly timer and HTTP service are active. The updater's vendor-key inspection
uses a short temporary public keyring, avoiding GnuPG socket-path limits in
nested staging directories.

VM107 backups: `/var/backups/nier-pacman-updater/all-packages-20261106/`.
Build/import/client-test inputs: `/var/lib/nier-pacman-updater/all-packages/`.
Validated candidate: `/srv/nier-pacman-repo/images/all-packages-candidate/`.
Local nonsecret evidence: `../nier-desktop/dist/publication/` (ignored).
VM106 was not changed; it still uses the earlier limited test-repository URL.
Widget/theme activation remains separate from package publication/installation.

### EAG theme packaging withdrawn (2026-11-06)

The user clarified that EAG themes use Omarchy's native theme installation flow,
not pacman. `nier-theme-eag` was removed from the production database, current
archive set, repository state and local import inputs. The theme PKGBUILD and
package build script were removed; the theme source and original installation
instructions remain. The repository now contains nine packages in
`releases/release-without-theme-J31txIZG`. The six tracked upstream recipes and
hourly timer are unchanged. The preceding ten-package validation is historical.

VM106 was subsequently switched to the normal production URL on 2026-11-06.
All nine packages, live widgets, GUI startup, broad fixture suites, repeat runs
and reboot checks passed. Windows VM installation was excluded; no accounts were
authenticated. See `../nier-deploy/README.md` for the validation scope and evidence.

### Automatic widget activation (source change)

Recipe revision 8 packages the OneDrive activation command and pacman install
script. The updater copies a recipe's optional `<package>.install` companion
beside its rendered PKGBUILD before starting makepkg. Installing or upgrading
`onedrive-quickshell` registers its user plugin and adds the bar widget for the
installing desktop user, preserving account settings and backing up changed
preferences. Authentication and synchronization still require account setup.

Publish a source release containing `nier-onedrive-enable` before deploying
these recipes. Existing updater installations preserve their configuration;
set `recipe_revision` to 8 explicitly when deploying this change. This source
change has not been published or deployed to either VM.

### Monitor arrangement tracking (source change)

The Git remote `gh-personal:0byt3/nier-monitor-arrangement.git` is an SSH alias for
GitHub. Its HTTPS clone URL is
`https://github.com/0byt3/nier-monitor-arrangement.git`; the updater uses the
GitHub HTTPS API and tag archives, without cloning that SSH remote.

The source default configuration includes `nier-monitor-arrangement` at recipe
revision 11. First push the source and publish a stable GitHub release, such as
`v0.1.0`; a tag alone is not a release, and ordinary branch commits do not trigger
builds. Then deploy the updated executable/recipes, rebuild the builder image
(which now includes Lua), add the package to the deployed configuration and
update its recipe revision. Existing configuration is preserved on reinstall.
A missing stable release fails discovery without changing the published pacman
repository, so do not enable this package on VM 107 before that first release.
This change has not yet been deployed or published.

The monitor-arrangement recipe now uses its source-owned
`packaging/install-files.sh` manifest. Revision 11 installs the runtime at
`/usr/local/nier/share/hypr/monitor-arrangement-runtime.lua`; generated user hooks
resolve it through `NIER_PATH`. Publish a release containing the shared manifest
before deploying this recipe revision.
