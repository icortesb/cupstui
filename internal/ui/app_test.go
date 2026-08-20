package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	m.logs.setSize(m.width, 10)
	m.print.setSize(m.width, 10)
	m.add.setSize(m.width, 10)
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
		{"imprimir", func(m *Model) { m.tab = tabPrint }},
		{"logs", func(m *Model) {
			m.tab = tabLogs
			m.logs.setLines([]string{"E [19/Aug/2026] algo falló", "W aviso"}, nil)
		}},
		{"logs sin permisos", func(m *Model) {
			m.tab = tabLogs
			m.logs.setLines(nil, &cups.Error{Kind: cups.KindForbidden, Hint: "sin permisos"})
		}},
		{"alta: explorando", func(m *Model) {
			m.add.active = true
			m.add.loading = true
		}},
		{"alta: dispositivos", func(m *Model) {
			m.add.active = true
			m.add.devices = []cups.Device{{URI: "lpd://1.2.3.4:515/x", MakeModel: "EPSON L3150"}}
		}},
		{"alta: drivers", func(m *Model) {
			m.add.active = true
			m.add.step = stepDriver
			m.add.matches = []cups.PPD{{Name: "a.ppd", MakeModel: "EPSON L3150 Series"}}
		}},
		{"alta: datos", func(m *Model) {
			m.add.active = true
			m.add.step = stepDetails
			m.add.chosenURI = "lpd://1.2.3.4:515/x"
			m.add.chosenPPD = "a.ppd"
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

// press arma un tea.KeyMsg a partir de lo que se tipea.
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
	// Los atajos globales (1..5 cambian de pestaña, q sale, / filtra) no pueden
	// robarle las teclas a un campo de texto: escribir una ruta como
	// /tmp/informe1.txt tiene que quedar escrita, no cambiar de pantalla.
	m := testModel()
	m.tab = tabPrint
	m.print.focus = fieldFile
	m.print.applyFocus()

	const path = "/tmp/prueba1/archivo2.txt"
	m = typeInto(m, path)

	if m.tab != tabPrint {
		t.Errorf("cambió a la pestaña %v mientras se escribía", m.tab)
	}
	if got := m.print.path.Value(); got != path {
		t.Errorf("el campo quedó en %q, quiero %q", got, path)
	}
}

func TestTypingInTheQueueFilterDoesNotTriggerGlobalShortcuts(t *testing.T) {
	m := testModel()
	m.tab = tabQueue
	m.queue.startFiltering()

	m = typeInto(m, "informe1")

	if m.tab != tabQueue {
		t.Errorf("cambió a la pestaña %v mientras se filtraba", m.tab)
	}
	if got := m.queue.filter.Value(); got != "informe1" {
		t.Errorf("el filtro quedó en %q", got)
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
		t.Fatal("esc tiene que sacar el foco del campo de texto")
	}

	next, _ = m.Update(press("2"))
	if got := next.(Model).tab; got != tabQueue {
		t.Errorf("después de esc, 2 debería ir a la Cola, fui a %v", got)
	}
}
