pkgname=remote-desktop-manager
pkgver={{.Version}}
pkgrel={{.Pkgrel}}
pkgdesc='Devolutions Remote Desktop Manager (official vendor RPM repackaged for Arch)'
arch=('x86_64')
url='https://devolutions.net/remote-desktop-manager/'
license=('custom:Proprietary')
depends=('ca-certificates' 'libsecret' 'vte3' 'webkit2gtk-4.1' 'xorg-server-xwayland')
optdepends=('fuse3: RDP file transfers' 'libappindicator-gtk3: tray icon'
            'xdotool: typing macros' 'libfido2: FIDO authentication')
options=('!strip' '!debug')
source=('vendor.rpm')
noextract=('vendor.rpm')
sha256sums=('{{.SHA256}}')

prepare() {
  mkdir -p "$srcdir/payload"
  bsdtar -xf "$srcdir/vendor.rpm" -C "$srcdir/payload"
}

package() {
  cp -a "$srcdir/payload/usr" "$pkgdir/"
  # Preserve vendor-supplied launchers outside /usr, if supplied by the RPM.
  if [[ -d "$srcdir/payload/bin" ]]; then
    install -d "$pkgdir/usr/bin"
    cp -a "$srcdir/payload/bin/." "$pkgdir/usr/bin/"
  fi
}
