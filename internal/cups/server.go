package cups

import (
	"bufio"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// defaultIPPPort is the port CUPS listens on when none is given.
const defaultIPPPort = "631"

// Encryption is how much CUPS is asked to protect the connection, following
// the values of the Encryption directive.
type Encryption int

const (
	// EncryptNever sends everything in the clear.
	EncryptNever Encryption = iota
	// EncryptIfRequested starts in the clear and upgrades if the server asks.
	EncryptIfRequested
	// EncryptRequired starts in the clear and upgrades before anything is sent.
	EncryptRequired
	// EncryptAlways connects with TLS from the first byte, as https does.
	EncryptAlways
)

func (e Encryption) String() string {
	switch e {
	case EncryptIfRequested:
		return "if requested"
	case EncryptRequired:
		return "required"
	case EncryptAlways:
		return "always"
	default:
		return "never"
	}
}

func parseEncryption(value string) Encryption {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ifrequested":
		return EncryptIfRequested
	case "required":
		return EncryptRequired
	case "always":
		return EncryptAlways
	default:
		return EncryptNever
	}
}

// Server is the CUPS instance to talk to.
type Server struct {
	// Local means the connection goes over a unix socket, in which case
	// Address is the socket path. Otherwise Address is host:port.
	Local   bool
	Address string
	Host    string
	User    string

	Encryption Encryption
	// AllowAnyRoot accepts a certificate this machine cannot verify. CUPS
	// itself defaults to allowing it, which makes a man in the middle
	// indistinguishable from the real server, so here it must be asked for.
	AllowAnyRoot bool
}

// TLS reports whether the connection starts encrypted. Required and
// IfRequested negotiate over a plain connection first, the way CUPS does, and a
// unix socket is never wrapped: it crosses no network and the certificate would
// name a host it does not have.
func (s Server) TLS() bool {
	return !s.Local && s.Encryption == EncryptAlways
}

// Encrypted reports whether the traffic ends up protected, however it got there.
func (s Server) Encrypted() bool {
	return !s.Local && s.Encryption >= EncryptRequired
}

func (s Server) String() string {
	if s.Local {
		return "local"
	}
	if !s.Encrypted() {
		return s.Address
	}
	if s.AllowAnyRoot {
		return s.Address + " (TLS, unverified)"
	}
	return s.Address + " (TLS)"
}

// ResolveServer decides which CUPS to talk to, following the same order the
// CUPS command line tools do: the CUPS_SERVER environment variable, then
// ServerName in the user's client.conf, and the local socket otherwise.
func ResolveServer() Server {
	s := Server{User: currentUser()}

	if name := readClientConf(&s); name != "" {
		applyServerName(&s, name)
	}
	if env := strings.TrimSpace(os.Getenv("CUPS_SERVER")); env != "" {
		applyServerName(&s, env)
	}
	if env := strings.TrimSpace(os.Getenv("CUPS_USER")); env != "" {
		s.User = env
	}
	if env := strings.TrimSpace(os.Getenv("CUPS_ENCRYPTION")); env != "" {
		s.Encryption = parseEncryption(env)
	}
	if env := strings.TrimSpace(os.Getenv("CUPS_ANYROOT")); env != "" && !strings.EqualFold(env, "no") {
		s.AllowAnyRoot = true
	}

	if s.Address == "" {
		socket, err := findSocket(socketSearchPaths)
		if err != nil {
			socket = socketSearchPaths[0]
		}
		s.Local, s.Address, s.Host = true, socket, "localhost"
	}
	return s
}

// applyServerName reads a ServerName value, which is either a socket path or a
// host with an optional port.
func applyServerName(s *Server, name string) {
	if strings.HasPrefix(name, "/") {
		s.Local, s.Address, s.Host = true, name, "localhost"
		return
	}

	host, port := name, defaultIPPPort
	// A bracketed IPv6 literal keeps its brackets; only the last colon of a
	// plain name separates the port.
	if i := strings.LastIndex(name, ":"); i > 0 && !strings.HasSuffix(name, "]") {
		if candidate := name[i+1:]; candidate != "" && !strings.Contains(candidate, "]") {
			host, port = name[:i], candidate
		}
	}

	s.Local = false
	s.Host = host
	s.Address = host + ":" + port
}

// readClientConf returns the ServerName found in the user's client.conf and
// records the User directive along the way. A file that cannot be read or
// understood is simply not used.
func readClientConf(s *Server) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	f, err := os.Open(filepath.Join(home, ".cups", "client.conf"))
	if err != nil {
		return ""
	}
	defer f.Close()

	var serverName string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		directive, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)

		switch strings.ToLower(directive) {
		case "servername":
			serverName = value
		case "user":
			s.User = value
		case "encryption":
			s.Encryption = parseEncryption(value)
		case "allowanyroot":
			s.AllowAnyRoot = isYes(value)
		}
	}
	return serverName
}

// isYes reads the Yes and No a CUPS configuration directive takes.
func isYes(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "true", "on", "1":
		return true
	}
	return false
}

func currentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return "root"
}
