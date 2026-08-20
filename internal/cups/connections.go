package cups

import (
	"net/url"
	"sort"
	"strings"
)

// DeviceChoice is a scanned device together with what the user needs in order
// to pick between the several URIs one printer usually answers on.
type DeviceChoice struct {
	Device
	// Label names the transport, because device-make-and-model is identical
	// across every connection of the same printer.
	Label string
	// Display is the URI with its percent-encoding decoded, for reading only:
	// CUPS still gets Device.URI.
	Display     string
	Recommended bool
}

// connection is what a URI scheme means to the user, and how much we want it.
// Lower ranks win; the gaps leave room to slot new backends in.
type connection struct {
	label string
	rank  int
}

// The service type inside a dnssd:// URI decides what the dnssd backend will
// actually speak, so it, not the scheme, sets the label.
var dnssdServices = map[string]connection{
	"_ipps._tcp":           {"DNS-SD · IPP/TLS · encrypted", 25},
	"_ipp._tcp":            {"DNS-SD · IPP · not encrypted", 35},
	"_pdl-datastream._tcp": {"DNS-SD · raw stream · no status", 55},
	"_printer._tcp":        {"DNS-SD · LPD · legacy", 65},
	"_uscans._tcp":         {"DNS-SD · scanner", 95},
	"_scanner._tcp":        {"DNS-SD · scanner", 95},
}

var schemes = map[string]connection{
	"usb":           {"USB · direct", 10},
	"ipps":          {"IPP/TLS · encrypted", 20},
	"ipp":           {"IPP · not encrypted", 30},
	"hp":            {"HPLIP", 40},
	"hpfax":         {"HPLIP fax", 90},
	"https":         {"HTTPS · encrypted", 45},
	"socket":        {"raw socket · port 9100 · no status", 50},
	"http":          {"HTTP · not encrypted", 55},
	"lpd":           {"LPD · legacy", 60},
	"smb":           {"SMB · Windows share", 70},
	"ipp14":         {"IPP 1.4 · legacy", 75},
	"beh":           {"backend error handler", 85},
	"implicitclass": {"CUPS class", 85},
}

// unknownRank sits behind everything named, so an unrecognised backend never
// gets recommended over one we understand.
const unknownRank = 100

// describeURI explains a device URI in the terms the list needs.
func describeURI(uri string) connection {
	scheme, rest, found := strings.Cut(uri, "://")
	if !found {
		return connection{uri, unknownRank}
	}
	scheme = strings.ToLower(scheme)

	if scheme == "dnssd" {
		if c, ok := dnssdServices[dnssdService(rest)]; ok {
			return c
		}
		return connection{"DNS-SD", 40}
	}
	if c, ok := schemes[scheme]; ok {
		return c
	}
	return connection{scheme, unknownRank}
}

// dnssdService pulls the service type out of a dnssd:// URI, whose host is the
// DNS-SD instance name followed by the service and domain, as in
// "Printer%20name._ipps._tcp.local".
func dnssdService(rest string) string {
	host, _, _ := strings.Cut(rest, "/")
	parts := strings.Split(host, ".")
	for i := 0; i < len(parts)-1; i++ {
		if strings.HasPrefix(parts[i], "_") && strings.HasPrefix(parts[i+1], "_") {
			return parts[i] + "." + parts[i+1]
		}
	}
	return ""
}

// displayURI decodes the percent-encoding CUPS puts in discovered URIs, which
// is what turns a printer name into an unreadable %20 run.
func displayURI(uri string) string {
	decoded, err := url.PathUnescape(uri)
	if err != nil {
		return uri
	}
	return decoded
}

// DeviceChoices orders a scan for presentation: the connections of one printer
// stay together, the best one leads, and it is the only one marked as
// recommended — and only when there is in fact a choice to make.
func DeviceChoices(devices []Device) []DeviceChoice {
	choices := make([]DeviceChoice, 0, len(devices))
	for _, d := range devices {
		c := describeURI(d.URI)
		choices = append(choices, DeviceChoice{
			Device:  d,
			Label:   c.label,
			Display: displayURI(d.URI),
		})
	}

	counts := map[string]int{}
	for _, c := range choices {
		counts[deviceGroup(c.Device)]++
	}

	rank := func(i int) int { return describeURI(choices[i].URI).rank }
	sort.SliceStable(choices, func(i, j int) bool {
		gi, gj := deviceGroup(choices[i].Device), deviceGroup(choices[j].Device)
		if gi != gj {
			return gi < gj
		}
		if ri, rj := rank(i), rank(j); ri != rj {
			return ri < rj
		}
		return choices[i].URI < choices[j].URI
	})

	seen := map[string]bool{}
	for i := range choices {
		g := deviceGroup(choices[i].Device)
		if !seen[g] && counts[g] > 1 {
			choices[i].Recommended = true
		}
		seen[g] = true
	}
	return choices
}

// deviceGroup identifies the physical printer behind a URI by the name the list
// shows for it, so grouping and display never disagree.
func deviceGroup(d Device) string {
	if name := DeviceName(d); name != "" {
		return strings.ToLower(name)
	}
	return strings.ToLower(d.URI)
}

// DeviceName is the name to show for a scanned device.
func DeviceName(d Device) string {
	if d.MakeModel != "" && !strings.EqualFold(d.MakeModel, "unknown") {
		return d.MakeModel
	}
	return d.Info
}
