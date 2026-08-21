# Print system for the demo recording (make demo).
#
# The recording runs inside this container rather than against the machine's
# own CUPS: the GIF is published, and a real desktop's printers, home directory
# and page log are nobody's business. Everything on screen is made up here.

FROM docker.io/library/debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
	cups cups-client cups-filters ghostscript socat iproute2 procps \
	&& rm -rf /var/lib/apt/lists/*

COPY demo-cupsd.conf /etc/cups/cupsd.conf

# The home the file browser opens in. Its contents are the ones the recording
# shows, so they are plausible rather than real.
RUN useradd -m -s /bin/bash ana \
	&& mkdir -p /home/ana/Documents /home/ana/Downloads /home/ana/Pictures \
		/home/ana/Desktop /home/ana/Invoices /home/ana/Scans

ENV HOME=/home/ana
WORKDIR /home/ana
