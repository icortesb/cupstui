# Changelog

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
