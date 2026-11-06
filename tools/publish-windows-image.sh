#!/usr/bin/env bash
# Maintainer tool: publish an already sanitized, generalized and checked image.
set -euo pipefail
umask 022
if (( $# != 3 )); then echo "Usage: $0 IMAGE.qcow2 VERSION BUILD-REPORT.json" >&2; exit 2; fi
image=$(realpath -e -- "$1") version=$2 report=$(realpath -e -- "$3")
[[ $version =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]{0,79}$ ]] || { echo 'Invalid version' >&2; exit 1; }
# Execute as nier-updater, with read access to the sanitized input files.
[[ $(id -un) == nier-updater ]] || { echo 'Run this tool as nier-updater.' >&2; exit 1; }
base=${NIER_WINDOWS_IMAGE_DIR:-/srv/nier-pacman-repo/images/windows11-pro}
[[ $base == /srv/nier-pacman-repo/images/* ]] || { echo 'Image channel must be under the served images directory.' >&2; exit 1; }
keyhome=/var/lib/nier-pacman-updater/gnupg
fingerprint=D08DCD6B97C9885E62968C927DEEC45C73A8FB34
info=$(qemu-img info --output=json "$image")
jq -e '.format == "qcow2" and (."backing-filename" == null) and (.snapshots // [] | length == 0) and (.encrypted // false | not)' <<< "$info" >/dev/null
qemu-img check -q "$image"
jq -e '.updates.phase == "complete" and .generalized == true and .sanitized == true and .offline_answer_cache_cleared == true and .answer_file_path == "D:\\Autounattend.xml" and .edition == "Professional"' "$report" >/dev/null
mkdir -p "$base/releases"
exec 9>"$base/.publish.lock"
flock 9
[[ ! -e $base/releases/$version ]] || { echo 'Version already published' >&2; exit 1; }
stage=$(mktemp -d "$base/releases/.staging.XXXXXXXX")
trap 'rm -rf -- "$stage"' EXIT
cp --sparse=always -- "$image" "$stage/windows11-pro.qcow2"
chmod 644 "$stage/windows11-pro.qcow2"
sha=$(sha256sum "$stage/windows11-pro.qcow2"); sha=${sha%% *}
bytes=$(stat -c %s "$stage/windows11-pro.qcow2")
virtual=$(jq -r '."virtual-size"' <<< "$info")
expanded=$(qemu-img measure --output=json -O qcow2 "$image" | jq -r .required)
jq -n --arg version "$version" --arg file "releases/$version/windows11-pro.qcow2" --arg sha "$sha" \
 --argjson bytes "$bytes" --argjson expanded "$expanded" --argjson virtual "$virtual" --slurpfile report "$report" \
 '{schema:1,os:"windows11",edition:"Professional",architecture:"amd64",version:$version,file:$file,sha256:$sha,bytes:$bytes,virtual_bytes:$virtual,expanded_bytes:$expanded,languages:["en-US"],build:$report[0]}' > "$stage/manifest.json"
gpg --homedir "$keyhome" --batch --yes --local-user "$fingerprint" --digest-algo SHA256 \
 --output "$stage/manifest.json.asc" --clearsign "$stage/manifest.json"
chmod 755 "$stage"
mv -- "$stage" "$base/releases/$version"
ln -s "releases/$version/manifest.json.asc" "$base/.latest.$$"
mv -Tf "$base/.latest.$$" "$base/latest.json.asc"
trap - EXIT
printf 'Published %s/latest.json.asc -> %s (%s bytes)\n' "${base#/srv/nier-pacman-repo}" "$version" "$bytes"
