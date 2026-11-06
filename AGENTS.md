# Project

Go implementation of an automated Arch pacman repository updater. Bash handles
installation and PKGBUILD recipes. Follow the parent project's Go preference.

## Scope and trusted sources

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
