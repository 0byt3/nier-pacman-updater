pkgname=onedrive-abraunegg
pkgver={{.Version}}
pkgrel={{.Pkgrel}}
pkgdesc='OneDrive client for Linux, built from abraunegg stable releases'
arch=('x86_64')
url='https://github.com/abraunegg/onedrive'
license=('GPL-3.0-or-later')
depends=('curl' 'sqlite' 'libnotify' 'liblphobos')
makedepends=('ldc' 'pkgconf')
provides=("onedrive=$pkgver")
conflicts=('onedrive' 'onedrive-abraunegg-git')
backup=('etc/logrotate.d/onedrive')
options=('!debug')
source=('source.tar.gz')
sha256sums=('{{.SHA256}}')

build() {
  cd "$srcdir"/onedrive-*
  ./configure --prefix=/usr --sysconfdir=/etc \
    --with-systemdsystemunitdir=/usr/lib/systemd/system \
    --with-systemduserunitdir=/usr/lib/systemd/user
  make
}

package() {
  cd "$srcdir"/onedrive-*
  make DESTDIR="$pkgdir" install
}
