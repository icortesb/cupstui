# Changelog

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
