# AUR packaging

`PKGBUILD` builds cupstui from a tagged source tarball.

Before publishing a new version, set `pkgver` to the tag, then regenerate the
checksums and the `.SRCINFO` the AUR reads:

```sh
updpkgsums
makepkg --printsrcinfo > .SRCINFO
```

`sha256sums` is `SKIP` here so the file is useful before a tag exists;
`updpkgsums` replaces it with the real digest.

Build and install locally to check it:

```sh
makepkg -si
```
