# Recording of the demo GIF in the README.
#
#   make demo
#
# Everything on screen belongs to the container the Makefile starts from
# scripts/demo.Containerfile: its printers, its home directory, its page log.
# The machine doing the recording never appears in the frame.

Output docs/demo.gif

Set Shell bash
Set FontSize 15
Set Width 1140
Set Height 520
Set Padding 22
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

Hide
Type "podman exec -it -e COLORTERM=truecolor cupstui-demo bash" Enter
Sleep 2s
Type "PS1='$ '" Enter
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

# Cancel one job, with the confirmation.
Type "j"
Sleep 500ms
Type "x"
Sleep 1500ms
Type "y"
Sleep 2s

# Release another, which then prints and leaves the queue.
Type "p"
Sleep 3s

# Printers, and the quota editor.
Type "3"
Sleep 1500ms
Type "j"
Sleep 1s
Type "u"
Sleep 2500ms
Escape
Sleep 500ms

# Diagnosing a printer.
Type "i"
Sleep 2500ms
Escape
Sleep 500ms

# Sending a file. The first escape closes the browser, the second takes the
# focus off the file field so the tab keys are global again.
Type "4"
Sleep 2s
Ctrl+o
Sleep 2500ms
Escape
Sleep 750ms
Escape
Sleep 500ms

# What has been printed, and by whom.
Type "5"
Sleep 2s
Type "s"
Sleep 3s

# The logs. cupsd repeats itself for pages at a time, so a run of the same
# message is folded into one line with a count; v raises the level below which
# nothing is shown, and n cycles to the next file.
Type "6"
Sleep 3s
Type "v"
Sleep 2s
Type "v"
Sleep 2500ms
Type "n"
Sleep 3s

Type "q"
Sleep 1s
