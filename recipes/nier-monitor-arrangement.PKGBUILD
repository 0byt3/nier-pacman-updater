pkgname=nier-monitor-arrangement
pkgver={{.Version}}
pkgrel={{.Pkgrel}}
pkgdesc='Standalone Quickshell display arrangement editor with saved Hyprland profiles'
arch=('x86_64')
url='https://github.com/0byt3/nier-monitor-arrangement'
license=('MIT')
depends=('bash' 'quickshell' 'hyprland>=0.55' 'lua' 'qt6-declarative')
makedepends=('go')
options=('!debug')
source=('source.tar.gz')
sha256sums=('{{.SHA256}}')

build() {
  cd "$srcdir"/nier-monitor-arrangement-*
  export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0
  bash bin/build
}

check() {
  cd "$srcdir"/nier-monitor-arrangement-*
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0 go test ./...
}

package() {
  cd "$srcdir"/nier-monitor-arrangement-*
  bash packaging/install-files.sh "$pkgdir"
}
