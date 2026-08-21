# Changelog

## v0.1.14 — 2026-08-21

- `A` adds a printer on a server that has none. The Printers tab required a
  selected printer before it read the key, so on an empty list the shortcut
  the footer advertised did nothing — on the one kind of machine where adding
  a printer is the only thing left to do.
- Both empty states say where the add is: the Printers tab names the key, and
  the Dashboard, which has no add of its own, points at the Printers tab.

## v0.1.13 — 2026-08-20

- The tab content sits in a panel. It ran edge to edge with nothing but the
  rule under the header between it and the navigation, and a screen with no
  shape to it is harder to read at a glance than one with a frame.
- The panel's border carries the focus. While a filter is being typed the keys
  go to the field rather than the list underneath, and until now only the
  cursor in the field said so.
- Column headers on the Queue and the History take the accent colour, which is
  what makes a row of names read as a header rather than as another row.
- The demo recording moved into a container of its own. The GIF on the README
  was recorded against a real machine and showed its printer by hostname, its
  owner on every job, and the whole of their home directory in the file
  browser. Nothing in the new one belongs to anybody: `make demo` now wants
  podman instead of a running CUPS.

## v0.1.12 — 2026-08-20

- The Logs tab folds a run of the same message into one line with a count.
  cupsd writes "Local authentication certificate not found." once per client
  connection, so a screen of the log was eighty-five copies of it and the one
  line that mattered — an internal error adding a printer — was somewhere
  above the top of the screen. Neighbouring repeats now collapse, which on a
  real `error_log` turned 150 lines into 29. Nothing is dropped: the count
  says how many there were, and only lines next to each other fold, so the
  order of events survives.
- `v` on the Logs tab sets a minimum level, cycling through all, info,
  warnings and errors. It is what makes `LogLevel debug` readable, and the
  header says which floor is in force. Lines cupsd stamps with no level at
  all — every line of `access_log` and `page_log` — are never hidden by it.
- The level letter and the timestamp are dimmed. They are the same thirty
  characters at the start of every line, and colouring them along with the
  message left a wall of red in which nothing stood out.
- The palette is written in hex instead of as ANSI 256 indices. lipgloss steps
  a colour down to whatever the terminal can take, so nothing is lost on a
  smaller palette, but a terminal with 24-bit colour now gets the colour that
  was chosen rather than the nearest corner of the xterm cube.
- Text on a filled block — the title, the active tab, a badge, the error
  banner — inverts with the theme. It was near-black throughout, which on a
  light terminal put black text on the dark blue title and the dark red
  banner and left both barely readable.
- The name in the header fades across its letters, on terminals with the
  colour to draw a fade with. Everywhere else it is the flat block it was,
  and it measures the same either way, so the tab bar does not move.

## v0.1.11 — 2026-08-20

- A narrow terminal keeps its navigation. The header and the row of key hints
  were laid out at their natural width and left to wrap when the screen was
  smaller, and every wrapped line pushed the screen down until the tabs
  scrolled off the top — on the Queue and the Printers tabs first, the two with
  the longest hints. Nothing is drawn wider than the terminal any more, and
  when the header cannot fit it gives up the clock and the server name first,
  then the application's own name, and keeps the tab you are on to the last.
- Colours appear in terminals whose name does not contain "color" — foot,
  alacritty, wezterm and the rest. They announce their colours through
  `COLORTERM`, which does not survive ssh or a plainly configured tmux, and
  without it the whole interface was drawn in no colour at all. terminfo is now
  asked what the terminal can do.

## v0.1.10 — 2026-08-20

- The install script sets up the PATH for whichever shell you actually use.
  It wrote the `export` line into the files the Bourne shells read, which left
  out fish and nushell — neither spells a PATH that way — and tcsh, and it only
  appended to files that already existed, so a zsh user who had never made a
  `~/.zshrc` was told the binary was installed and then could not run it. Each
  shell now gets its own file and its own syntax: `fish_add_path` for fish, a
  list for nushell, `setenv` for tcsh, `export` for the rest, and the file of
  the shell you are in is created when it is not there. Verified against bash,
  zsh, fish, tcsh, nushell and dash. Running the script twice still leaves one
  line.

## v0.1.9 — 2026-08-20

- The install script puts cupstui on the PATH instead of only saying how. It
  left the binary in `~/.local/bin` and printed the line to add, which on a
  distribution that does not already have that directory on the PATH still ended
  in `cupstui: command not found` — the one thing the script exists to prevent.
  The line now goes into the startup files the shell actually reads, bash and
  zsh alike, once however many times the script is run;
  `CUPSTUI_NO_MODIFY_PATH` keeps the old behaviour of printing it for you to add
  yourself.

## v0.1.8 — 2026-08-20

- The interface starts on a machine that has no printers yet. Asked for the
  printer list, a CUPS with nothing configured answers "not found" rather than
  with an empty list, and taking that at its word made the startup checks report
  the service as not responding and the interface refuse to go further — "CUPS
  is not available", on the very machine that most needs it, a fresh install
  where the first thing to do is add a printer. An empty answer is now read as
  what it is, and the check says "0 printers configured".

## v0.1.7 — 2026-08-20

- A first install no longer ends in `cupstui: command not found`. There is an
  install script now — `curl -fsSL .../scripts/install.sh | sh` — which takes
  the release build for the machine it runs on, checks it against the published
  checksum and leaves it in `~/.local/bin`, saying plainly where the binary went
  and what to add to the PATH when that directory is not on it. `go install`
  cannot do any of that, so the README stops leading with it and spells out the
  `$(go env GOPATH)/bin` caveat where it is still offered.
- `-version` reports the real version after `go install`, which used to answer
  "dev" because only the release pipeline stamps one in. It now falls back to
  the module version the build carries. A build from a clone says which commit
  it came from for the same reason.

## v0.1.6 — 2026-08-20

- The device list when adding a printer says what each connection is. One
  printer normally answers on several URIs under the very same make and model,
  which left two rows that read identically and could only be told apart by a
  long percent-encoded string. Every row now names its transport — "IPP/TLS ·
  encrypted", "DNS-SD · IPP · not encrypted" — the best connection of a printer
  comes first and is the only one marked as recommended, and the highlighted row
  spells out its URI with the percent-encoding decoded.
- Adding a printer no longer skips a step number. The header counted by the
  wizard's internal order, so choosing a printer the scan had found went from
  "step 1 of 4" to "step 3 of 4". Typing a URI by hand is the other way of
  finishing the first step rather than a step of its own, and the count is out
  of three because that is what the wizard asks for.
- The message confirming a new printer is in English, like the rest of the
  interface.

## v0.1.5 — 2026-08-20

- A message in the footer — "… set as default.", "Transparent background." —
  now leaves after a few seconds instead of staying for the rest of the
  session. The footer is one line, shared with the key hints, so a message that
  never left took the shortcuts with it. Errors are given longer to be read,
  then go the same way.

## v0.1.4 — 2026-08-20

- The driver advisory now recognises about 3,400 models — every one gutenprint
  itself drives — instead of only the Epson L3150. Installing `gutenprint`
  covers all of them, listed in `internal/drivers/gutenprint.json` and
  regenerated from gutenprint's own printer list by `scripts/gen-driverdb.py`.

## v0.1.3 — 2026-08-20

- Printers can be diagnosed: `i` on a printer turns its queue, connection,
  driver and last job into a short list of checks, each explained in plain
  text rather than as a raw IPP attribute. When the make and model is
  recognised, the screen also names the driver package and the command that
  installs it — shown, never run.

## v0.1.2 — 2026-08-20

- The startup checks no longer report administrative access as denied when CUPS
  merely asked for credentials. cupsd rotates the local certificate underneath a
  running process, so the copy read for a request could already be stale by the
  time it arrived; the request is now made again with a fresh one.
- A request for credentials and a refusal of them are told apart everywhere, so
  a remote server asking to sign in is no longer reported as a permission
  problem.

## v0.1.1 — 2026-08-20

- The row under the cursor in the queue and history tables no longer starts one
  column to the right of the header, so the table stays still as the cursor
  moves.
- The README opens on a recording of the interface instead of two hand-drawn
  screens.

## v0.1.0 — 2026-08-20

First public version.

- Dashboard with printer state and queue depth.
- Queue: hold, release, cancel one or all, filter, live progress for the job
  being printed.
- Printers: enable and disable, set default, accept and reject jobs, add with
  device discovery and driver matching, remove, per-printer quotas and access
  lists.
- Printing: file browser, copies, page ranges, duplex, colour and orientation.
- History from the CUPS page log, with per-user and per-printer totals and CSV
  export.
- CUPS logs with live tail, tinted by severity.
- Remote servers through `CUPS_SERVER` and `~/.cups/client.conf`, signing in
  when the server asks, with TLS through the `Encryption` directive.
- Startup checks reporting what the machine can and cannot do.
