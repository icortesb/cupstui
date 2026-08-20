package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Paleta. Se usan colores ANSI 256 para que ande en cualquier terminal, con
// variante clara/oscura donde importa el contraste.
var (
	colorBG     = lipgloss.AdaptiveColor{Light: "255", Dark: "234"}
	colorBGAlt  = lipgloss.AdaptiveColor{Light: "253", Dark: "236"}
	colorAccent = lipgloss.AdaptiveColor{Light: "26", Dark: "81"}
	colorOK     = lipgloss.AdaptiveColor{Light: "28", Dark: "78"}
	colorWarn   = lipgloss.AdaptiveColor{Light: "130", Dark: "215"}
	colorErr    = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "241", Dark: "247"}
	colorText   = lipgloss.AdaptiveColor{Light: "235", Dark: "253"}
)

// transparent deja pasar el fondo del terminal en vez de pintar el propio.
//
// Por omisión la app pinta su fondo: en un terminal translúcido, un texto gris
// que se lee sobre una zona oscura del fondo de escritorio desaparece sobre una
// clara, y no hay color de letra que resuelva eso.
var transparent bool

var (
	styleTitle       lipgloss.Style
	styleTabActive   lipgloss.Style
	styleTabInactive lipgloss.Style
	styleHeaderBar   lipgloss.Style
	styleStatusBar   lipgloss.Style
	styleStatusOK    lipgloss.Style
	styleStatusErr   lipgloss.Style
	styleBanner      lipgloss.Style
	styleCard        lipgloss.Style
	styleLabel       lipgloss.Style
	styleValue       lipgloss.Style
	styleBold        lipgloss.Style
	styleDim         lipgloss.Style
	styleKey         lipgloss.Style
	styleApp         lipgloss.Style
	styleAccentText  lipgloss.Style
	styleWarnText    lipgloss.Style
	styleOKText      lipgloss.Style
	styleErrText     lipgloss.Style
)

func init() { SetTransparent(false) }

// SetTransparent reconstruye los estilos. Hay que llamarlo antes de New.
func SetTransparent(v bool) {
	transparent = v
	buildStyles()
}

// base es el punto de partida de todo estilo: si la app pinta su fondo, cada
// fragmento de texto tiene que llevarlo, porque los tramos sin color de fondo
// dejan ver el del terminal y rompen el bloque.
func base() lipgloss.Style {
	s := lipgloss.NewStyle()
	if !transparent {
		s = s.Background(colorBG)
	}
	return s
}

func buildStyles() {
	styleApp = base()

	styleTitle = base().
		Bold(true).
		Foreground(lipgloss.Color("232")).
		Background(colorAccent).
		Padding(0, 1)

	styleTabActive = base().
		Bold(true).
		Foreground(colorAccent).
		Underline(true).
		Padding(0, 2)

	styleTabInactive = base().
		Foreground(colorMuted).
		Padding(0, 2)

	styleHeaderBar = base().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(colorMuted)

	styleStatusBar = base().Foreground(colorMuted).Padding(0, 1)
	styleStatusOK = base().Foreground(colorOK).Bold(true).Padding(0, 1)
	styleStatusErr = base().Foreground(colorErr).Bold(true).Padding(0, 1)

	styleBanner = base().
		Foreground(lipgloss.Color("232")).
		Background(colorErr).
		Bold(true).
		Padding(0, 1)

	styleCard = base().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorMuted).
		Padding(0, 1).
		MarginRight(1)

	styleLabel = base().Foreground(colorMuted)
	styleValue = base().Foreground(colorText)
	styleBold = base().Bold(true).Foreground(colorText)
	styleDim = base().Foreground(colorMuted)
	styleKey = base().Bold(true).Foreground(colorAccent)

	styleAccentText = base().Foreground(colorAccent)
	styleWarnText = base().Foreground(colorWarn)
	styleOKText = base().Foreground(colorOK)
	styleErrText = base().Foreground(colorErr)

	if !transparent {
		styleCard = styleCard.BorderBackground(colorBG)
		styleHeaderBar = styleHeaderBar.BorderBackground(colorBG)
	}
}

// backgroundSequence devuelve la secuencia ANSI que activa el color de fondo de
// la aplicación, resuelta para el perfil de color y el tema vigentes.
//
// Se obtiene pidiéndole a lipgloss que pinte un carácter centinela y quedándose
// con lo que emite antes: así no hay que replicar a mano ni la degradación de
// perfil ni la elección entre variante clara y oscura.
func backgroundSequence() string {
	if transparent {
		return ""
	}
	const sentinel = "\x00"
	rendered := lipgloss.NewStyle().Background(colorBG).Render(sentinel)
	i := strings.Index(rendered, sentinel)
	if i <= 0 {
		return "" // terminal sin color
	}
	return rendered[:i]
}

// paintBackground fuerza el fondo de la aplicación en toda la pantalla.
//
// No alcanza con poner Background en cada estilo: los espacios y los bordes que
// se concatenan a mano no llevan estilo, y cada `\x1b[0m` con el que lipgloss
// cierra un fragmento devuelve el fondo al del terminal. En una terminal
// translúcida eso deja el texto sobre el fondo de escritorio, ilegible en
// cuanto la imagen tiene una zona clara.
func paintBackground(view string, width int) string {
	bg := backgroundSequence()
	if bg == "" {
		return view
	}

	lines := strings.Split(view, "\n")
	for i, line := range lines {
		// Repintar después de cada reset, sea total (0m) o solo de fondo (49m).
		line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+bg)
		line = strings.ReplaceAll(line, "\x1b[49m", "\x1b[49m"+bg)

		if pad := width - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		lines[i] = bg + line + "\x1b[0m"
	}
	return strings.Join(lines, "\n")
}
