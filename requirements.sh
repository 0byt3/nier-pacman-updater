#!/usr/bin/env bash
# Install/reinstall the updater on an x86_64 Arch Linux server.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
if (( EUID != 0 )); then exec sudo -- bash "$PWD/requirements.sh" "$@"; fi
[[ $(uname -m) == x86_64 && -f /etc/arch-release ]] || { echo 'Requires x86_64 Arch Linux' >&2; exit 1; }
[[ ${1:-} == '' || ${1:-} == --no-start ]] || { echo 'Usage: requirements.sh [--no-start]' >&2; exit 1; }

# Full upgrade prevents unsupported partial Arch upgrades.
pacman -Syu --noconfirm --needed go podman fuse-overlayfs passt git curl gnupg pacman libarchive rpm-tools jq qemu-img
if ! id nier-updater &>/dev/null; then
  useradd --create-home --home-dir /var/lib/nier-pacman-updater --shell /usr/bin/nologin nier-updater
fi
grep -q '^nier-updater:' /etc/subuid
grep -q '^nier-updater:' /etc/subgid
if ! id nier-repo-web &>/dev/null; then
  useradd --create-home --home-dir /var/lib/nier-pacman-repo --shell /usr/bin/nologin nier-repo-web
fi
grep -q '^nier-repo-web:' /etc/subuid
grep -q '^nier-repo-web:' /etc/subgid
install -d -o nier-repo-web -g nier-repo-web -m 0700 /var/lib/nier-pacman-repo /run/nier-pacman-repo
install -d -o nier-updater -g nier-updater -m 0755 /var/lib/nier-pacman-updater /srv/nier-pacman-repo
install -d -o nier-updater -g nier-updater -m 0755 /srv/nier-pacman-repo/images
install -d -o nier-updater -g nier-updater -m 0700 /var/lib/nier-pacman-updater/{gnupg,tmp} /run/nier-pacman-updater
install -d -o nier-updater -g nier-updater -m 0755 /var/lib/nier-pacman-updater/work
install -d -m 0755 /etc/nier-pacman-updater /usr/share/nier/bin /usr/share/nier/pacman-updater/recipes /usr/local/bin

build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT
GOCACHE="$build_dir/cache" go build -trimpath -o "$build_dir/nier-pacman-updater" .
install -m 0755 "$build_dir/nier-pacman-updater" /usr/share/nier/bin/nier-pacman-updater
ln -sfn /usr/share/nier/bin/nier-pacman-updater /usr/local/bin/nier-pacman-updater
install -m 0644 recipes/*.PKGBUILD /usr/share/nier/pacman-updater/recipes/
install -m 0644 Containerfile /usr/share/nier/pacman-updater/Containerfile
install -Dm 0644 tools/publish-windows-image.sh /usr/share/nier/pacman-updater/tools/publish-windows-image.sh
install -m 0644 nginx/nginx.conf /etc/nier-pacman-updater/nginx.conf
if [[ ! -e /etc/nier-pacman-updater/config.json ]]; then
  install -m 0644 config.json /etc/nier-pacman-updater/config.json
elif jq -e '.recipe_dir == "/usr/local/lib/nier-pacman-updater/recipes"' /etc/nier-pacman-updater/config.json >/dev/null; then
  # Migrate only our previous default; retain custom paths and other settings.
  cp -a /etc/nier-pacman-updater/config.json "$build_dir/config.before.json"
  jq '.recipe_dir = "/usr/share/nier/pacman-updater/recipes" | .recipe_revision = ((.recipe_revision // 0) + 1)' \
    "$build_dir/config.before.json" > "$build_dir/config.json"
  install -m 0644 "$build_dir/config.json" /etc/nier-pacman-updater/config.json
fi

curl --fail --silent --show-error --location \
  https://dl.cloudsmith.io/public/devolutions/rdm/gpg.FE7407ECB26FD2FE.key \
  -o "$build_dir/devolutions.asc"
fingerprint=$(gpg --batch --show-keys --with-colons "$build_dir/devolutions.asc" | awk -F: '$1=="fpr" {print $10; exit}')
[[ $fingerprint == 880E66407780AD737F46CBADFE7407ECB26FD2FE ]] || { echo 'Vendor signing key changed; review it before proceeding.' >&2; exit 1; }
install -m 0644 "$build_dir/devolutions.asc" /etc/nier-pacman-updater/devolutions.asc

as_updater() (
  cd /
  runuser -u nier-updater -- env HOME=/var/lib/nier-pacman-updater \
    XDG_RUNTIME_DIR=/run/nier-pacman-updater \
    TMPDIR=/var/lib/nier-pacman-updater/tmp \
    GNUPGHOME=/var/lib/nier-pacman-updater/gnupg "$@"
)
if ! as_updater gpg --batch --list-secret-keys repo@nier.local &>/dev/null; then
  as_updater gpg --batch --pinentry-mode loopback --passphrase '' \
    --quick-generate-key 'Nier Pacman Repository <repo@nier.local>' ed25519 sign 0
fi
# Automatic signing uses a dedicated unencrypted key, owned only by the service user.
# Do not copy it into the source checkout or build containers.
as_updater podman --cgroup-manager=cgroupfs build --isolation=chroot \
  --tag localhost/nier-pacman-builder:latest \
  --file /usr/share/nier/pacman-updater/Containerfile /usr/share/nier/pacman-updater
as_web() (
  cd /
  runuser -u nier-repo-web -- env HOME=/var/lib/nier-pacman-repo \
    XDG_RUNTIME_DIR=/run/nier-pacman-repo "$@"
)
as_web podman --cgroup-manager=cgroupfs pull docker.io/library/nginx:stable-alpine
as_web podman --cgroup-manager=cgroupfs run --rm --cgroups=disabled --network=none \
  --user 101:101 --cap-drop=all --security-opt=no-new-privileges --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  --volume /etc/nier-pacman-updater/nginx.conf:/etc/nginx/nginx.conf:ro \
  --entrypoint nginx docker.io/library/nginx:stable-alpine -t
install -m 0644 systemd/*.service systemd/*.timer /etc/systemd/system/
systemctl daemon-reload
systemd-analyze verify /etc/systemd/system/nier-pacman-updater.service \
  /etc/systemd/system/nier-pacman-updater.timer /etc/systemd/system/nier-pacman-repo.service
if [[ ${1:-} != --no-start ]]; then
  systemctl enable nier-pacman-repo.service nier-pacman-updater.timer
  systemctl restart nier-pacman-repo.service
  systemctl start nier-pacman-updater.timer
  systemctl start --no-block nier-pacman-updater.service
fi
echo 'Installation complete. Follow builds: journalctl -fu nier-pacman-updater.service'
