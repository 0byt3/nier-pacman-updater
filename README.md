# nier-pacman-updater

A Go service that checks upstream releases, builds signed Arch Linux packages,
and publishes a pacman repository served by Nginx in a rootless Podman container.
Bash handles installation and packaging recipes. It runs on x86_64 Arch Linux;
VM 107 is the deployment/test server.

## Packages and trusted sources

- **`nier-airplay-plugin`**: numeric stable `0byt3/nier-airplay` releases,
  compiled offline with UxPlay and Nier Desktop dependencies and automatic
  widget activation for existing Omarchy users.
- **`nufmt`**: latest numeric stable release of
  [nushell/nufmt](https://github.com/nushell/nufmt), compiled from its HTTPS tag
  archive with the committed Cargo.lock.
- **`uxplay`**: latest numeric stable release of
  [FDH2/UxPlay](https://github.com/FDH2/UxPlay), built from its HTTPS tag archive.
  Experimental master and prereleases are excluded.
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
`nier-monitor-arrangement`, `neovim-nightly`, `powershell`, `uxplay`, `nufmt`, `nier-airplay-plugin`, and recipe revision 16. Reinstallation preserves an existing package selection, so
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
The monitor-arrangement deployment is recorded below.

The monitor-arrangement recipe now uses its source-owned
`packaging/install-files.sh` manifest. Revision 11 installs the runtime at
`/usr/local/nier/share/hypr/monitor-arrangement-runtime.lua`; generated user hooks
resolve it through `NIER_PATH`. Publish a release containing the shared manifest
before deploying this recipe revision.

### Monitor arrangement v0.1.0 deployment (2026-11-07)

Published the initial stable [v0.1.0 source release](https://github.com/0byt3/nier-monitor-arrangement/releases/tag/v0.1.0)
from commit `1112a5e`. VM 107 now runs the updated updater with
`nier-monitor-arrangement` enabled, recipe revision 11, and the Lua-enabled
builder image. The existing package selection, repository signing key, web
service and hourly timer were preserved. The initial targeted run built and
signed `nier-monitor-arrangement 0.1.0-11` from the public GitHub HTTPS tag archive
and atomically published generation `release-1180217378`. Existing packages were
retained in that generation.

The package is available at
`http://10.0.2.36:8080/nier-monitor-arrangement-0.1.0-11-x86_64.pkg.tar.zst`.
Its Apps-menu entry uses the selected desktop-and-laptop SVG icon. Installation
from the HTTP repository passed in a disposable Arch client with
`SigLevel = Required DatabaseRequired`, followed by dependency resolution,
`pacman -Qkk` (38 files, none altered), runtime Lua syntax, desktop-entry
validation, icon equality and launcher checks. Updater and editor Go race tests
and vet passed in the candidate image. Repeat release discovery reported the
package current. No workstation or VM 106 configuration was changed.

Deployment backup:
`/var/backups/nier-pacman-updater/monitor-arrangement-20261107T203127Z/`.
The source/candidate and client validation log are retained under
`/var/lib/nier-pacman-updater/deploy/monitor-v0.1.0-1112a5e/`.
The previous builder image ID and repository generation are recorded in the
backup. Private signing material was not copied into source or build containers.

The deployed recipes for existing packages were deliberately preserved. In
particular, the source-tree OneDrive activation recipe requires helper files not
present in the published OneDrive widget v1.6.0 source release. Deploy that
recipe and its install script only after publishing a compatible OneDrive widget
release; it was not enabled as part of this monitor-editor deployment.

### Monitor arrangement v0.2.0 publication (2026-11-08)

Published [v0.2.0](https://github.com/0byt3/nier-monitor-arrangement/releases/tag/v0.2.0)
from `d226b58`, adding the primary-monitor selector/`NIER_PRIMARY_MONITOR` Lua
global and fixing hidden-window reopening. VM107's existing updater fetched the
HTTPS tag archive, built and signed `nier-monitor-arrangement 0.2.0-11`, and
atomically published generation `release-2337269375`. No updater configuration,
recipe, executable or builder-image change was needed; existing packages remain
in the repository. The source archive SHA-256 is
`dc2335b9c8d5fcf9aa6b481cb17db01da45df143cbe039613bcfa6891e153df5`.

Installation from the HTTP repository passed in a disposable Arch client with
`SigLevel = Required DatabaseRequired`, followed by 38-file integrity, runtime
Lua syntax, desktop-entry, primary-selector and reopen-handler checks. The
minimal image excludes documentation, so its shared empty `/usr/share/doc`
directory was recreated before checking directory metadata; no package-specific
file was changed. Repeat discovery reports the release current, and the hourly
timer and HTTP service remain active/enabled. VM106 was not changed.

Publication backup is retained at
`/var/backups/nier-pacman-updater/monitor-v020-20261108T150446Z/`; validation logs
and publication metadata are under
`/var/lib/nier-pacman-updater/deploy/monitor-v0.2.0-d226b58/`.

## UxPlay

Source defaults now enable UxPlay at recipe revision 12. Existing installations
preserve their package selections: add `uxplay` explicitly, install the updated
recipe/executable, and rebuild the builder image before enabling it.

Checked stable v1.73.7 against upstream CMake files and Arch repository metadata.
Runtime dependencies: avahi, openssl, libplist, gstreamer, gst-plugins-base,
gst-plugins-good, gst-plugins-bad, gst-libav, libx11, dbus (plus transitive
dependencies). Build tools: base-devel, cmake and pkgconf. All are in Arch core
or extra; no Omarchy-only or AUR dependency is needed for normal operation.
The recipe retains upstream X11 and D-Bus support and installs its man pages,
documentation, sample service and optional Bluetooth beacon. Enable avahi-daemon
on the client for normal network discovery; packaging does not enable services.

Optional Bluetooth beacon dependencies python, python-psutil, bluez,
python-gobject, python-dbus and python-pyserial are also in core/extra. BleuIO
needs its separate USB dongle. The alternate legacy HCI beacon needs deprecated
`hcitool` and `hciconfig`; these are supplied by Arch extra
package `bluez-deprecated-tools`. Thus even this optional mode needs no AUR
package. Prefer the supported BlueZ/D-Bus beacon.
Windows winrt bindings are irrelevant to this Linux package.

Validation: Go race tests/vet, Bash syntax and whitespace checks passed.
A disposable VM107 container built uxplay 1.73.7-12 with networking disabled;
a separate disposable client resolved runtime dependencies from Arch, installed
the package, verified 22 files with none missing, and ran `uxplay -h`.
Logs/package remain under `/tmp/nier-uxplay-validation/` on VM107.
The upstream binary embeds a source path (makepkg warns); this does not prevent
packaging. AirPlay playback/Bluetooth pairing were not tested. The live updater,
its configuration, signing state and repository were not changed.

### UxPlay publication (2026-11-10 Pacific / 2026-11-11 UTC)

Deployed source commit `ecf09d8` to VM107, preserving existing deployed recipes
and package selection while adding UxPlay and advancing recipe revision to 12.
The updater rebuilt all eight tracked packages and atomically published signed
generation `release-400859526` at `http://10.0.2.36:8080/`.
UxPlay is available as `uxplay 1.73.7-12`.

All published package signatures and the database signature verified. A disposable
client installed UxPlay over HTTP with package/database signatures required;
`pacman -Qkk` reported 22 files with none altered, `uxplay -h` passed, and
GStreamer H.264/AAC decoder/parser and automatic audio/video sink checks passed.
The client removed the container image's documentation/man-page exclusions before
installation. Repeat discovery found all eight tracked packages current.
The hourly timer and repository service remain active. VM106 was not changed.

Deployment backup and retained validation logs:
`/var/backups/nier-pacman-updater/uxplay-20261111T032458Z/`.
The RAID-backed system disk stalled during packaging but recovered; publication
completed without rebooting or changing storage configuration.

### UxPlay OpenSSL ABI correction

The initial 1.73.7-12 builder retained OpenSSL 4 from its container base even
though the repository/client provided OpenSSL 3. The original client test reused
that newer base and missed the incompatibility. Version 1.73.7-13 corrects this.

The builder now defaults to Omarchy's stable Arch mirror and uses `pacman -Syyuu`
so newer packages inherited from the base image are downgraded to the target
repositories. Override `ARCH_MIRROR` at image build time for another target.
UxPlay also declares `libcrypto.so`, which makepkg resolves to the actual ABI
(`libcrypto.so=3-64` for this build), preventing installation with a missing ABI.

Published signed generation `release-647630407` contains the corrected UxPlay
package; other package versions were retained. The replacement binary was
extracted and checked against the actual workstation: all shared libraries
resolved and its help command passed. Stable-client validation synchronizes and
downgrades the base to Omarchy's stable mirror before installing the package.


## nufmt source builds

The default selection includes `nufmt` at recipe revision 14. Existing deployments
must explicitly add it to their package list and install the updated executable,
recipe and Rust-enabled builder image.

Runtime dependencies are Arch's `glibc` and `gcc-libs`; build dependencies are
`rust` (including Cargo) and base-devel. Nushell's parser libraries are compiled
from crates; installing the separate `nushell` package is not required.

Before the normal offline build, the updater runs only the maintained recipe's
prepare phase in an unprivileged container with pasta networking. Cargo fetches
dependencies using the upstream lockfile and verifies registry checksums.
Only that package's work directory is mounted, with no signing-key mount.
The Cargo cache lives outside srcdir so a clean build preserves downloaded
crates. Compilation and upstream tests use `--frozen` in the normal container
with networking disabled. Fetch/build/test failure prevents signing/publication.

### nufmt publication (2026-11-11 UTC)

VM107 now tracks stable `nushell/nufmt` releases at recipe revision 14.
Published signed `nufmt 0.1.6-14` in generation `release-3776081877`.
Existing deployed recipes and other published package versions were retained.

The candidate and final source builds passed all upstream tests (800 reported
passing cases each). Integration tests expect `target/debug/nufmt` even in
release mode, so the recipe supplies a test-only symlink to the release binary.
Signed installation, 10-file integrity, stdin formatting, idempotence, dry-run
exit codes and workstation library loading passed. The final HTTP client uses
host networking because pasta does not expose the server's own LAN address.
Package/database signatures are required.

Validation build storage is on the existing NVMe mount at
`/var/lib/nier-windows-images/nufmt-validation/`; the deployment run used that
scratch state with the live repo path. Normal deployed state paths were retained.
Signing keys were not copied or mounted into containers. Backup:
`/var/backups/nier-pacman-updater/nufmt-v016/`.
Repeat nufmt discovery is current; the hourly timer and HTTP service are active.
No workstation or VM106 package/configuration changes were made.

## Nier AirPlay release tracking

`nier-airplay-plugin` builds numeric stable releases of `0byt3/nier-airplay`
from HTTPS tag archives. It depends on `uxplay>=1.73.7` and `nier-desktop`.
The package name is `nier-airplay-plugin`; the Omarchy plugin ID is `nier.airplay`.
Installation/upgrade invokes the packaged enable helper as each existing normal
user with `~/.config/omarchy`, using an empty environment and dropping root
privileges before touching user files. The helper backs up changed bar config,
preserves custom plugin paths and avoids duplicate widgets. Logged-out users
may receive a user-bus warning; their configuration is applied for next login.
New users can run `nier-airplay-enable` themselves. The receiver remains
manual-start and installation neither opens firewall ports nor starts playback.
