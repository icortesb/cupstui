package cups

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// El envío de trabajos se hace con lp y no por IPP, al revés que el resto del
// paquete. La librería IPP disponible manda copies dos veces (pone su propio
// valor por omisión además del pedido), no sabe codificar page-ranges como
// rangeOfInteger —lo manda como texto, que el filtro puede ignorar sin avisar—
// y el trabajo termina atribuido a root en vez de al usuario real. lp resuelve
// los tres casos porque conoce el tipo de cada opción.
//
// Verificado contra CUPS 2.4.19 comparando los atributos que quedan guardados
// en el trabajo por una vía y por la otra.

// Duplex es la impresión a dos caras.
type Duplex int

const (
	DuplexDefault Duplex = iota // lo que tenga configurado la impresora
	DuplexNone
	DuplexLongEdge
	DuplexShortEdge
)

func (d Duplex) String() string {
	switch d {
	case DuplexNone:
		return "una cara"
	case DuplexLongEdge:
		return "dos caras (borde largo)"
	case DuplexShortEdge:
		return "dos caras (borde corto)"
	default:
		return "según la impresora"
	}
}

func (d Duplex) value() string {
	switch d {
	case DuplexNone:
		return "one-sided"
	case DuplexLongEdge:
		return "two-sided-long-edge"
	case DuplexShortEdge:
		return "two-sided-short-edge"
	default:
		return ""
	}
}

// ColorMode elige entre color y blanco y negro.
type ColorMode int

const (
	ColorDefault ColorMode = iota
	ColorColor
	ColorMono
)

func (c ColorMode) String() string {
	switch c {
	case ColorColor:
		return "color"
	case ColorMono:
		return "blanco y negro"
	default:
		return "según la impresora"
	}
}

func (c ColorMode) value() string {
	switch c {
	case ColorColor:
		return "color"
	case ColorMono:
		return "monochrome"
	default:
		return ""
	}
}

// Orientation es la orientación del papel.
type Orientation int

const (
	OrientationDefault Orientation = iota
	OrientationPortrait
	OrientationLandscape
)

func (o Orientation) String() string {
	switch o {
	case OrientationPortrait:
		return "vertical"
	case OrientationLandscape:
		return "horizontal"
	default:
		return "según la impresora"
	}
}

// value es el enum orientation-requested de IPP (3 vertical, 4 horizontal).
func (o Orientation) value() string {
	switch o {
	case OrientationPortrait:
		return "3"
	case OrientationLandscape:
		return "4"
	default:
		return ""
	}
}

// PrintOptions son las opciones de un trabajo nuevo. El cero de cada campo
// significa "lo que tenga configurado la impresora".
type PrintOptions struct {
	Printer     string
	Copies      int
	PageRanges  string
	Duplex      Duplex
	Color       ColorMode
	Orientation Orientation
	Hold        bool // encolar retenido, sin imprimir
}

const maxCopies = 999

// pageRangePattern acepta las formas que entiende CUPS: 1, 1-5, 2,4,6-8.
var pageRangePattern = regexp.MustCompile(`^\d+(-\d+)?(,\d+(-\d+)?)*$`)

func (o PrintOptions) validate() error {
	if o.Copies < 1 {
		return fmt.Errorf("las copias tienen que ser al menos 1")
	}
	if o.Copies > maxCopies {
		return fmt.Errorf("demasiadas copias (máximo %d)", maxCopies)
	}
	if o.PageRanges != "" {
		if err := validatePageRanges(o.PageRanges); err != nil {
			return err
		}
	}
	return nil
}

func validatePageRanges(ranges string) error {
	if !pageRangePattern.MatchString(ranges) {
		return fmt.Errorf("rango de páginas inválido: %q (se espera 1, 1-5 o 2,4,6-8)", ranges)
	}
	for _, part := range strings.Split(ranges, ",") {
		from, to, hasDash := strings.Cut(part, "-")
		first, _ := strconv.Atoi(from)
		if first < 1 {
			return fmt.Errorf("las páginas se numeran desde 1, no desde %d", first)
		}
		if hasDash {
			last, _ := strconv.Atoi(to)
			if last < first {
				return fmt.Errorf("el rango %q termina antes de empezar", part)
			}
		}
	}
	return nil
}

// lpArgs arma los argumentos de lp. Se devuelven como lista y se ejecutan sin
// intérprete de comandos, así un nombre de archivo con espacios o comillas no
// puede convertirse en otra cosa.
func lpArgs(path string, o PrintOptions) ([]string, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}

	var args []string
	if o.Printer != "" {
		args = append(args, "-d", o.Printer)
	}
	args = append(args, "-n", strconv.Itoa(o.Copies))
	if o.Hold {
		args = append(args, "-H", "hold")
	}
	for _, opt := range []struct{ name, value string }{
		{"page-ranges", o.PageRanges},
		{"sides", o.Duplex.value()},
		{"print-color-mode", o.Color.value()},
		{"orientation-requested", o.Orientation.value()},
	} {
		if opt.value != "" {
			args = append(args, "-o", opt.name+"="+opt.value)
		}
	}

	args = append(args, "-t", filepath.Base(path))
	// El "--" evita que un archivo cuyo nombre empiece con guion se interprete
	// como una opción más.
	return append(args, "--", path), nil
}

// jobIDPattern extrae el número de "request id is Epson_L3150-42 (1 file(s))".
var jobIDPattern = regexp.MustCompile(`-(\d+)\s`)

// Submit manda un archivo a imprimir y devuelve el id del trabajo.
func Submit(path string, o PrintOptions) (int, error) {
	if fi, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, &Error{Kind: KindNotFound, Hint: "el archivo no existe: " + path, Err: err}
		}
		return 0, classifyFileError(err)
	} else if fi.IsDir() {
		return 0, &Error{Kind: KindUnknown, Hint: path + " es un directorio, no un archivo"}
	}

	args, err := lpArgs(path, o)
	if err != nil {
		return 0, &Error{Kind: KindUnknown, Hint: err.Error(), Err: err}
	}

	out, err := exec.Command("lp", args...).CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return 0, &Error{
				Kind: KindNotFound,
				Hint: "no se encontró el comando lp (falta el paquete cups-client)",
				Err:  err,
			}
		}
		return 0, &Error{Kind: KindUnknown, Hint: lpError(out, err), Err: err}
	}

	if m := jobIDPattern.FindStringSubmatch(string(out)); m != nil {
		id, _ := strconv.Atoi(m[1])
		return id, nil
	}
	return 0, nil // aceptado, pero sin id reconocible en la salida
}

// lpError se queda con la salida de lp, que explica el problema mejor que el
// código de salida.
func lpError(out []byte, err error) string {
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return err.Error()
	}
	// lp antepone su propio nombre a cada línea; sobra en la UI.
	msg = strings.TrimPrefix(msg, "lp: ")
	if i := strings.IndexByte(msg, '\n'); i > 0 {
		msg = msg[:i]
	}
	return msg
}
