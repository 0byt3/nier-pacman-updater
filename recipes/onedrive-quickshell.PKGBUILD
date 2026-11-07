pkgname=onedrive-quickshell
pkgver={{.Version}}
pkgrel={{.Pkgrel}}
pkgdesc='OneDrive account setup and status widget for the Omarchy Quickshell bar'
arch=('x86_64')
url='https://github.com/0byt3/onedrive-quickshell'
license=('GPL-3.0-only' 'MIT')
depends=('onedrive-abraunegg>=2.5.12' 'omarchy>=4.0.6' 'quickshell' 'systemd' 'xdg-utils' 'jq' 'util-linux')
makedepends=('go')
options=('!debug')
install=onedrive-quickshell.install
source=('source.tar.gz')
sha256sums=('{{.SHA256}}')

build() {
  cd "$srcdir"/onedrive-quickshell-*
  export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0
  for program in nier-onedrive-monitor nier-onedrive-setup; do
    go build -mod=vendor -trimpath -buildmode=pie \
      -ldflags "-X main.program=$program" -o "$program" .
  done
}

check() {
  cd "$srcdir"/onedrive-quickshell-*
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0 go test -mod=vendor ./...
}

package() {
  cd "$srcdir"/onedrive-quickshell-*
  install -Dm755 nier-onedrive-enable "$pkgdir/usr/share/nier/bin/nier-onedrive-enable"
  install -d "$pkgdir/usr/bin"
  ln -s /usr/share/nier/bin/nier-onedrive-enable "$pkgdir/usr/bin/nier-onedrive-enable"
  for program in nier-onedrive-monitor nier-onedrive-setup; do
    install -Dm755 "$program" "$pkgdir/usr/share/nier/bin/$program"
    install -d "$pkgdir/usr/bin"
    ln -s "/usr/share/nier/bin/$program" "$pkgdir/usr/bin/$program"
  done
  install -d "$pkgdir/usr/share/nier/plugins/nier.onedrive"
  install -m644 manifest.json Panel.qml Service.qml Model.js onedrive.svg \
    onedrive-offline.svg "$pkgdir/usr/share/nier/plugins/nier.onedrive/"
  # Existing Omarchy user plugin links still target this legacy directory.
  install -d "$pkgdir/usr/share/onedrive-quickshell"
  local file
  for file in manifest.json Panel.qml Service.qml Model.js onedrive.svg onedrive-offline.svg; do
    ln -s "/usr/share/nier/plugins/nier.onedrive/$file" "$pkgdir/usr/share/onedrive-quickshell/$file"
  done
  install -Dm644 50-nier-monitor.conf \
    "$pkgdir/usr/lib/systemd/user/onedrive.service.d/50-nier-monitor.conf"
  install -Dm644 nier-onedrive@.service "$pkgdir/usr/lib/systemd/user/nier-onedrive@.service"
  install -Dm644 vendor/github.com/pelletier/go-toml/v2/LICENSE \
    "$pkgdir/usr/share/licenses/$pkgname/go-toml-LICENSE"
  install -Dm644 README.md "$pkgdir/usr/share/doc/$pkgname/README.md"
}
