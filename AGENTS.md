# Project

Go implementation of an automated Arch pacman repository updater. Bash handles
installation and PKGBUILD recipes. Follow the parent project's Go preference.

## Scope and trusted sources

- Build `nufmt` from numeric stable `nushell/nufmt` releases. Use the upstream
  Cargo.lock, fetch dependencies separately without signing keys, and compile/test
  offline with Cargo frozen.

- Build `uxplay` from numeric stable `FDH2/UxPlay` GitHub releases using HTTPS tag archives.

- Build `onedrive-abraunegg` from stable `abraunegg/onedrive` GitHub releases.
- Repackage `remote-desktop-manager` from Devolutions' official Cloudsmith `rdm`
  RPM repository; verify the pinned vendor key, signature, and SHA-256 checksum.
- Do not fetch or execute AUR recipes/packages for RDM.
- Build `onedrive-quickshell` from stable `0byt3/onedrive-quickshell` GitHub
  releases. It depends directly on `onedrive-abraunegg>=2.5.12` and uses
  Omarchy's Quickshell components, without requiring Nier Desktop.
- Repackage the exact x86_64 RPM from stable
  `IsmaelMartinez/teams-for-linux` GitHub releases. Require GitHub's SHA-256
  asset digest and a repository-scoped HTTPS download URL.
- Repackage official `neovim/neovim` nightly Linux x86_64 tarballs, requiring
  GitHub asset SHA-256 digests and matching Arch Neovim dependencies.
- Repackage stable `PowerShell/PowerShell` Linux x64 tarballs, requiring
  GitHub asset SHA-256 digests and exact repository release asset URLs.
- Build `nier-monitor-arrangement` from numeric stable releases of
  `0byt3/nier-monitor-arrangement`, using GitHub HTTPS API/archive URLs. The
  standalone editor requires Lua in the builder for profile-decoder tests.
- Do not modify Omarchy VM 106 for updater tests.

## Implementation

- Nier-owned executables live in `/usr/share/nier/bin/`; updater assets live
  in `/usr/share/nier/pacman-updater/`. Preserve the former command entry point
  as a symlink. The installer migrates only the old default recipe path and
  increments recipe revision once, preserving custom paths and other settings.
- OneDrive executables also live in `/usr/share/nier/bin/`, with public command
  symlinks. Widget assets live in `/usr/share/nier/plugins/nier.onedrive/`;
  retain file symlinks in the old asset directory for existing user plugins.

- `main.go`: detection, downloads, builds, signatures, atomic publication.
- `recipes/`: maintained templates; bump deployed `recipe_revision` after changes.
- `requirements.sh`: official Arch dependencies, rootless image, key setup, units.
  Preserve existing configuration and signing keys on repeated installation.
- `systemd/`: hourly checks and rootless Podman Nginx service.
- `nginx/nginx.conf`: serve `/repo/current`; mount the entire repository parent
  read-only so publication symlink changes remain visible. Keep signing keys out
  of this container. `requirements.sh` manages the separate `nier-repo-web` user.
- `tests/nginx.sh`: atomic publication visibility and read-only HTTP checks.
- Keep keys outside build containers and run makepkg unprivileged.
- Validate upstream versions, URLs, checksums before rendering recipes.
- Never advance live state when verification, build, or publication fails.
- Run `go test -race ./...`, `go vet ./...`, `bash -n requirements.sh`, and
  `git diff --check`. Test packaging/container changes with real builds on VM 107.
- Package metadata validation does not prove graphical application compatibility.

## Deployment

VM 107, Proxmox `root@10.0.2.235`, SSH `localadmin@10.0.2.36`, parent project's
`id_eagpve` key, passwordless sudo. IP is DHCP-assigned. State is under
`/var/lib/nier-pacman-updater`, config under `/etc/nier-pacman-updater`, repository
under `/srv/nier-pacman-repo`, HTTP port 8080. See README.md for operation and
known limitations and the parent AGENTS.md for VM details.

## Monitor arrangement deployment (2026-11-07)

- Stable `0byt3/nier-monitor-arrangement` v0.1.0 is published from `1112a5e`.
  VM 107 tracks it over HTTPS at recipe revision 11 with a Lua-enabled builder.
- Signed `nier-monitor-arrangement 0.1.0-11` is published in generation
  `release-1180217378`; the hourly timer and HTTP repository remain active.
  Strict signed-repository client installation and 38-file integrity checks
  passed, including the Apps-menu desktop entry and selected SVG icon.
- Backup: `/var/backups/nier-pacman-updater/monitor-arrangement-20261107T203127Z/`.
  Candidate source/image and client log:
  `/var/lib/nier-pacman-updater/deploy/monitor-v0.1.0-1112a5e/`.
- Existing deployed package recipes were preserved. Do not deploy the new
  OneDrive activation recipe/sidecar against upstream widget v1.6.0: that
  release lacks its new helper files. Publish a compatible widget release first.
- No workstation or VM106 installation/configuration was changed by this deployment.

## Monitor arrangement v0.2.0 publication (2026-11-08)

- Stable v0.2.0 from `d226b58` adds primary-monitor selection and hidden-window
  reopening. The existing VM107 updater published signed `0.2.0-11` in generation
  `release-2337269375`; no updater code, recipes, configuration or image changed.
- Strict signed HTTP client installation passed, with 38 package files unchanged,
  runtime Lua syntax, desktop entry, primary selector and reopen-handler checks.
  The minimal client image required recreating its excluded shared empty
  `/usr/share/doc` directory before checking directory metadata.
- Backup: `/var/backups/nier-pacman-updater/monitor-v020-20261108T150446Z/`.
  Validation logs/state: `/var/lib/nier-pacman-updater/deploy/monitor-v0.2.0-d226b58/`.
  Repeat discovery is current; timer and web service remain active/enabled.

## UxPlay deployment (2026-11-10 Pacific / 2026-11-11 UTC)

- Source commit `ecf09d8` adds stable `FDH2/UxPlay` release tracking.
- VM107 runs the updated executable and expanded rootless builder at recipe
  revision 12, with UxPlay added to its existing seven-package selection.
  Existing deployed recipes were preserved; only the UxPlay recipe was added.
- Published eight rebuilt packages in signed generation `release-400859526`:
  UxPlay 1.73.7-12, OneDrive 2.5.12-12, widget 1.6.0-12, monitor arrangement
  0.2.0-12, PowerShell 7.7.0-12, Teams 2.28.0-12,
  RDM 2026.3.2.1-1.12, Neovim nightly 20261110190723.732606500-12.
- Database/all package signatures verified. Strict signed HTTP client install,
  22-file integrity check, UxPlay help and GStreamer element checks passed.
  Repeat discovery is current; timer and HTTP service remain active.
- Backup and validation logs:
  `/var/backups/nier-pacman-updater/uxplay-20261111T032458Z/`.
  RAID-backed system-disk I/O caused long stalls during Teams packaging;
  the run recovered and completed successfully without reboot or storage changes.
- No workstation/VM106 changes; AirPlay playback and Bluetooth were not tested.

## UxPlay OpenSSL ABI fix (2026-11-11 UTC)

- Initial UxPlay 1.73.7-12 erroneously linked OpenSSL 4 inherited from the
  container base. The workstation uses OpenSSL 3. Reusing the newer container
  base for client validation masked this.
- Builder defaults to `https://stable-mirror.omarchy.org` (override
  `ARCH_MIRROR` for other targets) and runs `pacman -Syyuu` to align base-image
  packages with the target repository, including required downgrades.
- UxPlay now records `libcrypto.so=3-64` via makepkg's SONAME dependency support.
  Signed 1.73.7-13 is published in generation `release-647630407`; other package
  versions were retained. The deployed recipe revision is 13.
- The extracted binary loaded against the workstation's actual libraries and
  its help command passed. ABI-fix backup:
  `/var/backups/nier-pacman-updater/uxplay-abi-fix/`.

## nufmt deployment (2026-11-11 UTC)

- Stable `nushell/nufmt` v0.1.6 is now tracked. VM107 runs the updated updater,
  Rust-enabled Omarchy-stable builder and recipe revision 14.
- Published signed `nufmt 0.1.6-14` in `release-3776081877`; existing deployed
  recipes and other published package versions were retained.
- Cargo fetch uses a separate unprivileged pasta container with only package
  work mounted. Compile/tests remain offline with the lockfile frozen.
  Upstream tests expect a debug binary path: recipe links it to the release binary.
- Candidate and final builds passed upstream tests (800 reported cases each).
  Signed installation/integrity, stdin, idempotence/dry-run and actual workstation
  library checks passed. HTTP client validation uses host networking to reach
  the server's own LAN address; package/database signatures remain required.
- NVMe scratch/candidate state: `/var/lib/nier-windows-images/nufmt-validation/`.
  Normal deployed state paths were preserved. Signing material stays outside
  containers; the scratch keyring path is a host-side symlink to the existing keyring.
- Backup/logs: `/var/backups/nier-pacman-updater/nufmt-v016/`.
  Repeat nufmt discovery is current; timer/web service active. VM106 unchanged.

## VM106 repository client and UxPlay deployment (2026-11-11)

- At the user's explicit request, configured VM106's pacman client with [nier]
  at `http://10.0.2.36:8080`, requiring package/database signatures and the
  pinned repository signing fingerprint. The updater service remains on VM107.
- Direct pacman upgrade was rejected by Omarchy's guard before package changes;
  canonical `omarchy update -y` completed instead, upgrading herdr, mise-bin,
  owe and superwhisper-bin. Then `omarchy pkg add uxplay` installed signed
  `uxplay 1.73.7-14` and its dependencies.
- Integrity: 22 files, none altered; startup help and OpenSSL 3 linkage passed.
  Wayland sink is available. Avahi was already active/enabled; UFW was inactive.
- A transient localadmin user service `nier-uxplay-test.service` is running:
  `uxplay -n "Omarchy VM106" -nh -vs waylandsink`.
  Avahi resolves that AirPlay target on ens18 at 10.0.2.126. The desktop session
  was retained; no reboot or permanent UxPlay autostart was configured.
  Stop with `systemctl --user stop nier-uxplay-test.service`.
- Pacman config backup and deployment/update/verification logs:
  `/var/backups/nier-uxplay/20261111T143800Z/` on VM106.
  No failed system/user units or pending package upgrades were reported.
  Actual client mirroring remains for the user to test.

## Nier AirPlay release tracking and publication (2026-11-11)

- Track numeric stable `0byt3/nier-airplay` GitHub releases as
  `nier-airplay-plugin` (Omarchy ID `nier.airplay`). Build the tag source offline
  with Go, using separate privileged helper executables as upstream does.
- Recipe depends directly on `uxplay>=1.73.7` and `nier-desktop` plus upstream
  runtime dependencies. Revision 16 is newer than the initial GitHub asset's
  pkgrel 15. Other deployed recipes and package selections were preserved.
- VM107 published signed 0.1.0-16 in generation `release-422584700`.
  Repeat discovery reports current. Backup:
  `/var/backups/nier-pacman-updater/airplay-v010/`.
- Pacman install/upgrade sidecar drops privileges with runuser/env -i and calls
  the packaged enable helper for normal users with ~/.config/omarchy. Their
  widget config is backed up/idempotent and custom plugin paths are preserved.
  No receiver start, firewall opening or default login-start is requested.
  New users run nier-airplay-enable themselves. A logged-out user's absent bus
  can produce a warning after the widget files/config have already been applied.
- Go race/vet, stable-release rejection/URL fixtures, Bash syntax and package
  build passed. Strict signed HTTP installation in a disposable Omarchy-stable
  client installed UxPlay 1.73.7-14 and Nier Desktop 3.0.0-20; AirPlay integrity
  reports 45 files, none altered. Automatic widget activation and user ownership
  were observed. Container bootloader/user-bus hook warnings reflect no booted
  desktop in the container. VM106 was not modified.
- Disposable client log: /tmp/nier-airplay-http-client.log on VM107. The reused
  image needed its obsolete openssl-3 compatibility package removed before the
  Omarchy-stable full upgrade to avoid conflicting libcrypto.so.3 ownership.
- Hourly timer and HTTP service restored active after publication.
- Repeat installation, single-widget placement, user-owned created files, static
  receiver and preservation of a custom plugin directory passed in the disposable
  installed client. Repeat log: /tmp/nier-airplay-repeat-client.log on VM107.
  The initial ownership assertion included the root-created fixture parent
  ~/.config; the corrected assertion checks the paths created by activation.
