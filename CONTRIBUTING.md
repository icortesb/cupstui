# Contributing

## Building

```sh
make build   # static binary
make test
make vet
make fmt
```

The suite runs without CUPS. One integration test talks to the local daemon and
skips itself when it cannot reach one, so `go test ./...` passes on a machine
with no printing stack.

## Re-recording the demo

```sh
make demo   # needs vhs and podman
```

The recording runs inside a container built from `scripts/demo.Containerfile`,
never against the machine's own CUPS: the GIF is published, and the printers,
home directory and page log of whoever records it are nobody's business.
Everything on screen is invented there by `scripts/demo-fixture.sh` — two
queues pointed at a local socket that discards the data, a handful of completed
jobs so the page log has rows, three held jobs, one undeliverable one so the
log has a warning in it, and a home directory for the file browser to open in.
The Makefile removes the container from a trap, so nothing is left running even
if vhs fails partway.

## Working on it

Behaviour comes with a test. The interesting parts of this program are the ones
that talk to CUPS, and they are covered without a daemon: `internal/cups` takes
an `ipp.Adapter`, so a fake one stands in, and the adapter tests run against a
pretend cupsd on a temporary unix socket.

Changes to the interface should keep two invariants, both of which have tests:
every screen paints the full width of the terminal, and text fields keep their
keys instead of losing them to the global shortcuts.

Comments explain why, not what. If a decision looks odd — talking IPP for reads
but shelling out to `lp` for submission, for one — the reason belongs next to
it, because the next person will otherwise "fix" it.

## Reporting a problem

Include the CUPS version (`cupsd --version`), the distribution, and whether the
server is local or remote. `cupstui -check` prints what the machine can and
cannot do, which answers most of it.
