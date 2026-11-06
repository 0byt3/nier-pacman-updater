pkgname=neovim-nightly
pkgver={{.Version}}
pkgrel={{.Pkgrel}}
pkgdesc='Neovim nightly build (official upstream Linux binary)'
arch=('x86_64')
url='https://neovim.io'
license=('Apache-2.0' 'LicenseRef-vim')
# Match extra/neovim runtime dependencies, checked 2026-11-04.
depends=('glibc' 'libgcc' 'libluv' 'libtree-sitter.so=0.26-64'
         'libunibilium.so=4-64' 'libutf8proc' 'libuv' 'libvterm>=0.3.3'
         'lua51-lpeg' 'luajit' 'msgpack-c' 'tree-sitter' 'tree-sitter-c'
         'tree-sitter-lua' 'tree-sitter-markdown' 'tree-sitter-query'
         'tree-sitter-vim' 'tree-sitter-vimdoc' 'unibilium')
optdepends=('python-pynvim: for Python plugin support (see :help python)'
            'wl-clipboard: for clipboard support on wayland (see :help clipboard)'
            'xclip: for clipboard support on X11 (or xsel) (see :help clipboard)'
            'xsel: for clipboard support on X11 (or xclip) (see :help clipboard)')
# package() sets the actual Neovim API version independently of the date-based pkgver.
provides=('neovim')
conflicts=('neovim')
options=('!strip' '!debug')
source=('source.tar.gz')
sha256sums=('{{.SHA256}}')

package() {
  local upstream_version
  upstream_version=$("$srcdir/nvim-linux-x86_64/bin/nvim" --version) || return 1
  [[ $upstream_version =~ ^NVIM\ v([0-9]+\.[0-9]+\.[0-9]+) ]] || return 1
  # Indexed assignment avoids makepkg's static package-function attribute scan:
  # the version is only available after the verified source is extracted.
  provides[0]="neovim=${BASH_REMATCH[1]}"
  install -d "$pkgdir/usr"
  cp -a "$srcdir/nvim-linux-x86_64/." "$pkgdir/usr/"
  install -Dm644 "$pkgdir/usr/share/nvim/runtime/doc/uganda.txt" \
    "$pkgdir/usr/share/licenses/$pkgname/uganda.txt"
}
