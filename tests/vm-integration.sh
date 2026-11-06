#!/usr/bin/env bash
# Run on the provisioned Arch VM, after a successful repository publication.
set -euo pipefail
if (( EUID != 0 )); then exec sudo -- bash "$0" "$@"; fi
cd /
repo_url=${1:-http://10.0.2.36:8080}
as_updater() {
  runuser -u nier-updater -- env HOME=/var/lib/nier-pacman-updater \
    XDG_RUNTIME_DIR=/run/nier-pacman-updater \
    GNUPGHOME=/var/lib/nier-pacman-updater/gnupg "$@"
}
fingerprint=$(as_updater gpg --batch --with-colons --fingerprint repo@nier.local | awk -F: '$1=="fpr" {print $10; exit}')
[[ -n $fingerprint ]]
before=$(readlink /srv/nier-pacman-repo/current)
systemctl start nier-pacman-updater.service
[[ $(readlink /srv/nier-pacman-repo/current) == "$before" ]]
echo 'PASS: repeated update is a no-op'

# A real build failure must not change the live database/state.
bad_config=$(mktemp /tmp/nier-failure-config.XXXXXX)
trap 'rm -f "$bad_config"' EXIT
sed 's#localhost/nier-pacman-builder:latest#localhost/nier-intentionally-missing:latest#' \
  /etc/nier-pacman-updater/config.json > "$bad_config"
chmod 644 "$bad_config"
if as_updater /usr/local/bin/nier-pacman-updater -config "$bad_config" \
  -package onedrive-abraunegg -rebuild; then
  echo 'ERROR: intentionally broken build unexpectedly succeeded' >&2; exit 1
fi
[[ $(readlink /srv/nier-pacman-repo/current) == "$before" ]]
echo 'PASS: build failure preserves published repository'

# Bootstrap trust against the locally verified fingerprint, then install through
# pacman with both package and database signatures required. No host packages
# are installed by this client test.
as_updater podman --cgroup-manager=cgroupfs run --rm -i --cgroups=disabled \
  --network=host --user 0 --env "REPO_URL=$repo_url" \
  --env "EXPECTED_FINGERPRINT=$fingerprint" localhost/nier-pacman-builder:latest bash -se <<'CLIENT'
pacman-key --init
pacman-key --populate archlinux
curl -f "$REPO_URL/nier-repo.asc" -o /tmp/nier.asc
actual=$(gpg --batch --show-keys --with-colons /tmp/nier.asc | awk -F: '$1=="fpr" {print $10; exit}')
[[ $actual == "$EXPECTED_FINGERPRINT" ]]
pacman-key --add /tmp/nier.asc
pacman-key --lsign-key "$EXPECTED_FINGERPRINT"
printf '\n[nier]\nSigLevel = Required DatabaseRequired\nServer = %s\n' "$REPO_URL" >> /etc/pacman.conf
pacman -Syu --noconfirm onedrive-abraunegg remote-desktop-manager teams-for-linux xorg-server-xvfb xorg-xauth
pacman -Q onedrive-abraunegg remote-desktop-manager teams-for-linux
onedrive --version
test -x /usr/bin/remotedesktopmanager
test -x /usr/bin/teams-for-linux
runuser -u builder -- teams-for-linux --version
echo 'PASS: signed database and packages accepted, dependencies installed, OneDrive and Teams executables run'

# Check that RDM survives initial GUI startup in a disposable virtual display.
# This does not test licensed features or connections to remote servers.
Xvfb :99 -screen 0 1280x800x24 -nolisten tcp -ac >/tmp/xvfb.log 2>&1 &
sleep 2
set +e
runuser -u builder -- env DISPLAY=:99 dbus-run-session -- timeout 20 remotedesktopmanager >/tmp/rdm.log 2>&1
status=$?
set -e
cat /tmp/rdm.log
[[ $status == 124 ]] || { echo "RDM exited unexpectedly: $status" >&2; exit 1; }
echo 'PASS: RDM survived 20 seconds of headless GUI startup'
CLIENT
