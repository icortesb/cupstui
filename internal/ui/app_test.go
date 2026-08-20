package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/icortes/cupstui/internal/cups"
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
			{ID: 42, Name: "informe.pdf", User: "icortes", Printer: "Epson_L3150", State: cups.JobProcessing, Created: time.Now()},
		},
	}
	m.queue.setSize(m.width, 10)
	m.queue.setJobs(m.snap.Jobs)
	return m
}

// Cada pestaña se dibuja completa: ninguna línea puede quedar sin fondo ni con
// un ancho distinto al de la pantalla, porque ahí se cuela el fondo del
// terminal y el texto se vuelve ilegible.
func TestEveryTabPaintsTheWholeScreen(t *testing.T) {
	withColor(t)
	bg := backgroundSequence()

	for _, tc := range []struct {
		name string
		set  func(m *Model)
	}{
		{"dashboard", func(m *Model) { m.tab = tabDashboard }},
		{"cola", func(m *Model) { m.tab = tabQueue }},
		{"impresoras", func(m *Model) { m.tab = tabPrinters }},
		{"ayuda", func(m *Model) { m.showHelp = true }},
		{"con error", func(m *Model) { m.err = &cups.Error{Kind: cups.KindDaemonDown, Hint: "CUPS no responde"} }},
		{"confirmando", func(m *Model) {
			m.tab = tabQueue
			m.confirm = &confirmation{prompt: "¿Cancelar el trabajo 42?"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel()
			tc.set(&m)

			for i, line := range strings.Split(m.View(), "\n") {
				if !strings.HasPrefix(line, bg) {
					t.Errorf("línea %d sin fondo: %q", i, first(line, 40))
				}
				if w := lipgloss.Width(line); w != m.width {
					t.Errorf("línea %d mide %d, quiero %d: %q", i, w, m.width, first(line, 40))
				}
			}
		})
	}
}

func TestViewNeverExceedsTheTerminalHeight(t *testing.T) {
	withColor(t)
	m := testModel()
	if got := strings.Count(m.View(), "\n") + 1; got > m.height {
		t.Errorf("la vista ocupa %d líneas y la pantalla tiene %d", got, m.height)
	}
}

func first(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
