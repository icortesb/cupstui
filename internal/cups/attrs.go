package cups

import (
	"fmt"

	ipp "github.com/phin1x/go-ipp"
)

// Los valores que devuelve el decodificador IPP son interface{}, así que cada
// lectura tolera el tipo inesperado y el atributo ausente en vez de entrar en
// pánico: CUPS omite atributos según versión y driver.

func attrValues(a ipp.Attributes, key string) []interface{} {
	list, ok := a[key]
	if !ok {
		return nil
	}
	out := make([]interface{}, 0, len(list))
	for _, at := range list {
		out = append(out, at.Value)
	}
	return out
}

func attrString(a ipp.Attributes, key string) string {
	for _, v := range attrValues(a, key) {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprint(v)
	}
	return ""
}

func attrStrings(a ipp.Attributes, key string) []string {
	var out []string
	for _, v := range attrValues(a, key) {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func attrInt(a ipp.Attributes, key string) int {
	for _, v := range attrValues(a, key) {
		switch n := v.(type) {
		case int:
			return n
		case int32:
			return int(n)
		case int64:
			return int(n)
		}
	}
	return 0
}

func attrBool(a ipp.Attributes, key string) bool {
	for _, v := range attrValues(a, key) {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
