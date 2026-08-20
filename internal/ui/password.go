package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/icortesb/cupstui/internal/cups"
)

// passwordModel asks for the password of a remote CUPS. It appears when the
// server refuses a request, not before: many servers answer reads without one.
type passwordModel struct {
	active    bool
	server    string
	encrypted bool
	input     textinput.Model
	width     int
}

func newPassword() passwordModel {
	t := textinput.New()
	t.Prompt = ""
	t.Placeholder = "password"
	t.EchoMode = textinput.EchoPassword
	t.CharLimit = 200

	p := passwordModel{input: t}
	p.restyle()
	return p
}

func (p *passwordModel) restyle()          { styleInput(&p.input) }
func (p *passwordModel) setSize(width int) { p.width = width; p.input.Width = width - 20 }

func (p *passwordModel) start(server string, encrypted bool) tea.Cmd {
	p.active = true
	p.server = server
	p.encrypted = encrypted
	p.input.SetValue("")
	p.input.Focus()
	return textinput.Blink
}

func (p *passwordModel) cancel() {
	p.active = false
	p.input.Blur()
}

// handleKey takes every key while the prompt is up.
func (p *passwordModel) handleKey(msg tea.KeyMsg, c *cups.Client) tea.Cmd {
	switch msg.String() {
	case "esc":
		p.cancel()
		return nil
	case "enter":
		password := p.input.Value()
		p.cancel()
		if password == "" {
			return nil
		}
		c.SetPassword(password)
		return func() tea.Msg { return statusMsg{text: "Credentials sent."} }
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return cmd
}

func (p passwordModel) view() string {
	lines := []string{
		"  " + styleBold.Render("Sign in to "+p.server),
		"",
		"  " + styleLabel.Render("Password  ") + p.input.View(),
		"",
	}

	if !p.encrypted {
		// Sending a password over a connection nobody is protecting is worth
		// saying out loud, once, at the moment it would happen.
		lines = append(lines,
			"  "+styleWarnText.Render("This connection is not encrypted; the password would cross the network in the clear."),
			"  "+styleDim.Render("Set Encryption Required in ~/.cups/client.conf, or CUPS_ENCRYPTION=Required."),
			"")
	}

	return strings.Join(append(lines,
		"  "+styleDim.Render("enter to send · esc to cancel"),
		"  "+styleDim.Render("The password is kept for this session only, never written to disk."),
	), "\n")
}
