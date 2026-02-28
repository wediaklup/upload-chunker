# Maintainer: wédiaklup <wediaklup@planteam.org>
pkgname=uploadchunker
pkgver=1.0
pkgrel=1
pkgdesc="a simple tool to upload a large volume of files over http"
arch=('any')
url="https://github.com/wediaklup/upload-chunker"
license=('GPL')
depends=()
makedepends=('go')

build() {
	cd ..
	make
	gzip -c ./man/uploadchunkerd.1 > "$srcdir/uploadchunkerd.1.gz"
}

package() {
	cd ..
	install -Dm755 "./uploadchunkerd" "$pkgdir/usr/bin/uploadchunkerd"
	install -Dm644 "$srcdir/uploadchunkerd.1.gz" "$pkgdir/usr/share/man/man1/uploadchunkerd.1.gz"
	make clean
}
