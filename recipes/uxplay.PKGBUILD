pkgname=uxplay
pkgver={{.Version}}
pkgrel={{.Pkgrel}}
pkgdesc='AirPlay mirroring and audio server'
arch=('x86_64')
url='https://github.com/FDH2/UxPlay'
license=('GPL-3.0-or-later')
depends=('avahi' 'openssl' 'libcrypto.so' 'libplist' 'gstreamer' 'gst-plugins-base'
         'gst-plugins-good' 'gst-plugins-bad' 'gst-libav' 'libx11' 'dbus')
makedepends=('cmake' 'pkgconf')
optdepends=('python: Bluetooth LE discovery beacon'
            'python-psutil: Bluetooth LE discovery beacon'
            'bluez: BlueZ Bluetooth LE beacon'
            'python-gobject: BlueZ Bluetooth LE beacon'
            'python-dbus: BlueZ Bluetooth LE beacon'
            'python-pyserial: optional BleuIO USB beacon'
            'bluez-deprecated-tools: legacy HCI Bluetooth beacon')
options=('!debug')
source=('source.tar.gz')
sha256sums=('{{.SHA256}}')

build() {
  cmake -S "UxPlay-$pkgver" -B build \
    -DCMAKE_BUILD_TYPE=None \
    -DCMAKE_INSTALL_PREFIX=/usr \
    -DCMAKE_POLICY_VERSION_MINIMUM=3.5
  cmake --build build
}

package() {
  DESTDIR="$pkgdir" cmake --install build
}
