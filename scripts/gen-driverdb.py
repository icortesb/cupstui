#!/usr/bin/env python3
"""
Regenerates internal/drivers/gutenprint.json from gutenprint's own printer
family XML — the same list cupsd would show once gutenprint is installed
(usr/lib/cups/driver/gutenprint.5.3 lists it dynamically for lpinfo -m).
This only extracts the vendor/model pairs, not gutenprint's XML itself.

Usage:
    pacman -Sp gutenprint | xargs -n1 curl -LO
    tar --zstd -xf gutenprint-*.pkg.tar.zst usr/share/gutenprint/*/xml/printers/
    python3 scripts/gen-driverdb.py usr/share/gutenprint/*/xml/printers/{escp2,canon,pcl}.xml \
        > internal/drivers/gutenprint.json

Other families in the same directory (lexmark.xml, dyesub.xml, dpl.xml, ...)
can be added the same way once their model names have been spot-checked.
"""
import json
import re
import sys

PRINTER = re.compile(r'<printer\s+[^>]*\bname="([^"]+)"[^>]*\bmanufacturer="([^"]+)"')


def extract(paths):
    seen = set()
    for path in paths:
        with open(path, encoding="utf-8") as f:
            text = f.read()
        for name, vendor in PRINTER.findall(text):
            model = name[len(vendor) + 1 :] if name.startswith(vendor + " ") else name
            seen.add((vendor, model))
    return sorted(seen)


def main():
    rows = extract(sys.argv[1:])
    print("[")
    for i, (vendor, model) in enumerate(rows):
        comma = "," if i < len(rows) - 1 else ""
        print(json.dumps({"vendor": vendor, "model": model}, ensure_ascii=False) + comma)
    print("]")
    print(f"{len(rows)} entries", file=sys.stderr)


if __name__ == "__main__":
    main()
