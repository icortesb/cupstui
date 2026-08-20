package ui

import (
	"os"
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

// En el formulario de impresión las flechas ←/→ cambian el valor del campo, así
// que no pueden además cambiar de pestaña. Para que no se quede uno encerrado,
// tab y shift+tab tienen que cambiar de pestaña siempre, incluso mientras se
// escribe en un campo de texto.
func TestTabSwitchesTabsEvenWhileEditingThePrintForm(t *testing.T) {
	withColor(t)
	m := testModel()
	m.tab = tabPrint
	m.print.focus = fieldFile // un campo de texto
	m.print.applyFocus()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := next.(Model).tab; got != tabLogs {
		t.Errorf("tab desde Imprimir fue a %v, quiero tabLogs", got)
	}

	m.tab = tabPrint
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if got := next.(Model).tab; got != tabPrinters {
		t.Errorf("shift+tab desde Imprimir fue a %v, quiero tabPrinters", got)
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
		t.Errorf("↓ dejó el foco en %d, quiero fieldPrinter", m.print.focus)
	}
	if m.tab != tabPrint {
		t.Errorf("↓ cambió de pestaña a %v", m.tab)
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

	// ← mueve el cursor dentro del texto; lo que se escriba después entra ahí.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(Model)
	m = typeInto(m, "9")

	if got := m.print.ranges.Value(); got != "192" {
		t.Errorf("el campo quedó en %q, quiero \"192\": ← tiene que mover el cursor, no cambiar un valor", got)
	}
}

func TestTransparencyCanBeToggledAtRuntime(t *testing.T) {
	withColor(t)
	m := testModel()

	painted := backgroundSequence()
	if painted == "" {
		t.Fatal("de arranque la aplicación tiene que pintar su fondo")
	}

	next, _ := m.Update(press("T"))
	m = next.(Model)
	if backgroundSequence() != "" {
		t.Error("después del toggle no se tiene que pintar fondo")
	}
	for i, line := range strings.Split(m.View(), "\n") {
		if strings.HasPrefix(line, painted) {
			t.Errorf("línea %d sigue pintada en modo transparente", i)
			break
		}
	}

	next, _ = m.Update(press("T"))
	m = next.(Model)
	if backgroundSequence() != painted {
		t.Error("el segundo toggle tiene que devolver el fondo")
	}
	for i, line := range strings.Split(m.View(), "\n") {
		if !strings.HasPrefix(line, painted) {
			t.Errorf("línea %d quedó sin pintar al volver del modo transparente", i)
			break
		}
	}
}

// El buscador de archivos pide el contenido del directorio con un comando, y
// la respuesta llega como un mensaje cualquiera. Si el modelo raíz no le
// reenvía los mensajes que no son teclas, el listado nunca le llega y muestra
// "Bummer. No Files Found." aunque el directorio esté lleno.
func TestFilePickerReceivesItsDirectoryListing(t *testing.T) {
	withColor(t)
	m := testModel()
	m.tab = tabPrint

	cmd := m.print.openPicker()
	if cmd == nil {
		t.Fatal("abrir el buscador tiene que pedir el listado")
	}

	// Se ejecuta el comando y su mensaje se mete por el Update del modelo raíz,
	// igual que haría bubbletea.
	next, _ := m.Update(cmd())
	m = next.(Model)

	view := m.print.picker.View()
	if strings.Contains(view, "Bummer") {
		t.Errorf("el buscador quedó vacío:\n%s", view)
	}
	if strings.TrimSpace(view) == "" {
		t.Error("el buscador no dibujó nada")
	}
}

func TestFilePickerStartsInTheHomeDirectory(t *testing.T) {
	m := testModel()
	m.print.openPicker()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("sin directorio personal")
	}
	if got := m.print.picker.CurrentDirectory; got != home {
		t.Errorf("el buscador arranca en %q, quiero %q", got, home)
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
