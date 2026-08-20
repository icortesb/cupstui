# Changelog

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
  when the server asks.
- Startup checks reporting what the machine can and cannot do.
