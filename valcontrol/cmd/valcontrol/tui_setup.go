package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m tuiModel) updateSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab", "down":
			return m.setupCycleField(+1), nil
		case "shift+tab", "up":
			return m.setupCycleField(-1), nil
		case "enter":
			return m.submitSetup()
		}
	}

	var cmd tea.Cmd
	if m.setupFocus == 0 {
		m.setupCount, cmd = m.setupCount.Update(msg)
	} else {
		m.setupName, cmd = m.setupName.Update(msg)
	}
	return m, cmd
}

func (m tuiModel) setupCycleField(delta int) tuiModel {
	n := 2
	m.setupFocus = ((m.setupFocus + delta) % n + n) % n
	if m.setupFocus == 0 {
		m.setupCount.Focus()
		m.setupName.Blur()
	} else {
		m.setupName.Focus()
		m.setupCount.Blur()
	}
	return m
}

func (m tuiModel) submitSetup() (tea.Model, tea.Cmd) {
	m.setupError = ""

	countStr := strings.TrimSpace(m.setupCount.Value())
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 {
		m.setupError = "number of validators must be a positive integer"
		return m, nil
	}

	libPath, err := resolveScenarioLib(m.scenarioLib)
	if err != nil {
		m.setupError = err.Error()
		return m, nil
	}

	scenarioName := strings.TrimSpace(m.setupName.Value())
	if scenarioName == "" {
		scenarioName = fmt.Sprintf("valcontrol-%d-validators", count)
	}

	script := buildBootstrapScript(libPath, scenarioName, count, true)

	tmpFile, err := os.CreateTemp("", "valcontrol-bootstrap-*.sh")
	if err != nil {
		m.setupError = fmt.Sprintf("failed to create temp script: %v", err)
		return m, nil
	}
	if _, err := tmpFile.WriteString(script); err != nil {
		os.Remove(tmpFile.Name())
		m.setupError = fmt.Sprintf("failed to write script: %v", err)
		return m, nil
	}
	tmpFile.Close()

	tmpPath := tmpFile.Name()
	captured := scenarioName
	return m, tea.ExecProcess(exec.Command("bash", tmpPath), func(err error) tea.Msg {
		os.Remove(tmpPath)
		return bootstrapDoneMsg{scenarioName: captured, err: err}
	})
}

func (m tuiModel) viewSetup() string {
	w := m.width
	if w == 0 {
		w = 80
	}
	h := m.height
	if h == 0 {
		h = 24
	}

	boxWidth := min(64, w-4)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("New Chain") + "\n\n")

	// Count field
	countLabel := "Number of validators"
	if m.setupFocus == 0 {
		countLabel = lipgloss.NewStyle().Bold(true).Render("> " + countLabel)
	} else {
		countLabel = "  " + countLabel
	}
	b.WriteString(countLabel + "\n")
	b.WriteString("  " + m.setupCount.View() + "\n\n")

	// Name field
	nameLabel := "Scenario name (optional)"
	if m.setupFocus == 1 {
		nameLabel = lipgloss.NewStyle().Bold(true).Render("> " + nameLabel)
	} else {
		nameLabel = "  " + nameLabel
	}
	b.WriteString(nameLabel + "\n")
	b.WriteString("  " + m.setupName.View() + "\n")

	countVal := strings.TrimSpace(m.setupCount.Value())
	nameVal := strings.TrimSpace(m.setupName.Value())
	if nameVal == "" && countVal != "" {
		if n, err := strconv.Atoi(countVal); err == nil && n > 0 {
			b.WriteString(lipgloss.NewStyle().Faint(true).Render(
				fmt.Sprintf("  default: valcontrol-%d-validators", n),
			) + "\n")
		}
	}
	b.WriteString("\n")

	b.WriteString(lipgloss.NewStyle().Faint(true).Render(
		fmt.Sprintf("scenario-lib: %s", m.scenarioLib),
	) + "\n\n")

	b.WriteString("Tab: next field   Enter: create   q: quit\n")

	if m.setupError != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(
			"error: "+m.setupError,
		))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(boxWidth).
		Render(b.String())

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// min is needed for setup box width calculation (Go 1.21+ builtin, but
// redeclaring avoids conflicts with older toolchain assumptions).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

