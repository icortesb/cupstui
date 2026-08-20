// Package config guarda las preferencias de la aplicación entre sesiones.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config son las preferencias que sobreviven al cierre. El cero de cada campo
// es el valor por omisión, así que un archivo ausente o incompleto funciona.
type Config struct {
	// Transparent deja ver el fondo del terminal en vez de pintar el propio.
	Transparent bool `json:"transparent"`
	// Seen records that the startup checks have already been shown once.
	Seen bool `json:"seen"`
}

// Path es dónde vive el archivo, respetando XDG_CONFIG_HOME.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cupstui", "config.json"), nil
}

// Load lee las preferencias. Cualquier problema —archivo ausente, ilegible o
// corrupto— devuelve los valores por omisión: una preferencia de aspecto no
// puede impedir que la aplicación arranque.
func Load() Config {
	var c Config

	path, err := Path()
	if err != nil {
		return c
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}
	}
	return c
}

// Save escribe las preferencias, creando el directorio si hace falta.
func Save(c Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
