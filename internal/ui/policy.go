package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/icortes/cupstui/internal/cups"
)

// Fields of the quota editor, in the order they are visited.
const (
	policyPages = iota
	policyKilobytes
	policyPeriod
	policyAccess
	policyUsers
	policyFieldCount
)

var policyLabels = [policyFieldCount]string{
	"Page limit", "Size limit", "Period", "Access", "Users",
}

// accessMode is which list CUPS holds for the printer.
type accessMode int

const (
	accessEveryone accessMode = iota
	accessAllow
	accessDeny
)

func (a accessMode) String() string {
	switch a {
	case accessAllow:
		return "only these users"
	case accessDeny:
		return "everyone except these users"
	default:
		return "everyone"
	}
}

// policyModel edits the quota and access rules of one printer.
type policyModel struct {
	active  bool
	printer string
	focus   int
	err     error

	pages  textinput.Model
	kbytes textinput.Model
	period textinput.Model
	users  textinput.Model
	access accessMode

	width int
}

func newPolicy() policyModel {
	mk := func(placeholder string) textinput.Model {
		t := textinput.New()
		t.Prompt = ""
		t.Placeholder = placeholder
		t.CharLimit = 200
		return t
	}
	p := policyModel{
		pages:  mk("0 for no limit"),
		kbytes: mk("0 for no limit"),
		period: mk("days"),
		users:  mk("names separated by commas"),
	}
	p.restyle()
	return p
}

func (p *policyModel) restyle() {
	for _, t := range []*textinput.Model{&p.pages, &p.kbytes, &p.period, &p.users} {
		styleInput(t)
	}
}

func (p *policyModel) setSize(width int) {
	p.width = width
	for _, t := range []*textinput.Model{&p.pages, &p.kbytes, &p.period, &p.users} {
		t.Width = width - 24
	}
}

// start opens the editor filled with what the printer currently has.
func (p *policyModel) start(printer cups.Printer) tea.Cmd {
	width := p.width
	*p = newPolicy()
	p.setSize(width)

	p.active = true
	p.printer = printer.Name
	p.pages.SetValue(strconv.Itoa(printer.Policy.PageLimit))
	p.kbytes.SetValue(strconv.Itoa(printer.Policy.KLimit))
	p.period.SetValue(strconv.Itoa(printer.Policy.QuotaDays))

	switch {
	case len(printer.Policy.AllowedUsers) > 0:
		p.access = accessAllow
		p.users.SetValue(strings.Join(printer.Policy.AllowedUsers, ", "))
	case len(printer.Policy.DeniedUsers) > 0:
		p.access = accessDeny
		p.users.SetValue(strings.Join(printer.Policy.DeniedUsers, ", "))
	}

	p.applyFocus()
	return textinput.Blink
}

func (p *policyModel) cancel() { p.active = false }

func (p *policyModel) applyFocus() {
	for _, t := range []*textinput.Model{&p.pages, &p.kbytes, &p.period, &p.users} {
		t.Blur()
	}
	if t := p.input(p.focus); t != nil {
		t.Focus()
	}
}

func (p *policyModel) input(field int) *textinput.Model {
	switch field {
	case policyPages:
		return &p.pages
	case policyKilobytes:
		return &p.kbytes
	case policyPeriod:
		return &p.period
	case policyUsers:
		return &p.users
	}
	return nil
}

func (p *policyModel) move(delta int) {
	p.focus = (p.focus + delta + policyFieldCount) % policyFieldCount
	p.applyFocus()
}

// policy reads the form back, reporting what cannot be understood.
func (p policyModel) policy() (cups.Policy, error) {
	number := func(field int, label string) (int, error) {
		raw := strings.TrimSpace(p.inputValue(field))
		if raw == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("%s must be a whole number, zero or more", label)
		}
		return n, nil
	}

	pages, err := number(policyPages, "Page limit")
	if err != nil {
		return cups.Policy{}, err
	}
	kb, err := number(policyKilobytes, "Size limit")
	if err != nil {
		return cups.Policy{}, err
	}
	period, err := number(policyPeriod, "Period")
	if err != nil {
		return cups.Policy{}, err
	}

	policy := cups.Policy{PageLimit: pages, KLimit: kb, QuotaDays: period}
	users := cups.ParseUserList(p.users.Value())
	switch p.access {
	case accessAllow:
		policy.AllowedUsers = users
	case accessDeny:
		policy.DeniedUsers = users
	}
	return policy, nil
}

func (p policyModel) inputValue(field int) string {
	switch field {
	case policyPages:
		return p.pages.Value()
	case policyKilobytes:
		return p.kbytes.Value()
	case policyPeriod:
		return p.period.Value()
	case policyUsers:
		return p.users.Value()
	}
	return ""
}

// handleKey takes every key while the editor is open.
func (p *policyModel) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		p.cancel()
		return nil
	case "enter":
		return p.submit()
	case "up", "shift+tab":
		p.move(-1)
		return nil
	case "down", "tab":
		p.move(1)
		return nil
	case "left", "right":
		if p.focus == policyAccess {
			delta := 1
			if msg.String() == "left" {
				delta = -1
			}
			p.access = accessMode(((int(p.access)+delta)%3 + 3) % 3)
			return nil
		}
	}

	if t := p.input(p.focus); t != nil {
		var cmd tea.Cmd
		*t, cmd = t.Update(msg)
		return cmd
	}
	return nil
}

func (p *policyModel) submit() tea.Cmd {
	policy, err := p.policy()
	if err != nil {
		p.err = err
		return nil
	}

	printer := p.printer
	p.cancel()
	return action(fmt.Sprintf("Rules updated for %s.", printer), func() error {
		return cups.SetPolicy(printer, policy)
	})
}

func (p policyModel) view() string {
	var b strings.Builder
	b.WriteString(styleBold.Render("  Quotas and access — " + p.printer))
	b.WriteString(styleDim.Render("   esc back"))
	b.WriteString("\n\n")

	if p.err != nil {
		b.WriteString("  " + styleErrText.Render(p.err.Error()) + "\n\n")
	}

	for i := 0; i < policyFieldCount; i++ {
		marker := "  "
		label := styleLabel.Render(pad(policyLabels[i], 13))
		if i == p.focus {
			marker = styleKey.Render("▸ ")
			label = styleAccentText.Bold(true).Render(pad(policyLabels[i], 13))
		}

		var value string
		switch i {
		case policyAccess:
			value = styleValue.Render(p.access.String())
		case policyUsers:
			if p.access == accessEveryone {
				value = styleDim.Render("not used while access is set to everyone")
			} else {
				value = p.users.View()
			}
		default:
			value = p.input(i).View()
		}
		fmt.Fprintf(&b, "%s%s%s\n", marker, label, value)
	}

	// Each line is rendered on its own: a newline inside a styled block makes
	// lipgloss align the rest of it as one paragraph.
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  Limits count per period. Zero means no limit."))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  ←/→ change access · enter to apply"))
	return b.String()
}
