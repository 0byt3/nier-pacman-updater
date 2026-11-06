#!/usr/bin/env bash
# Validate the Nginx container and publication visibility without changing the live repo.
set -euo pipefail
if (( EUID != 0 )); then exec sudo -- bash "$0" "$@"; fi
cd /
as_web() {
  runuser -u nier-repo-web -- env HOME=/var/lib/nier-pacman-repo \
    XDG_RUNTIME_DIR=/run/nier-pacman-repo "$@"
}
container=nier-repo-nginx-test
fixture=$(mktemp -d /var/lib/nier-pacman-repo/nginx-test.XXXXXX)
cleanup() {
  as_web podman --cgroup-manager=cgroupfs rm -f --ignore "$container" >/dev/null
  rm -rf "$fixture"
}
trap cleanup EXIT
chmod 755 "$fixture"
mkdir -p "$fixture/repo/releases/first" "$fixture/repo/releases/second"
mkdir -p "$fixture/repo/images/windows11-pro/releases/test"
printf 'first-image\n' > "$fixture/repo/images/windows11-pro/releases/test/windows11-pro.qcow2"
printf 'first\n' > "$fixture/repo/releases/first/probe.txt"
printf 'second\n' > "$fixture/repo/releases/second/probe.txt"
ln -s releases/first "$fixture/repo/current"
sed 's/8080/18080/g' /etc/nier-pacman-updater/nginx.conf > "$fixture/nginx.conf"
as_web podman --cgroup-manager=cgroupfs run -d --name "$container" --cgroups=disabled \
  --network=host --user 101:101 --cap-drop=all --security-opt=no-new-privileges \
  --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  --volume "$fixture/repo:/repo:ro" --volume "$fixture/nginx.conf:/etc/nginx/nginx.conf:ro" \
  --entrypoint nginx docker.io/library/nginx:stable-alpine -g 'daemon off;'
for attempt in {1..20}; do
  if curl -fsS http://127.0.0.1:18080/probe.txt > "$fixture/response"; then break; fi
  sleep 0.2
done
[[ $(cat "$fixture/response") == first ]]
ln -s releases/second "$fixture/repo/.next"
mv -Tf "$fixture/repo/.next" "$fixture/repo/current"
[[ $(curl -fsS http://127.0.0.1:18080/probe.txt) == second ]]
[[ $(curl -fsS --range 0-4 http://127.0.0.1:18080/images/windows11-pro/releases/test/windows11-pro.qcow2) == first ]]
[[ $(curl -s -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:18080/images/windows11-pro/releases/test/windows11-pro.qcow2) == 403 ]]
curl -fsSI http://127.0.0.1:18080/probe.txt | grep -qi 'cache-control: no-cache'
[[ $(curl -s -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:18080/probe.txt) == 403 ]]
if as_web podman --cgroup-manager=cgroupfs exec "$container" touch /repo/write-test; then
  echo 'ERROR: repository mount is writable' >&2; exit 1
fi
echo 'PASS: independent image hosting, HTTP ranges, atomic repo visibility and read-only mounts'
