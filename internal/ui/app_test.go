package ui

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/icortesb/cupstui/internal/cups"
	"github.com/icortesb/cupstui/internal/drivers"
)

func testModel() Model {
	m := New(nil)
	m.width, m.height = 100, 20
	m.loaded = true
	m.lastSync = time.Now()
	m.snap = cups.Snapshot{
		Default: "Epson_L3150",
		Printers: []cups.Printer{
			{Name: "Epson_L3150", State: cups.StateIdle, Accepting: true, IsDefault: true, Info: "Epson L3150 WiFi"},
			{Name: "HP_LaserJet", State: cups.StateStopped, Reasons: []string{"media-empty"}},
		},
		Jobs: []cups.Job{
			{ID: 42, Name: "report.pdf", User: "icortes", Printer: "Epson_L3150", State: cups.JobProcessing, Created: time.Now()},
		},
	}
	m.queue.setSize(m.width, 10)
	m.queue.setJobs(m.snap.Jobs)
	m.logs.setSize(m.width, 10)
	m.print.setSize(m.width, 10)
	m.add.setSize(m.width, 10)
	m.diagnose.setSize(m.width, 10)
	m.history.setSize(m.width, 10)
	m.policy.setSize(m.width)
	m.preflight.setSize(m.width)
	return m
}

// Every tab draws in full: no line may be left unpainted or at a width other
// than the screen's, because that is where the terminal background slips
// through and the text turns unreadable.
func TestEveryTabPaintsTheWholeScreen(t *testing.T) {
	withColor(t)
	bg := backgroundSequence()

	for _, tc := range []struct {
		name string
		set  func(m *Model)
	}{
		{"dashboard", func(m *Model) { m.tab = tabDashboard }},
		{"queue", func(m *Model) { m.tab = tabQueue }},
		{"printers", func(m *Model) { m.tab = tabPrinters }},
		{"help", func(m *Model) { m.showHelp = true }},
		{"with an error", func(m *Model) { m.err = &cups.Error{Kind: cups.KindDaemonDown, Hint: "CUPS no responde"} }},
		{"confirming", func(m *Model) {
			m.tab = tabQueue
			m.confirm = &confirmation{prompt: "¿Cancelar el trabajo 42?"}
		}},
		{"print", func(m *Model) { m.tab = tabPrint }},
		{"logs", func(m *Model) {
			m.tab = tabLogs
			m.logs.setLines([]string{"E [19/Aug/2026] algo falló", "W aviso"}, nil)
		}},
		{"logs without permission", func(m *Model) {
			m.tab = tabLogs
			m.logs.setLines(nil, &cups.Error{Kind: cups.KindForbidden, Hint: "permission denied"})
		}},
		{"add: scanning", func(m *Model) {
			m.add.active = true
			m.add.loading = true
		}},
		{"add: devices", func(m *Model) {
			m.add.active = true
			m.add.devices = []cups.Device{{URI: "lpd://1.2.3.4:515/x", MakeModel: "EPSON L3150"}}
		}},
		{"add: drivers", func(m *Model) {
			m.add.active = true
			m.add.step = stepDriver
			m.add.matches = []cups.PPD{{Name: "a.ppd", MakeModel: "EPSON L3150 Series"}}
		}},
		{"history", func(m *Model) {
			m.tab = tabHistory
			m.history.setEntries([]cups.HistoryEntry{
				{Printer: "Epson_L3150", User: "icortes", JobID: 7, Pages: 3,
					Document: "report.pdf", When: time.Now()},
			}, nil)
		}},
		{"history totals", func(m *Model) {
			m.tab = tabHistory
			m.history.summary = true
			m.history.setEntries([]cups.HistoryEntry{
				{Printer: "Epson_L3150", User: "icortes", Pages: 3, Document: "a.pdf", When: time.Now()},
				{Printer: "HP_LaserJet", User: "ana", Pages: 9, Document: "b.pdf", When: time.Now()},
			}, nil)
		}},
		{"add: scanning with spinner", func(m *Model) {
			m.add.active = true
			m.add.loading = true
		}},
		{"history without permission", func(m *Model) {
			m.tab = tabHistory
			m.history.setEntries(nil, &cups.Error{Kind: cups.KindForbidden, Hint: "denied"})
		}},
		{"startup checks running", func(m *Model) {
			m.preflight.active = true
		}},
		{"startup checks done", func(m *Model) {
			m.preflight.active = true
			for _, r := range []cups.CheckResult{
				{Name: "CUPS service", Status: cups.CheckOK, Detail: "running"},
				{Name: "Printing tools", Status: cups.CheckOK, Detail: "available"},
				{Name: "Administrative access", Status: cups.CheckWarn, Detail: "denied",
					Hint: "a long remediation note that has to wrap onto more than one line to be readable"},
				{Name: "Printer drivers", Status: cups.CheckFail, Detail: "none"},
			} {
				m.preflight.results[r.Name] = r
			}
		}},
		{"quotas", func(m *Model) {
			m.policy.start(m.snap.Printers[0])
		}},
		{"add: details", func(m *Model) {
			m.add.active = true
			m.add.step = stepDetails
			m.add.chosenURI = "lpd://1.2.3.4:515/x"
			m.add.chosenPPD = "a.ppd"
		}},
		{"diagnose: loading", func(m *Model) {
			m.diagnose.active = true
			m.diagnose.loading = true
			m.diagnose.printer = m.snap.Printers[0]
		}},
		{"diagnose: results", func(m *Model) {
			m.diagnose.active = true
			m.diagnose.printer = m.snap.Printers[0]
			m.diagnose.advice, m.diagnose.hasAdvice = drivers.Info{
				Vendor: "Epson", Model: "L3150", Package: "epson-inkjet-printer-escpr",
				Source: "AUR", InstallHint: "yay -S epson-inkjet-printer-escpr",
			}, true
			m.diagnose.result = cups.PrinterDiagnosis{
				Printer: "Epson_L3150",
				Overall: cups.CheckFail,
				Checks: []cups.CheckResult{
					{Name: "CUPS service", Status: cups.CheckOK, Detail: "running"},
					{Name: "Driver", Status: cups.CheckFail, Detail: "the filter needed to process jobs is missing",
						Hint: "reinstall or reconfigure the printer's driver"},
				},
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel()
			tc.set(&m)

			for i, line := range strings.Split(m.View(), "\n") {
				if !strings.HasPrefix(line, bg) {
					t.Errorf("line %d has no background: %q", i, first(line, 40))
				}
				if w := lipgloss.Width(line); w != m.width {
					t.Errorf("line %d is %d wide, want %d: %q", i, w, m.width, first(line, 40))
				}
			}
		})
	}
}

func TestViewNeverExceedsTheTerminalHeight(t *testing.T) {
	withColor(t)
	m := testModel()
	if got := strings.Count(m.View(), "\n") + 1; got > m.height {
		t.Errorf("the view takes %d lines and the screen has %d", got, m.height)
	}
}

func first(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func press(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func typeInto(m Model, text string) Model {
	for _, r := range text {
		next, _ := m.Update(press(string(r)))
		m = next.(Model)
	}
	return m
}

func TestTypingInAFormFieldDoesNotTriggerGlobalShortcuts(t *testing.T) {
	// The global shortcuts (1..6 switch tabs, q quits, / filters) cannot steal
	// keys from a text field: typing a path such as /tmp/report1.txt must end
	// up written, not change the screen.
	m := testModel()
	m.tab = tabPrint
	m.print.focus = fieldFile
	m.print.applyFocus()

	const path = "/tmp/prueba1/archivo2.txt"
	m = typeInto(m, path)

	if m.tab != tabPrint {
		t.Errorf("switched to tab %v while typing", m.tab)
	}
	if got := m.print.path.Value(); got != path {
		t.Errorf("the field holds %q, want %q", got, path)
	}
}

func TestTypingInTheQueueFilterDoesNotTriggerGlobalShortcuts(t *testing.T) {
	m := testModel()
	m.tab = tabQueue
	m.queue.startFiltering()

	m = typeInto(m, "informe1")

	if m.tab != tabQueue {
		t.Errorf("switched to tab %v while filtering", m.tab)
	}
	if got := m.queue.filter.Value(); got != "informe1" {
		t.Errorf("the filter holds %q", got)
	}
}

func TestEscapeLeavesTheTextFieldSoShortcutsWorkAgain(t *testing.T) {
	m := testModel()
	m.tab = tabPrint
	m.print.focus = fieldFile
	m.print.applyFocus()

	next, _ := m.Update(press("esc"))
	m = next.(Model)
	if m.print.editing() {
		t.Fatal("esc must take focus off the text field")
	}

	next, _ = m.Update(press("2"))
	if got := next.(Model).tab; got != tabQueue {
		t.Errorf("after esc, 2 should go to the Queue, went to %v", got)
	}
}

// In the print form the ←/→ arrows change the field value, so they cannot also
// switch tabs. To keep nobody trapped, tab and shift+tab must switch tabs
// always, even while typing in a text field.
func TestTabSwitchesTabsEvenWhileEditingThePrintForm(t *testing.T) {
	withColor(t)
	m := testModel()
	m.tab = tabPrint
	m.print.focus = fieldFile // a text field
	m.print.applyFocus()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := next.(Model).tab; got != tabPrint+1 {
		t.Errorf("tab desde Imprimir fue a %v, want the next tab", got)
	}

	m.tab = tabPrint
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if got := next.(Model).tab; got != tabPrint-1 {
		t.Errorf("shift+tab desde Imprimir fue a %v, want the previous one", got)
	}
}

func TestArrowsMoveBetweenFormFieldsWhileEditing(t *testing.T) {
	withColor(t)
	m := testModel()
	m.tab = tabPrint
	m.print.focus = fieldFile
	m.print.applyFocus()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.print.focus != fieldPrinter {
		t.Errorf("↓ left focus on %d, want fieldPrinter", m.print.focus)
	}
	if m.tab != tabPrint {
		t.Errorf("↓ switched tab to %v", m.tab)
	}
}

func TestHorizontalArrowsEditTextInsteadOfCyclingValues(t *testing.T) {
	withColor(t)
	m := testModel()
	m.tab = tabPrint
	m.print.focus = fieldRanges
	m.print.applyFocus()
	m.print.ranges.SetValue("12")
	m.print.ranges.SetCursor(2)

	// ← moves the cursor inside the text; whatever is typed next lands there.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(Model)
	m = typeInto(m, "9")

	if got := m.print.ranges.Value(); got != "192" {
		t.Errorf("the field holds %q, want \"192\": ← must move the cursor, not change a value", got)
	}
}

func TestTransparencyCanBeToggledAtRuntime(t *testing.T) {
	withColor(t)
	m := testModel()

	painted := backgroundSequence()
	if painted == "" {
		t.Fatal("on startup the application must paint its background")
	}

	next, _ := m.Update(press("T"))
	m = next.(Model)
	if backgroundSequence() != "" {
		t.Error("after the toggle no background must be painted")
	}
	for i, line := range strings.Split(m.View(), "\n") {
		if strings.HasPrefix(line, painted) {
			t.Errorf("line %d is still painted in transparent mode", i)
			break
		}
	}

	next, _ = m.Update(press("T"))
	m = next.(Model)
	if backgroundSequence() != painted {
		t.Error("the second toggle must bring the background back")
	}
	for i, line := range strings.Split(m.View(), "\n") {
		if !strings.HasPrefix(line, painted) {
			t.Errorf("line %d was left unpainted on returning from transparent mode", i)
			break
		}
	}
}

// The file browser asks for the directory contents with a command, and the
// answer arrives as an ordinary message. If the root model does not forward the
// messages that are not keys, the listing never reaches it and it shows
// "Bummer. No Files Found." with the directory full.
func TestFilePickerReceivesItsDirectoryListing(t *testing.T) {
	withColor(t)
	m := testModel()
	m.tab = tabPrint

	cmd := m.print.openPicker()
	if cmd == nil {
		t.Fatal("opening the browser must request the listing")
	}

	// Run the command and feed its message through the root Update, just as
	// bubbletea would.
	next, _ := m.Update(cmd())
	m = next.(Model)

	view := m.print.picker.View()
	if strings.Contains(view, "Bummer") {
		t.Errorf("the browser came up empty:\n%s", view)
	}
	if strings.TrimSpace(view) == "" {
		t.Error("the browser drew nothing")
	}
}

func TestFilePickerStartsInTheHomeDirectory(t *testing.T) {
	m := testModel()
	m.print.openPicker()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	if got := m.print.picker.CurrentDirectory; got != home {
		t.Errorf("the browser starts at %q, want %q", got, home)
	}
}

func TestRemovingAPrinterWarnsAboutQueuedJobs(t *testing.T) {
	// Deleting a queue discards whatever is still on it, so the count belongs
	// in the prompt rather than in a surprise afterwards.
	withColor(t)
	m := testModel()
	m.tab = tabPrinters
	m.printers.cursor = 0 // Epson_L3150, which has one job in testModel

	next, _ := m.Update(press("x"))
	m = next.(Model)

	if m.confirm == nil {
		t.Fatal("removing a printer must ask for confirmation")
	}
	if !strings.Contains(m.confirm.prompt, "1 queued job") {
		t.Errorf("prompt = %q, want it to mention the queued job", m.confirm.prompt)
	}
}

func TestRemovingAnEmptyPrinterAsksPlainly(t *testing.T) {
	withColor(t)
	m := testModel()
	m.tab = tabPrinters
	m.printers.cursor = 1 // HP_LaserJet, with no jobs

	next, _ := m.Update(press("x"))
	m = next.(Model)

	if m.confirm == nil {
		t.Fatal("removing a printer must ask for confirmation")
	}
	if strings.Contains(m.confirm.prompt, "queued") {
		t.Errorf("prompt = %q, should not mention jobs when there are none", m.confirm.prompt)
	}
}

func TestDiagnoseOpensForTheSelectedPrinter(t *testing.T) {
	withColor(t)
	m := testModel()
	m.tab = tabPrinters
	m.printers.cursor = 1 // HP_LaserJet

	next, _ := m.Update(press("i"))
	m = next.(Model)

	if !m.diagnose.active {
		t.Fatal("pressing i on the Printers tab must open the diagnosis screen")
	}
	if m.diagnose.printer.Name != "HP_LaserJet" {
		t.Errorf("diagnosing %q, want the selected printer HP_LaserJet", m.diagnose.printer.Name)
	}
}

func TestEscapeClosesTheDiagnosisScreen(t *testing.T) {
	withColor(t)
	m := testModel()
	m.diagnose.active = true

	next, _ := m.Update(press("esc"))
	m = next.(Model)

	if m.diagnose.active {
		t.Error("esc must close the diagnosis screen")
	}
}

func TestSignInIsOfferedWhenTheServerAsksForCredentials(t *testing.T) {
	t.Setenv("CUPS_SERVER", "print.example.org")

	c, err := cups.New()
	if err != nil {
		t.Fatalf("cups.New: %v", err)
	}
	m := testModel()
	m.client = c

	// A challenge and a refusal both leave the user needing to sign in; only the
	// challenge says a password is what is missing, but neither may be swallowed.
	for _, kind := range []cups.Kind{cups.KindUnauthorized, cups.KindForbidden} {
		if cmd := m.offerSignIn(&cups.Error{Kind: kind}); cmd == nil {
			t.Errorf("kind %v: no sign-in was offered", kind)
		}
	}
}

// The footer has one line, shared between a status message and the key hints.
// A message that never expires takes the hints away for the rest of the
// session.
func TestAStatusMessageExpiresSoTheHintsComeBack(t *testing.T) {
	withColor(t)
	m := testModel()

	next, cmd := m.Update(statusMsg{text: "Transparent background."})
	m = next.(Model)
	if !strings.Contains(m.View(), "Transparent background.") {
		t.Fatal("the status message never reached the footer")
	}
	if cmd == nil {
		t.Fatal("a status message must schedule its own expiry")
	}

	next, _ = m.Update(expireStatusMsg{seq: m.statusSeq})
	m = next.(Model)
	view := m.View()
	if strings.Contains(view, "Transparent background.") {
		t.Error("the status message stayed in the footer after expiring")
	}
	if !strings.Contains(view, "quit") {
		t.Error("the key hints did not come back once the status was gone")
	}
}

// Errors take the same line, so they must expire too.
func TestAnErrorStatusExpiresAsWell(t *testing.T) {
	withColor(t)
	m := testModel()

	next, cmd := m.Update(statusMsg{err: errors.New("no job selected")})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("an error status must schedule its own expiry")
	}

	next, _ = m.Update(expireStatusMsg{seq: m.statusSeq})
	m = next.(Model)
	if m.status != "" {
		t.Errorf("the error stayed in the footer: status = %q", m.status)
	}
}

// Two messages in quick succession leave two expiries in flight. The first must
// not wipe the second.
func TestALateExpiryDoesNotWipeANewerStatus(t *testing.T) {
	m := testModel()

	next, _ := m.Update(statusMsg{text: "first"})
	m = next.(Model)
	stale := m.statusSeq

	next, _ = m.Update(statusMsg{text: "second"})
	m = next.(Model)

	next, _ = m.Update(expireStatusMsg{seq: stale})
	m = next.(Model)
	if m.status != "second" {
		t.Errorf("a late expiry wiped a newer message: status = %q", m.status)
	}
}
