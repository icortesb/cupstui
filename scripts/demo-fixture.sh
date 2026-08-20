#!/usr/bin/env bash
#
#   scripts/demo-fixture.sh setup
#   scripts/demo-fixture.sh teardown
#
# Print system for the demo recording. The queues point at a local socket that
# throws the data away, so nothing reaches paper and the GIF does not depend on
# whichever printer the machine happens to have. The jobs still complete, which
# is what makes cupsd write the page_log the History tab reads.

set -euo pipefail

readonly WORKDIR=/tmp/cupstui-demo
readonly SINK_PORT=9100
readonly PIDFILE="$WORKDIR/sink.pid"
readonly CONFIG="$HOME/.config/cupstui/config.json"
readonly BACKUP="$WORKDIR/config.bak"

readonly PRINTERS=(Office_Laser Reception_MFP)
readonly DEFAULT_PRINTER=Office_Laser

# A PostScript driver rather than a raw queue: the text filter counts pages on
# the way through, which is what puts real totals in page_log.
readonly DRIVER=drv:///sample.drv/generic.ppd

log() { printf '  %s\n' "$*" >&2; }

start_sink() {
	# Without a listener the socket backend fails and cupsd disables the queue.
	socat -u "TCP-LISTEN:$SINK_PORT,fork,reuseaddr,bind=127.0.0.1" OPEN:/dev/null &
	echo $! >"$PIDFILE"

	for _ in $(seq 20); do
		if ss -ltn "sport = :$SINK_PORT" 2>/dev/null | grep -q LISTEN; then
			log "sink listening on 127.0.0.1:$SINK_PORT"
			return 0
		fi
		sleep 0.1
	done

	log "sink never came up on port $SINK_PORT"
	return 1
}

stop_sink() {
	[[ -f $PIDFILE ]] || return 0
	kill "$(cat "$PIDFILE")" 2>/dev/null || true
	rm -f "$PIDFILE"
}

add_printers() {
	local p
	for p in "${PRINTERS[@]}"; do
		lpadmin -p "$p" -E -v "socket://127.0.0.1:$SINK_PORT" -m "$DRIVER" \
			-D "$(description_for "$p")" -L "$(location_for "$p")" 2>/dev/null
	done
	lpadmin -d "$DEFAULT_PRINTER"
	log "added ${#PRINTERS[@]} printers, default $DEFAULT_PRINTER"
}

# Keyed off the device URI rather than the name, so a queue left behind by an
# earlier run with a different set of names still gets removed.
remove_printers() {
	local name
	while read -r name; do
		[[ -n $name ]] || continue
		cancel -a "$name" 2>/dev/null || true
		lpadmin -x "$name" 2>/dev/null || true
		log "removed $name"
	done < <(lpstat -v 2>/dev/null |
		awk -v sink="socket://127.0.0.1:$SINK_PORT" '$NF == sink { sub(/:$/, "", $3); print $3 }')
}

description_for() {
	case $1 in
	Office_Laser) echo "Generic laser, second floor" ;;
	Reception_MFP) echo "Reception multifunction" ;;
	esac
}

location_for() {
	case $1 in
	Office_Laser) echo "Office" ;;
	Reception_MFP) echo "Reception" ;;
	esac
}

# Real PDFs rather than text files named .pdf: CUPS sniffs the content, and a
# mismatch puts a filter failure in error_log that the Logs tab then shows.
make_pdf() {
	local out=$1 title=$2 pages=$3
	local ps="$WORKDIR/.page.ps" p

	printf '%%!PS\n' >"$ps"
	for p in $(seq "$pages"); do
		cat >>"$ps" <<-EOF
			/Helvetica findfont 22 scalefont setfont
			72 700 moveto ($title) show
			/Helvetica findfont 12 scalefont setfont
			72 670 moveto (page $p of $pages) show
			showpage
		EOF
	done

	ps2pdf "$ps" "$out" 2>/dev/null
	rm -f "$ps"
}

print_history() {
	local -a jobs=(
		"Office_Laser|Q3 forecast|3"
		"Office_Laser|meeting notes|1"
		"Reception_MFP|invoice 4471|2"
		"Reception_MFP|onboarding pack|4"
		"Office_Laser|price list|2"
	)

	local entry printer title pages file
	for entry in "${jobs[@]}"; do
		IFS='|' read -r printer title pages <<<"$entry"
		file="$WORKDIR/$(echo "$title" | tr ' ' '_').pdf"
		make_pdf "$file" "$title" "$pages"
		lp -d "$printer" -t "$title" "$file" >/dev/null
	done

	log "sent ${#jobs[@]} jobs for history, waiting for them to finish"
	wait_for_empty_queue
}

wait_for_empty_queue() {
	local i
	for i in $(seq 60); do
		if [[ -z $(lpstat -o 2>/dev/null) ]]; then
			log "queue drained"
			return 0
		fi
		sleep 0.5
	done
	log "queue did not drain in 30s; history may be short"
}

# Held, so releasing one on camera commits nothing. One name carries a space,
# which is the case the page log parser has to get right.
hold_jobs() {
	local -a titles=("annual report.pdf" "invoice.pdf" "receipt.pdf")
	local title file
	for title in "${titles[@]}"; do
		file="$WORKDIR/$title"
		make_pdf "$file" "${title%.pdf}" 1
		lp -H hold -d "$DEFAULT_PRINTER" -t "$title" "$file" >/dev/null
	done
	log "queued ${#titles[@]} held jobs on $DEFAULT_PRINTER"
}

seed_browsable_files() {
	local f
	for f in "delivery note.pdf" "statement.pdf" "warranty.pdf"; do
		make_pdf "$WORKDIR/$f" "${f%.pdf}" 1
	done
}

stub_config() {
	mkdir -p "$(dirname "$CONFIG")"
	if [[ -f $CONFIG ]]; then
		cp "$CONFIG" "$BACKUP"
	fi
	printf '{"seen":true}' >"$CONFIG"
}

restore_config() {
	if [[ -f $BACKUP ]]; then
		cp "$BACKUP" "$CONFIG"
	else
		rm -f "$CONFIG"
	fi
}

setup() {
	mkdir -p "$WORKDIR"
	stub_config
	start_sink
	add_printers
	print_history
	hold_jobs
	seed_browsable_files
	log "ready"
}

teardown() {
	remove_printers
	stop_sink
	restore_config
	rm -rf "$WORKDIR"
	log "cleaned up"
}

case ${1:-} in
setup) setup ;;
teardown) teardown ;;
*)
	echo "usage: $0 {setup|teardown}" >&2
	exit 2
	;;
esac
