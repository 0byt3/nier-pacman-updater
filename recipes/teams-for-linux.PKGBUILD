pkgname=teams-for-linux
pkgver={{.Version}}
pkgrel={{.Pkgrel}}
pkgdesc='Unofficial Microsoft Teams client for Linux (official upstream RPM repackaged for Arch)'
arch=('x86_64')
url='https://github.com/IsmaelMartinez/teams-for-linux'
license=('GPL-3.0-or-later')
depends=('alsa-lib' 'at-spi2-core' 'gtk3' 'libnotify' 'libsecret' 'libxss'
         'libxtst' 'mesa' 'nss' 'util-linux-libs' 'xdg-utils')
optdepends=('libappindicator-gtk3: system tray integration'
            'libfido2: FIDO2 security-key support')
options=('!strip' '!debug')
source=('vendor.rpm')
noextract=('vendor.rpm')
sha256sums=('{{.SHA256}}')

prepare() {
  mkdir -p "$srcdir/payload"
  bsdtar -xf "$srcdir/vendor.rpm" -C "$srcdir/payload"
}

package() {
  cp -a "$srcdir/payload/opt" "$pkgdir/"
  cp -a "$srcdir/payload/usr" "$pkgdir/"
  install -d "$pkgdir/usr/bin"
  ln -s /opt/teams-for-linux/teams-for-linux "$pkgdir/usr/bin/teams-for-linux"
}
