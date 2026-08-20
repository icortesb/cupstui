package cups

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// LogFile es uno de los registros que escribe cupsd.
type LogFile struct {
	Name string
	Path string
	Desc string
}

// LogFiles son los registros estándar de CUPS. Las rutas son las de la
// configuración por omisión; si alguna no existe, la UI lo informa.
var LogFiles = []LogFile{
	{Name: "error_log", Path: "/var/log/cups/error_log", Desc: "errores y avisos del demonio"},
	{Name: "access_log", Path: "/var/log/cups/access_log", Desc: "pedidos HTTP/IPP atendidos"},
	{Name: "page_log", Path: "/var/log/cups/page_log", Desc: "páginas impresas por trabajo"},
}

// tailWindow es cuánto se lee desde el final del archivo. El error_log crece
// sin límite, así que se lee una ventana y no el archivo entero.
const tailWindow int64 = 256 * 1024

// Tail devuelve las últimas n líneas del archivo.
func Tail(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, classifyFileError(err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, classifyFileError(err)
	}

	size := fi.Size()
	offset := size - tailWindow
	if offset < 0 {
		offset = 0
	}

	buf := make([]byte, size-offset)
	if _, err := f.ReadAt(buf, offset); err != nil && !errors.Is(err, os.ErrClosed) {
		// ReadAt devuelve io.EOF si el archivo se acortó entre el Stat y la
		// lectura; lo leído hasta ahí sigue sirviendo.
		if len(buf) == 0 {
			return nil, classifyFileError(err)
		}
	}

	lines := strings.Split(strings.TrimRight(string(buf), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}

	// Si se empezó a leer por el medio, la primera línea puede estar cortada.
	if offset > 0 && len(lines) > 1 {
		lines = lines[1:]
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

func classifyFileError(err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &Error{Kind: KindNotFound, Hint: "el archivo de registro no existe", Err: err}
	case errors.Is(err, fs.ErrPermission):
		return &Error{Kind: KindForbidden, Hint: "sin permisos para leer el registro de CUPS", Err: err}
	default:
		return &Error{Kind: KindUnknown, Hint: err.Error(), Err: err}
	}
}
