pkgname=nier-airplay-plugin
pkgver={{.Version}}
pkgrel={{.Pkgrel}}
pkgdesc='Omarchy AirPlay receiver controls and connected-client status powered by UxPlay'
arch=('x86_64')
url='https://github.com/0byt3/nier-airplay'
license=('MIT' 'CC0-1.0')
depends=('nier-desktop' 'omarchy>=4.5.0' 'quickshell' 'uxplay>=1.73.7' 'avahi' 'systemd' 'coreutils' 'bash' 'jq' 'util-linux' 'ufw' 'sudo' 'polkit' 'nftables' 'iptables-nft' 'xorg-xwayland')
makedepends=('go')
options=('!debug')
install=nier-airplay-plugin.install
source=('source.tar.gz')
sha256sums=('{{.SHA256}}')
build() {
  cd "$srcdir"/nier-airplay-*
  CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go build -trimpath -ldflags='-s -w -buildid=' -o nier-airplay-session .
  CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go build -trimpath -ldflags='-s -w -buildid= -X main.commandRole=firewall-remove' -o nier-airplay-firewall-remove .
  CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go build -trimpath -ldflags='-s -w -buildid= -X main.commandRole=firewall-setup' -o nier-airplay-firewall-setup .
  CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go build -trimpath -ldflags='-s -w -buildid= -X main.commandRole=firewall-check' -o nier-airplay-firewall-check .
}
check() {
  cd "$srcdir"/nier-airplay-*
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./...
}
package() {
  cd "$srcdir"/nier-airplay-*
  install -Dm755 nier-airplay-session "$pkgdir/usr/share/nier/bin/nier-airplay-session"
  install -Dm755 nier-airplay-enable "$pkgdir/usr/share/nier/bin/nier-airplay-enable"
  install -d "$pkgdir/usr/bin"
  for tool in nier-airplay-firewall-setup nier-airplay-firewall-check; do
    install -Dm755 "$tool" "$pkgdir/usr/share/nier/bin/$tool"
    ln -s "/usr/share/nier/bin/$tool" "$pkgdir/usr/bin/$tool"
  done
  install -Dm644 org.nier.airplay.firewall.policy "$pkgdir/usr/share/polkit-1/actions/org.nier.airplay.firewall.policy"
  install -Dm755 nier-airplay-firewall-remove "$pkgdir/usr/share/nier/bin/nier-airplay-firewall-remove"
  ln -s /usr/share/nier/bin/nier-airplay-firewall-remove "$pkgdir/usr/bin/nier-airplay-firewall-remove"
  install -Dm644 nier-airplay-firewall-cleanup.service "$pkgdir/usr/lib/systemd/system/nier-airplay-firewall-cleanup.service"
  install -d -m750 "$pkgdir/etc/sudoers.d"
  install -Dm440 nier-airplay-firewall "$pkgdir/etc/sudoers.d/nier-airplay-firewall"
  ln -s nier-airplay-session "$pkgdir/usr/share/nier/bin/nier-airplay-ports"
  ln -s /usr/share/nier/bin/nier-airplay-ports "$pkgdir/usr/bin/nier-airplay-ports"
  ln -s /usr/share/nier/bin/nier-airplay-enable "$pkgdir/usr/bin/nier-airplay-enable"
  install -Dm644 nier-airplay.service "$pkgdir/usr/lib/systemd/user/nier-airplay.service"
  install -d "$pkgdir/usr/share/nier/plugins/nier.airplay/i18n"
  install -m644 AirPlay.qml Service.qml Model.js manifest.json apple.svg "$pkgdir/usr/share/nier/plugins/nier.airplay/"
  install -m644 en.json "$pkgdir/usr/share/nier/plugins/nier.airplay/i18n/"
  install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
  install -Dm644 THIRD_PARTY "$pkgdir/usr/share/licenses/$pkgname/THIRD_PARTY"
  install -Dm644 README.md "$pkgdir/usr/share/doc/$pkgname/README.md"
}
