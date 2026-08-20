#!/bin/sh
# Installs the latest cupstui release for this machine.
#
#   curl -fsSL https://raw.githubusercontent.com/icortesb/cupstui/main/scripts/install.sh | sh
#
# CUPSTUI_INSTALL_DIR chooses where it lands (~/.local/bin by default) and
# CUPSTUI_VERSION pins a tag instead of taking the latest one.
set -eu

repo=icortesb/cupstui
dir=${CUPSTUI_INSTALL_DIR:-${HOME:?HOME is not set; set CUPSTUI_INSTALL_DIR instead}/.local/bin}

die() { echo "install.sh: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "needs $1"; }

need curl
need tar
need sha256sum

[ "$(uname -s)" = Linux ] || die "cupstui runs on Linux; CUPS is the whole point"

case $(uname -m) in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	armv7l | armv7 | armhf) arch=armv7 ;;
	*) die "no release build for $(uname -m); build it from source with 'go install github.com/$repo/cmd/cupstui@latest'" ;;
esac

# The releases/latest page redirects to the newest tag, which is one request
# and does not spend the unauthenticated API budget.
if [ -n "${CUPSTUI_VERSION:-}" ]; then
	tag=$CUPSTUI_VERSION
else
	url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest") ||
		die "cannot reach github.com"
	tag=${url##*/}
fi
case $tag in
	v*) ;;
	*) tag=v$tag ;;
esac

asset="cupstui_${tag#v}_linux_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

echo "downloading cupstui $tag ($arch)"
curl -fsSL -o "$tmp/$asset" "$base/$asset" || die "no $asset in release $tag"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || die "release $tag publishes no checksums"

# Only the line for this archive, so an unrelated missing file is not a failure.
grep " \*\{0,1\}$asset\$" "$tmp/checksums.txt" > "$tmp/want" || die "$asset is not in checksums.txt"
(cd "$tmp" && sha256sum -c want >/dev/null) || die "$asset does not match its published checksum"

tar -xzf "$tmp/$asset" -C "$tmp" cupstui || die "no cupstui binary inside $asset"

mkdir -p "$dir" || die "cannot create $dir"
cp "$tmp/cupstui" "$dir/cupstui.new" || die "cannot write to $dir"
chmod 755 "$dir/cupstui.new"
# Replacing the binary in one step, so a running cupstui is never half-written.
mv "$dir/cupstui.new" "$dir/cupstui"

case ":$PATH:" in
	*":$dir:"*)
		echo "cupstui $tag is in $dir. Run it: cupstui"
		exit 0
		;;
esac

line="export PATH=\"$dir:\$PATH\""
conf=${XDG_CONFIG_HOME:-$HOME/.config}

manual() {
	echo "cupstui $tag is in $dir, which is not on your PATH."
	echo "Put this line in your shell's startup file, then open a new shell:"
	echo
	echo "  ${1:-$line}"
}

# Printing the line and leaving it at that ends in "command not found", which
# is the whole thing this script exists to avoid, so the line goes in. Set
# CUPSTUI_NO_MODIFY_PATH to be told what to add and add it yourself.
if [ -n "${CUPSTUI_NO_MODIFY_PATH:-}" ]; then
	manual
	exit 0
fi

# startup_file and path_line say where a shell reads its configuration from and
# how it spells "put this directory first". fish keeps a command for it, and
# nushell's PATH is a list rather than a string, so neither takes the export
# line the Bourne shells do.
startup_file() {
	case $1 in
		*/fish) echo "$conf/fish/config.fish" ;;
		*/nu) echo "$conf/nushell/env.nu" ;;
		*csh) echo "$HOME/.tcshrc" ;;
		*/zsh) echo "${ZDOTDIR:-$HOME}/.zshrc" ;;
		*/bash) echo "$HOME/.bashrc" ;;
		*) echo "$HOME/.profile" ;;
	esac
}

path_line() {
	case $1 in
		*/fish) echo "fish_add_path $dir" ;;
		*/nu) echo "\$env.PATH = (\$env.PATH | prepend '$dir')" ;;
		*csh) echo "setenv PATH \"$dir:\${PATH}\"" ;;
		*) echo "$line" ;;
	esac
}

# A file already naming the directory is left alone, so running this twice does
# not stack up a second copy.
add_shell() {
	if grep -qF "$dir" "$1" 2>/dev/null; then
		written="$written $1"
		return 0
	fi
	mkdir -p "${1%/*}" 2>/dev/null || return 1
	printf '\n# added by cupstui install.sh\n%s\n' "$2" >> "$1" 2>/dev/null || return 1
	written="$written $1"
}

written=
# The login shell's own file is written whether or not it exists yet: a zsh
# user who has never made a ~/.zshrc still needs one for this.
own=$(startup_file "${SHELL:-/bin/sh}")
ownline=$(path_line "${SHELL:-/bin/sh}")
add_shell "$own" "$ownline" || :

# The files of the other shells are only appended to when they already exist,
# so someone who moves between shells keeps finding the binary, while nobody
# grows a configuration for a shell they never open.
for other in /bin/bash /bin/zsh /bin/fish /bin/nu /bin/csh /bin/sh; do
	rc=$(startup_file "$other")
	[ "$rc" = "$own" ] && continue
	[ -f "$rc" ] || continue
	add_shell "$rc" "$(path_line "$other")" || :
done

if [ -z "$written" ]; then
	manual "$ownline"
	exit 0
fi

echo "cupstui $tag is in $dir, and the PATH line was added to:$written"
echo "Open a new shell, or run this in the current one:"
echo
echo "  $ownline"
