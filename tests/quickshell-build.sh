#!/usr/bin/env bash
# Run on VM 107. Build a local plugin checkout without changing the live repo.
set -euo pipefail
if (( EUID != 0 )); then exec sudo -- bash "$0" "$@"; fi
updater_source=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
plugin_source=$(realpath "${1:-$updater_source/../onedrive-quickshell}")
version=$(sed -n 's/^const version = "\([0-9.]*\)"$/\1/p' "$plugin_source/shared.go")
[[ $version =~ ^[0-9]+(\.[0-9]+)*$ ]]
work=$(mktemp -d /var/lib/nier-pacman-updater/work/quickshell-validation.XXXXXX)
printf 'Validation work retained at %s\n' "$work"
# Follow the release archive's top-level directory convention.
mkdir "$work/onedrive-quickshell-$version"
tar -C "$plugin_source" --exclude=.git --exclude=target --exclude=src --exclude=pkg \
  --exclude='*.pkg.tar.*' --exclude='./nier-onedrive-monitor' --exclude='./nier-onedrive-setup' \
  -cf - . | tar -C "$work/onedrive-quickshell-$version" -xf -
tar -C "$work" -czf "$work/source.tar.gz" "onedrive-quickshell-$version"
sha=$(sha256sum "$work/source.tar.gz")
sha=${sha%% *}
sed -e "s/{{.Version}}/$version/g" -e 's/{{.Pkgrel}}/3/g' -e "s/{{.SHA256}}/$sha/g" \
  "$updater_source/recipes/onedrive-quickshell.PKGBUILD" > "$work/PKGBUILD"
chown -R nier-updater:nier-updater "$work"
cd /
as_updater() {
  runuser -u nier-updater -- env HOME=/var/lib/nier-pacman-updater \
    XDG_RUNTIME_DIR=/run/nier-pacman-updater "$@"
}
image=localhost/nier-pacman-builder-quickshell-test:latest
install -d -o nier-updater -g nier-updater "$work/image-context"
install -m 0644 "$updater_source/Containerfile" "$work/image-context/Containerfile"
as_updater podman --cgroup-manager=cgroupfs build --isolation=chroot \
  --tag "$image" --file "$work/image-context/Containerfile" "$work/image-context"
as_updater podman --cgroup-manager=cgroupfs run --rm --cgroups=disabled \
  --userns=keep-id:uid=1000,gid=1000 --network=none \
  --volume "$work:/work:rw" --workdir /work "$image" \
  makepkg --nodeps --cleanbuild --clean --noconfirm
packages=("$work"/*.pkg.tar.zst)
(( ${#packages[@]} == 1 ))
bsdtar -xOf "${packages[0]}" .PKGINFO > "$work/PKGINFO"
grep -Fx 'pkgname = onedrive-quickshell' "$work/PKGINFO"
grep -Fx "pkgver = $version-3" "$work/PKGINFO"
grep -Fx 'depend = onedrive-abraunegg>=2.5.12' "$work/PKGINFO"
as_updater podman --cgroup-manager=cgroupfs run --rm -i --cgroups=disabled \
  --user 0 --network=none --volume "$work:/work:ro" "$image" bash -se <<'CLIENT'
pacman -Udd --noconfirm /work/*.pkg.tar.zst
runuser -u builder -- nier-onedrive-setup --version
runuser -u builder -- nier-onedrive-setup --list --json
runuser -u builder -- nier-onedrive-monitor --list --json
test -f /usr/share/onedrive-quickshell/manifest.json
test -f /usr/share/nier/plugins/nier.onedrive/manifest.json
test "$(readlink /usr/bin/nier-onedrive-setup)" = /usr/share/nier/bin/nier-onedrive-setup
test "$(readlink /usr/share/onedrive-quickshell/manifest.json)" = /usr/share/nier/plugins/nier.onedrive/manifest.json
test -f /usr/lib/systemd/user/nier-onedrive@.service
test -f /usr/lib/systemd/user/onedrive.service.d/50-nier-monitor.conf
CLIENT
echo 'PASS: offline build, tests, direct client dependency, installed helpers and assets'
echo 'GUI loading is checked separately on an Omarchy desktop; dependency installation is skipped in this isolated Arch container.'
