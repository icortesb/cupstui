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

// Server is the CUPS instance to talk to.
type Server struct {
	// Local means the connection goes over a unix socket, in which case
	// Address is the socket path. Otherwise Address is host:port.
	Local   bool
	Address string
	Host    string
	User    string
}

func (s Server) String() string {
	if s.Local {
		return "local"
	}
	return s.Address
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
		}
	}
	return serverName
}

func currentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return "root"
}
