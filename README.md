# cupstui

A terminal interface for CUPS. Watch the print queue, cancel and hold jobs,
enable and disable printers, send files, add and remove queues, set per-printer
quotas, read the daemon logs and export a usage report — without leaving the
terminal or opening the CUPS web page.

```
 cupstui    Dashboard    Queue   3   Printers    Print    History    Logs    ⟳ 08:25:18
────────────────────────────────────────────────────────────────────────────────────────
  1 printer   3 jobs queued

╭──────────────────────────────╮
│ Epson_L3150  ● idle          │
│ default                      │
│ Epson L3150 WiFi             │
│ 3 jobs waiting               │
╰──────────────────────────────╯
```

The queue refreshes on its own every three seconds, and the tab carries its
count so it can be watched from any other screen.

```
3 jobs
 ID      USER          DOCUMENT            PRINTER             STATE          TIME
──────────────────────────────────────────────────────────────────────────────────────
 24      icortes       recibo.pdf          Epson_L3150         held           08:25
 23      icortes       factura.pdf         Epson_L3150         held           08:25
 22      icortes       informe_anual.pdf   Epson_L3150         held           08:25
```

A job that is printing shows how far along it is, drawn from the sheet counts
CUPS reports.

## Install

Download a binary from the [releases](https://github.com/icortesb/cupstui/releases),
or:

```sh
go install github.com/icortesb/cupstui/cmd/cupstui@latest
```

From source:

```sh
git clone https://github.com/icortesb/cupstui
cd cupstui
make build
```

Every route produces a static binary with no shared library dependencies.

## Requirements

CUPS, running. Nothing else — `lp` and `lpadmin` ship with CUPS itself, and the
binary carries no runtime dependencies of its own.

Reading the queue, the printers and the logs works for any user. Enabling a
printer, setting quotas, adding and removing queues need membership in the CUPS
administrative group: `wheel` on Arch and Fedora, `lpadmin` on Debian and
Ubuntu. The first run reports which of these apply on your machine, and
`cupstui -check` repeats it later.

## Remote servers

A CUPS on another machine is reached the same way the command line tools reach
it: the `CUPS_SERVER` environment variable, or `ServerName` in
`~/.cups/client.conf`. `CUPS_USER` and the `User` directive set the account.

```sh
CUPS_SERVER=print.example.org cupstui
```

The server is named in the header so it is never a surprise which machine is
being administered. Many servers answer reads without credentials and ask for
them only on an administrative operation; when that happens the password is
asked for then, kept for the session and never written to disk. `S` brings the
prompt up on demand.

### Encryption

Nothing is encrypted by default, which is what CUPS does — turn it on with the
`Encryption` directive or `CUPS_ENCRYPTION`:

| Value | |
|---|---|
| `Never` | in the clear (default) |
| `IfRequested` | upgrade when the server asks |
| `Required` | upgrade before anything is sent |
| `Always` | TLS from the first byte, as https does |

```sh
CUPS_SERVER=print.example.org CUPS_ENCRYPTION=Required cupstui
```

The certificate is verified. CUPS defaults the other way, which makes a man in
the middle indistinguishable from the real server, so accepting one this machine
cannot vouch for has to be asked for: `AllowAnyRoot Yes` in `client.conf`, or
`CUPS_ANYROOT=1`. A server whose certificate is unverified says so in the
header, and an unencrypted connection is flagged before a password is typed
into it.

## Keys

| Key | |
|---|---|
| `1`–`6`, `tab` | switch tab |
| `j` `k` | move |
| `/` | filter |
| `r` | refresh now |
| `?` | help |
| `T` | toggle transparent background |
| `S` | sign in to a remote server |
| `q` | quit |

**Queue** — `p` hold or release, `x` cancel, `X` cancel every job.

**Printers** — `e` enable or disable, `d` set as default, `a` accept or reject
jobs, `A` add, `x` remove, `u` quotas and access.

**Print** — `↑` `↓` change field, `←` `→` change value, `ctrl+o` browse files,
`enter` print. Copies, page ranges, duplex, colour and orientation.

**History** — `s` totals, `E` export CSV.

**Logs** — `n` next log, `G` jump to the end.

## Filtering

The queue and the history take the same query. A bare term is looked for
everywhere; a prefixed one is scoped to a single field. Terms combine, and every
one must match.

```
printer:epson user:ana state:held
```

## Usage report

The history is read from the CUPS page log, which is the durable record — the
daemon keeps only recent completed jobs in memory. `s` switches between the rows
and the totals:

```
10 jobs · 3 pages

  BY USER                                BY PRINTER
  icortes    ██████████ 3p · 10j         Epson_WiFi    ██████████ 3p · 3j
                                         Epson_L3150   ░░░░░░░░░░ 0p · 7j
```

`E` writes what the filter currently shows to a dated CSV in your home
directory.

## Notes on the implementation

Reads and control go over IPP on the unix socket rather than through `lpstat`.
The output of the command line tools is localised and free-form, while IPP
returns typed attributes — `printer-state` is an integer and
`printer-state-reasons` is machine-readable. Over the unix socket CUPS
authenticates the local user; the same request over TCP to `localhost:631`
answers 401.

Submitting jobs and setting quotas go through `lp` and `lpadmin` instead. The Go
IPP library available today sends `copies` twice, cannot encode `page-ranges` as
a `rangeOfInteger` — it sends text, which the filter may ignore without saying
so — and attributes the job to root instead of the real user. It also cannot
encode the multi-valued access list a quota needs. Both were checked against
CUPS 2.4.19 by comparing the attributes each route leaves stored on the job.

The IPP transport is a single connection with one shared `http.Transport`. The
library builds a fresh transport per request and never closes it, which leaks a
socket per query until the collector gets to it; refreshing every few seconds
that exhausts the `MaxClients` of cupsd and stalls printing for every program on
the machine, not just this one.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Licence

MIT. See [LICENSE](LICENSE).
