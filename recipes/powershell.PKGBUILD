pkgname=powershell
pkgver={{.Version}}
pkgrel={{.Pkgrel}}
pkgdesc='Microsoft PowerShell (official stable Linux x64 binary)'
arch=('x86_64')
url='https://github.com/PowerShell/PowerShell'
license=('MIT')
depends=('glibc' 'libgcc' 'libstdc++' 'icu' 'krb5' 'openssl' 'zlib')
conflicts=('powershell-bin' 'powershell-git')
options=('!strip' '!debug')
source=('source.tar.gz')
noextract=('source.tar.gz')
sha256sums=('{{.SHA256}}')

package() {
  local target="$pkgdir/opt/microsoft/powershell/7"
  install -d "$target" "$pkgdir/usr/bin"
  bsdtar -xf "$srcdir/source.tar.gz" -C "$target"
  chmod 755 "$target/pwsh"
  ln -s /opt/microsoft/powershell/7/pwsh "$pkgdir/usr/bin/pwsh"
  install -Dm644 "$target/LICENSE.txt" "$pkgdir/usr/share/licenses/$pkgname/LICENSE.txt"
  install -Dm644 "$target/ThirdPartyNotices.txt" \
    "$pkgdir/usr/share/licenses/$pkgname/ThirdPartyNotices.txt"
}
