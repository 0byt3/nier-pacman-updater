pkgname=nufmt
pkgver={{.Version}}
pkgrel={{.Pkgrel}}
pkgdesc="An AST-based formatter for Nushell scripts"
arch=('x86_64')
url='https://github.com/nushell/nufmt'
license=('MIT')
depends=('gcc-libs' 'glibc')
makedepends=('rust')
options=('!debug')
source=('source.tar.gz')
sha256sums=('{{.SHA256}}')

# Keep downloaded crates outside srcdir: the offline clean build recreates it.
export CARGO_HOME="$startdir/cargo-home"
export CARGO_BUILD_JOBS=3

prepare() {
  cd "nufmt-$pkgver"
  cargo fetch --locked
}

build() {
  cd "nufmt-$pkgver"
  cargo build --release --frozen
}

check() {
  cd "nufmt-$pkgver"
  # Upstream integration tests look in target/debug even for release tests.
  mkdir -p target/debug
  ln -sfn ../release/nufmt target/debug/nufmt
  cargo test --release --frozen
}

package() {
  cd "nufmt-$pkgver"
  install -Dm755 target/release/nufmt "$pkgdir/usr/bin/nufmt"
  install -Dm644 LICENSE "$pkgdir/usr/share/licenses/nufmt/LICENSE"
  install -Dm644 README.md "$pkgdir/usr/share/doc/nufmt/README.md"
}
