# Recording of the demo GIF in the README.
#
#   make demo
#
# Needs vhs, which pulls in ttyd and ffmpeg. The queue is filled with held
# jobs so nothing reaches paper, and emptied again at the end. The
# configuration file is put back the way it was found.

Output docs/demo.gif

Set Shell zsh
Set FontSize 15
Set Width 1180
Set Height 660
Set Padding 22
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

# Setup, off camera: three jobs that will never print, and a configuration
# that skips the first-run checks so the recording opens on the interface.
Hide
Type "mkdir -p /tmp/cupstui-demo && cd /tmp/cupstui-demo" Enter
Type `printf 'annual report\n' > 'annual report.pdf'` Enter
Type `printf 'invoice\n' > invoice.pdf` Enter
Type `printf 'receipt\n' > receipt.pdf` Enter
Type `for f in 'annual report.pdf' invoice.pdf receipt.pdf; do lp -H hold -t "$f" "$f" >/dev/null; done` Enter
Type "mkdir -p ~/.config/cupstui" Enter
Type "cp ~/.config/cupstui/config.json /tmp/cupstui-demo/config.bak 2>/dev/null" Enter
Type `printf '{"seen":true}' > ~/.config/cupstui/config.json` Enter
Type "clear" Enter
Show

Type "cupstui" Enter
Sleep 3s

# The queue, with its count on the tab.
Type "2"
Sleep 2s

# Scoped filtering.
Type "/"
Sleep 500ms
Type "document:invoice"
Sleep 2s
Escape
Sleep 1s

# Hold and release one job.
Type "j"
Sleep 500ms
Type "p"
Sleep 2s

# Cancel one, with the confirmation.
Type "x"
Sleep 1500ms
Type "y"
Sleep 2s

# Printers, and the quota editor.
Type "3"
Sleep 2s
Type "u"
Sleep 2500ms
Escape
Sleep 500ms

# Sending a file.
Type "4"
Sleep 2s
Ctrl+o
Sleep 2500ms
Escape
Sleep 500ms

# What has been printed, and by whom.
Type "5"
Sleep 2s
Type "s"
Sleep 3s

# The logs, tinted by level.
Type "6"
Sleep 2500ms

Type "q"
Sleep 1s

# Teardown, off camera.
Hide
Type "cancel -a >/dev/null 2>&1" Enter
Type "cp /tmp/cupstui-demo/config.bak ~/.config/cupstui/config.json 2>/dev/null || rm -f ~/.config/cupstui/config.json" Enter
Type "rm -rf /tmp/cupstui-demo" Enter
