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

manual() {
	echo "cupstui $tag is in $dir, which is not on your PATH."
	echo "Put this line in your shell's startup file, then open a new shell:"
	echo
	echo "  $line"
}

# Printing the line and leaving it at that ends in "command not found", which
# is the whole thing this script exists to avoid, so the line goes in. Set
# CUPSTUI_NO_MODIFY_PATH to be told what to add and add it yourself.
if [ -n "${CUPSTUI_NO_MODIFY_PATH:-}" ]; then
	manual
	exit 0
fi

# Every startup file the user actually has, so a bash user who later opens zsh
# still finds the binary. A file already mentioning the directory is left
# alone: running this script twice should not stack up lines.
written=
for rc in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
	[ -f "$rc" ] || continue
	if grep -qF "$dir" "$rc" 2>/dev/null; then
		written="$written $rc"
		continue
	fi
	printf '\n# added by cupstui install.sh\n%s\n' "$line" >> "$rc" 2>/dev/null &&
		written="$written $rc"
done

# With no startup file at all, ~/.profile is the one every POSIX login shell
# reads.
if [ -z "$written" ] &&
	printf '\n# added by cupstui install.sh\n%s\n' "$line" >> "$HOME/.profile" 2>/dev/null; then
	written=" $HOME/.profile"
fi

if [ -z "$written" ]; then
	manual
	exit 0
fi

echo "cupstui $tag is in $dir, and the PATH line was added to:$written"
echo "Open a new shell, or run this in the current one:"
echo
echo "  $line"
