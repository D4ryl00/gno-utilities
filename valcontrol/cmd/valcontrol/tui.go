package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/remi/gno-utilities/valcontrol/pkg/valcontrol"
)

type tuiState int

const (
	stateSetup tuiState = iota
	stateRunning
)

const minWideLayoutWidth = 110

// message types

type refreshTickMsg time.Time
type snapshotsMsg []valcontrol.ValidatorSnapshot

type actionResultMsg struct {
	status string
	err    error
}

type bootstrapDoneMsg struct {
	scenarioName string
	err          error
}

type destroyDoneMsg struct {
	err error
}

// model

type tuiModel struct {
	state         tuiState
	inventoryPath string
	scenarioLib   string
	inv           *valcontrol.Inventory
	client        *valcontrol.Client
	width, height int

	// setup state
	setupCount textinput.Model
	setupName  textinput.Model
	setupFocus int
	setupError string

	// running state
	table              table.Model
	snapshots          []valcontrol.ValidatorSnapshot
	selectedValidators map[string]struct{}
	showDetails        bool
	loading            bool
	status             string
	lastError          string

	delayMode  bool
	delayPhase string
	delayInput textinput.Model

	confirmMode   bool
	confirmAction string
}

func runTUI(args []string) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	invPath := fs.String("inventory", defaultInventoryPath(), "inventory path")
	timeout := fs.Duration("timeout", 5*time.Second, "HTTP timeout")
	scenarioLib := fs.String("scenario-lib", defaultScenarioLibPath(), "path to val-scenarios/lib/scenario.sh")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	resolved := *invPath
	if abs, err := filepath.Abs(resolved); err == nil {
		resolved = abs
	}

	client := valcontrol.NewClient(*timeout)
	inv, _ := valcontrol.LoadInventory(resolved) // nil if missing — shows setup form

	m := newTUIModel(inv, client, resolved, *scenarioLib)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newTUIModel(inv *valcontrol.Inventory, client *valcontrol.Client, inventoryPath, scenarioLib string) tuiModel {
	m := tuiModel{
		inventoryPath: inventoryPath,
		scenarioLib:   scenarioLib,
		inv:           inv,
		client:        client,
	}
	if inv == nil {
		m.state = stateSetup
		m.setupCount = newSetupInput("e.g. 3", 3)
		m.setupName = newSetupInput("optional, e.g. my-testnet", 64)
		m.setupCount.Focus()
	} else {
		m.state = stateRunning
		m = m.initRunningState()
	}
	return m
}

func newSetupInput(placeholder string, charLimit int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = charLimit
	return ti
}

func (m tuiModel) initRunningState() tuiModel {
	cols := []table.Column{
		{Title: "Validator", Width: 18},
		{Title: "Height", Width: 8},
		{Title: "Sync", Width: 7},
		{Title: "Control", Width: 8},
		{Title: "Rules", Width: 22},
	}
	tbl := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(12),
	)
	tbl.SetStyles(table.DefaultStyles())

	input := textinput.New()
	input.Placeholder = "e.g. 5s"
	input.CharLimit = 16

	m.table = tbl
	m.delayInput = input
	m.selectedValidators = make(map[string]struct{})
	m.showDetails = true
	m.loading = true
	m.status = "loading validators..."
	return m
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m tuiModel) Init() tea.Cmd {
	if m.state == stateRunning {
		return tea.Batch(fetchSnapshotsCmd(m.inv, m.client), refreshTickCmd())
	}
	return textinput.Blink
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == stateRunning {
			m.resizeTable()
		}
		return m, nil
	case bootstrapDoneMsg:
		return m.handleBootstrapDone(msg)
	case destroyDoneMsg:
		return m.handleDestroyDone(msg)
	}

	switch m.state {
	case stateSetup:
		return m.updateSetup(msg)
	case stateRunning:
		return m.updateRunning(msg)
	}
	return m, nil
}

func (m tuiModel) handleBootstrapDone(msg bootstrapDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.state = stateSetup
		m.setupError = fmt.Sprintf("bootstrap failed: %v", msg.err)
		return m, textinput.Blink
	}
	invPath := computeInventoryPath(msg.scenarioName)
	inv, err := valcontrol.LoadInventory(invPath)
	if err != nil {
		m.state = stateSetup
		m.setupError = fmt.Sprintf("inventory not found after bootstrap: %v", err)
		return m, textinput.Blink
	}
	m.state = stateRunning
	m.inv = inv
	m.inventoryPath = invPath
	m = m.initRunningState()
	return m, tea.Batch(fetchSnapshotsCmd(m.inv, m.client), refreshTickCmd())
}

func (m tuiModel) handleDestroyDone(msg destroyDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.lastError = fmt.Sprintf("destroy failed: %v", msg.err)
		m.status = "destroy failed"
		return m, nil
	}
	m.state = stateSetup
	m.inv = nil
	m.snapshots = nil
	m.setupError = ""
	m.setupCount = newSetupInput("e.g. 3", 3)
	m.setupName = newSetupInput("optional, e.g. my-testnet", 64)
	m.setupFocus = 0
	m.setupCount.Focus()
	m.confirmMode = false
	m.confirmAction = ""
	return m, textinput.Blink
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m tuiModel) View() string {
	switch m.state {
	case stateSetup:
		return m.viewSetup()
	case stateRunning:
		return m.viewRunning()
	}
	return ""
}

// ── Running state ─────────────────────────────────────────────────────────────

func (m tuiModel) updateRunning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case snapshotsMsg:
		m.loading = false
		m.snapshots = []valcontrol.ValidatorSnapshot(msg)
		m.setRows()
		m.status = fmt.Sprintf("refreshed %d validators at %s", len(m.snapshots), time.Now().Format("15:04:05"))
		m.lastError = ""
		return m, nil

	case actionResultMsg:
		if msg.err != nil {
			m.status = "action failed"
			m.lastError = msg.err.Error()
		} else {
			m.status = msg.status
			m.lastError = ""
		}
		return m, fetchSnapshotsCmd(m.inv, m.client)

	case refreshTickMsg:
		return m, tea.Batch(fetchSnapshotsCmd(m.inv, m.client), refreshTickCmd())

	case tea.KeyMsg:
		if m.delayMode {
			return m.updateDelayMode(msg)
		}
		if m.confirmMode {
			return m.updateConfirmMode(msg)
		}
		return m.updateRunningKey(msg)
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m tuiModel) updateRunningKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "ctrl+r":
		m.status = "refreshing..."
		m.loading = true
		return m, fetchSnapshotsCmd(m.inv, m.client)
	case "r":
		m.confirmMode = true
		m.confirmAction = "safe-reset"
		m.status = "confirm safe reset validator (y/n)"
		return m, nil
	case "R":
		m.confirmMode = true
		m.confirmAction = "reset"
		m.status = "confirm clean reset validator (y/n)"
		return m, nil
	case " ":
		m.toggleSelection()
		return m, nil
	case "a":
		m.toggleAllSelections()
		return m, nil
	case "tab":
		m.toggleDetailsPanel()
		m.resizeTable()
		return m, nil
	case "p":
		return m, m.toggleDropCmd("proposal")
	case "v":
		return m, m.toggleDropCmd("prevote")
	case "c":
		return m, m.toggleDropCmd("precommit")
	case "P":
		return m.startDelayMode("proposal"), nil
	case "V":
		return m.startDelayMode("prevote"), nil
	case "C":
		return m.startDelayMode("precommit"), nil
	case "x":
		return m, m.clearAllCmd()
	case "s":
		return m, m.stopValidatorCmd()
	case "S":
		return m, m.startValidatorCmd()
	case "d":
		m.confirmMode = true
		m.confirmAction = "destroy"
		m.status = "confirm destroy network"
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m tuiModel) updateDelayMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.delayMode = false
		m.delayPhase = ""
		m.delayInput.Blur()
		m.delayInput.SetValue("")
		m.status = "delay prompt cancelled"
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.delayInput.Value())
		if value == "" {
			m.lastError = "delay is required"
			return m, nil
		}
		if _, err := time.ParseDuration(value); err != nil {
			m.lastError = fmt.Sprintf("invalid duration: %v", err)
			return m, nil
		}
		phase := m.delayPhase
		m.delayMode = false
		m.delayPhase = ""
		m.delayInput.Blur()
		m.delayInput.SetValue("")
		return m, m.setDelayCmd(phase, value)
	}

	var cmd tea.Cmd
	m.delayInput, cmd = m.delayInput.Update(msg)
	return m, cmd
}

func (m tuiModel) updateConfirmMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "N":
		m.confirmMode = false
		m.confirmAction = ""
		m.status = "cancelled"
		return m, nil
	case "enter", "y", "Y":
		action := m.confirmAction
		m.confirmMode = false
		m.confirmAction = ""
		switch action {
		case "destroy":
			m.status = "destroying network..."
			m.loading = true
			return m, m.destroyNetworkCmd()
		case "reset":
			m.status = "resetting validator (clean)..."
			m.loading = true
			return m, m.resetValidatorCmd(false)
		case "safe-reset":
			m.status = "resetting validator (safe)..."
			m.loading = true
			return m, m.resetValidatorCmd(true)
		}
	}
	return m, nil
}

func (m tuiModel) viewRunning() string {
	left := m.renderTable()
	content := left
	if m.showDetails {
		right := m.renderDetails()
		content = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
		if m.compactLayout() {
			content = lipgloss.JoinVertical(lipgloss.Left, left, right)
		}
	}
	footer := m.renderFooter()
	if m.delayMode {
		footer = lipgloss.JoinVertical(lipgloss.Left, footer, m.renderDelayPrompt())
	}
	if m.confirmMode {
		footer = lipgloss.JoinVertical(lipgloss.Left, footer, m.renderConfirmPrompt())
	}
	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}

func (m *tuiModel) resizeTable() {
	if m.width == 0 || m.height == 0 {
		return
	}
	leftWidth := max(52, m.width/2)
	tableHeight := max(8, m.height-8)
	validatorColWidth := 18
	rulesMinWidth := 16

	if !m.showDetails {
		leftWidth = max(32, m.width-4)
		tableHeight = max(8, m.height-8)
		validatorColWidth = 18
		rulesMinWidth = 16
	} else if m.compactLayout() {
		leftWidth = max(32, m.width-4)
		tableHeight = max(6, (m.height-10)/2)
		validatorColWidth = 14
		rulesMinWidth = 10
	} else {
		if leftWidth > m.width-24 {
			leftWidth = m.width - 24
		}
		if leftWidth < 40 {
			leftWidth = 40
		}
	}

	m.table.SetWidth(leftWidth)
	m.table.SetHeight(tableHeight)
	cols := m.table.Columns()
	if len(cols) == 5 {
		cols[0].Width = validatorColWidth
		cols[4].Width = max(rulesMinWidth, leftWidth-validatorColWidth-8-7-8-10)
		m.table.SetColumns(cols)
	}
}

func (m *tuiModel) setRows() {
	selectedName := m.selectedValidatorName()
	rows := make([]table.Row, 0, len(m.snapshots))
	selectedIndex := 0
	validSelections := make(map[string]struct{}, len(m.selectedValidators))

	for idx, snap := range m.snapshots {
		height := "error"
		sync := "-"
		if snap.RPC != nil {
			height = snap.RPC.Result.SyncInfo.LatestBlockHeight
			if snap.RPC.Result.SyncInfo.CatchingUp {
				sync = "yes"
			} else {
				sync = "no"
			}
		}
		control := "no"
		if snap.Validator != nil && snap.Validator.ControlURL != nil {
			control = "yes"
		}
		name := ""
		label := ""
		if snap.Validator != nil {
			name = snap.Validator.Name
			label = fmt.Sprintf("%s(%s)", name, shortAddr(snap.Validator.Address))
			if _, ok := m.selectedValidators[name]; ok {
				validSelections[name] = struct{}{}
			}
		}
		mark := " "
		if _, ok := m.selectedValidators[name]; ok {
			mark = "x"
		}
		rows = append(rows, table.Row{
			fmt.Sprintf("[%s] %s", mark, label), height, sync, control,
			valcontrol.FormatRules(snap.Signer),
		})
		if snap.Validator != nil && snap.Validator.Name == selectedName {
			selectedIndex = idx
		}
	}

	m.selectedValidators = validSelections
	m.table.SetRows(rows)
	if len(rows) > 0 {
		m.table.SetCursor(selectedIndex)
	}
}

func (m tuiModel) renderTable() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(m.table.Width() + 2)
	if !m.compactLayout() {
		box = box.Height(m.tableBoxHeight())
	}

	header := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("Validators  Scenario: %s  Selected: %d", m.inv.Scenario, m.selectedCount()),
	)
	return box.Render(lipgloss.JoinVertical(lipgloss.Left, header, m.table.View()))
}

func (m tuiModel) renderDetails() string {
	width := max(30, m.width-m.table.Width()-4)
	if m.compactLayout() {
		width = max(32, m.width-4)
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(width)

	snap := m.selectedSnapshot()
	if snap == nil || snap.Validator == nil {
		return box.Render("No validator selected")
	}

	lines := []string{
		lipgloss.NewStyle().Bold(true).Render("Details"),
		fmt.Sprintf("name: %s", snap.Validator.Name),
		fmt.Sprintf("service: %s", snap.Validator.Service),
		fmt.Sprintf("address: %s", snap.Validator.Address),
		fmt.Sprintf("rule targets: %s", m.ruleTargetSummary()),
	}

	if snap.RPC != nil {
		lines = append(lines,
			fmt.Sprintf("moniker: %s", snap.RPC.Result.NodeInfo.Moniker),
			fmt.Sprintf("height: %s", snap.RPC.Result.SyncInfo.LatestBlockHeight),
			fmt.Sprintf("catching up: %v", snap.RPC.Result.SyncInfo.CatchingUp),
			fmt.Sprintf("rpc: %s", snap.Validator.RPCURL),
		)
	} else if snap.RPCErr != "" {
		lines = append(lines, fmt.Sprintf("rpc error: %s", snap.RPCErr))
	}

	if snap.Validator.ControlURL != nil {
		lines = append(lines, fmt.Sprintf("control: %s", *snap.Validator.ControlURL))
	} else {
		lines = append(lines, "control: unavailable")
	}

	lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("Rules"))
	if snap.Signer != nil && len(snap.Signer.Rules) > 0 {
		for _, phase := range []string{"proposal", "prevote", "precommit"} {
			rule := snap.Signer.Rules[phase]
			if rule == nil {
				continue
			}
			line := fmt.Sprintf("- %s: %s", phase, rule.Action)
			if rule.Delay != "" {
				line += " " + rule.Delay
			}
			lines = append(lines, line)
		}
	} else {
		lines = append(lines, "- none")
	}

	lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("Stats"))
	if snap.Signer != nil {
		for _, phase := range []string{"proposal", "prevote", "precommit"} {
			stat, ok := snap.Signer.Stats[phase]
			if !ok {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s: matched=%d dropped=%d delayed=%d",
				phase, stat.Matched, stat.Dropped, stat.Delayed))
		}
	} else if snap.SignerErr != "" {
		lines = append(lines, fmt.Sprintf("signer error: %s", snap.SignerErr))
	}

	lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("Keys"))
	lines = append(lines,
		"↑/↓: select validator",
		"tab: hide/show details",
		"space: toggle validator selection",
		"a: select all / clear selection",
		"r: refresh",
		"p/v/c: toggle drop rule on rule targets",
		"P/V/C: set delay rule on rule targets",
		"x: clear all rules on rule targets",
		"s/S: stop/start validator",
		"r/R: safe reset / clean reset validator",
		"d: destroy network",
		"ctrl+r: refresh",
		"q: quit",
	)

	return box.Render(strings.Join(lines, "\n"))
}

func (m tuiModel) renderFooter() string {
	statusStyle := lipgloss.NewStyle().Padding(0, 1)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Padding(0, 1)

	lines := []string{
		statusStyle.Render(fmt.Sprintf("inventory: %s", m.inventoryPath)),
		statusStyle.Render(fmt.Sprintf("status: %s", m.status)),
	}
	if m.lastError != "" {
		lines = append(lines, errStyle.Render("error: "+m.lastError))
	}
	return strings.Join(lines, "\n")
}

func (m tuiModel) renderDelayPrompt() string {
	label := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("Set %s delay on %s", m.delayPhase, m.ruleTargetSummary()),
	)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Render(label + "\n" + m.delayInput.View() + "\n" + "enter duration and press Enter, Esc to cancel")
}

func (m tuiModel) renderConfirmPrompt() string {
	name := ""
	if m.inv != nil {
		name = m.inv.Scenario
	}
	label := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("Destroy chain %q and delete all data?", name),
	)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Render(label + "\n" + "Enter/y: confirm   Esc/n: cancel")
}

func (m tuiModel) selectedSnapshot() *valcontrol.ValidatorSnapshot {
	if len(m.snapshots) == 0 {
		return nil
	}
	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= len(m.snapshots) {
		return nil
	}
	return &m.snapshots[cursor]
}

func (m tuiModel) selectedValidatorName() string {
	snap := m.selectedSnapshot()
	if snap == nil || snap.Validator == nil {
		return ""
	}
	return snap.Validator.Name
}

// shortAddr returns the first 6 characters of a validator address for compact display.
func shortAddr(address string) string {
	if len(address) > 6 {
		return address[:6]
	}
	return address
}

func (m tuiModel) compactLayout() bool {
	return m.width > 0 && m.width < minWideLayoutWidth
}

func (m tuiModel) tableBoxHeight() int {
	return max(10, m.height-4)
}

func (m *tuiModel) toggleDetailsPanel() {
	m.showDetails = !m.showDetails
	if m.showDetails {
		m.status = "details panel shown"
	} else {
		m.status = "details panel hidden"
	}
	m.lastError = ""
}

func (m *tuiModel) toggleSelection() {
	name := m.selectedValidatorName()
	if name == "" {
		return
	}
	if _, ok := m.selectedValidators[name]; ok {
		delete(m.selectedValidators, name)
		m.status = fmt.Sprintf("removed %s from rule targets", name)
	} else {
		m.selectedValidators[name] = struct{}{}
		m.status = fmt.Sprintf("added %s to rule targets", name)
	}
	m.lastError = ""
	m.setRows()
}

func (m *tuiModel) toggleAllSelections() {
	if len(m.snapshots) == 0 {
		return
	}
	if m.selectedCount() == len(m.snapshots) {
		m.selectedValidators = make(map[string]struct{})
		m.status = "cleared validator selection"
		m.lastError = ""
		m.setRows()
		return
	}

	selected := make(map[string]struct{}, len(m.snapshots))
	for _, snap := range m.snapshots {
		if snap.Validator != nil {
			selected[snap.Validator.Name] = struct{}{}
		}
	}
	m.selectedValidators = selected
	m.status = fmt.Sprintf("selected all %d validators", len(selected))
	m.lastError = ""
	m.setRows()
}

func (m tuiModel) startDelayMode(phase string) tuiModel {
	targets := m.ruleTargets()
	if len(targets) == 0 {
		m.lastError = "no validator selected"
		return m
	}
	for _, snap := range targets {
		if snap.Validator == nil || snap.Validator.ControlURL == nil {
			m.lastError = "one or more selected validators have no control API"
			return m
		}
	}
	m.delayMode = true
	m.delayPhase = phase
	m.delayInput.SetValue("")
	m.delayInput.Focus()
	m.status = fmt.Sprintf("setting %s delay on %s", phase, m.ruleTargetSummary())
	m.lastError = ""
	return m
}

func (m tuiModel) toggleDropCmd(phase string) tea.Cmd {
	targets := m.ruleTargets()
	if len(targets) == 0 {
		return nil
	}
	if shouldClearDropForTargets(targets, phase) {
		return m.clearRuleCmd(phase)
	}
	return m.putRuleCmd(phase, "drop", "")
}

func (m tuiModel) setDelayCmd(phase, delay string) tea.Cmd {
	targets := m.ruleTargets()
	if len(targets) == 0 {
		return func() tea.Msg { return actionResultMsg{err: fmt.Errorf("no validator selected")} }
	}
	targetLabel := describeTargetCount(targets)
	return func() tea.Msg {
		var failures []string
		applied := 0
		for _, snap := range targets {
			if snap.Validator == nil {
				continue
			}
			if snap.Validator.ControlURL == nil {
				failures = append(failures, fmt.Sprintf("%s has no control URL", snap.Validator.Name))
				continue
			}
			if err := m.client.PutRule(*snap.Validator.ControlURL, phase, "delay", nil, nil, delay); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", snap.Validator.Name, err))
				continue
			}
			applied++
		}
		return batchActionResult(
			fmt.Sprintf("set %s delay=%s on %s", phase, delay, targetLabel),
			applied,
			failures,
		)
	}
}

func (m tuiModel) clearAllCmd() tea.Cmd {
	targets := m.ruleTargets()
	if len(targets) == 0 {
		return func() tea.Msg { return actionResultMsg{err: fmt.Errorf("no validator selected")} }
	}
	targetLabel := describeTargetCount(targets)
	return func() tea.Msg {
		var failures []string
		restarted := 0
		applied := 0
		for _, snap := range targets {
			if snap.Validator == nil {
				continue
			}
			if snap.Validator.ControlURL == nil {
				failures = append(failures, fmt.Sprintf("%s has no control URL", snap.Validator.Name))
				continue
			}
			restart := shouldRestartAfterClear(snap.Signer, "")
			if err := m.client.Reset(*snap.Validator.ControlURL); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", snap.Validator.Name, err))
				continue
			}
			applied++
			if restart {
				if err := restartValidator(m.inv, m.scenarioLib, snap.Validator.Name); err != nil {
					failures = append(failures, fmt.Sprintf("%s restart: %v", snap.Validator.Name, err))
					continue
				}
				restarted++
			}
		}
		status := fmt.Sprintf("cleared all rules on %s", targetLabel)
		if restarted > 0 {
			status += fmt.Sprintf(" and restarted %d validator", restarted)
			if restarted != 1 {
				status += "s"
			}
		}
		return batchActionResult(status, applied, failures)
	}
}

func (m tuiModel) clearRuleCmd(phase string) tea.Cmd {
	targets := m.ruleTargets()
	if len(targets) == 0 {
		return func() tea.Msg { return actionResultMsg{err: fmt.Errorf("no validator selected")} }
	}
	targetLabel := describeTargetCount(targets)
	return func() tea.Msg {
		var failures []string
		restarted := 0
		applied := 0
		for _, snap := range targets {
			if snap.Validator == nil {
				continue
			}
			if snap.Validator.ControlURL == nil {
				failures = append(failures, fmt.Sprintf("%s has no control URL", snap.Validator.Name))
				continue
			}
			restart := shouldRestartAfterClear(snap.Signer, phase)
			if err := m.client.ClearRule(*snap.Validator.ControlURL, phase); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", snap.Validator.Name, err))
				continue
			}
			applied++
			if restart {
				if err := restartValidator(m.inv, m.scenarioLib, snap.Validator.Name); err != nil {
					failures = append(failures, fmt.Sprintf("%s restart: %v", snap.Validator.Name, err))
					continue
				}
				restarted++
			}
		}
		status := fmt.Sprintf("cleared %s on %s", phase, targetLabel)
		if restarted > 0 {
			status += fmt.Sprintf(" and restarted %d validator", restarted)
			if restarted != 1 {
				status += "s"
			}
		}
		return batchActionResult(status, applied, failures)
	}
}

func (m tuiModel) putRuleCmd(phase, action, delay string) tea.Cmd {
	targets := m.ruleTargets()
	if len(targets) == 0 {
		return func() tea.Msg { return actionResultMsg{err: fmt.Errorf("no validator selected")} }
	}
	targetLabel := describeTargetCount(targets)
	return func() tea.Msg {
		var failures []string
		applied := 0
		for _, snap := range targets {
			if snap.Validator == nil {
				continue
			}
			if snap.Validator.ControlURL == nil {
				failures = append(failures, fmt.Sprintf("%s has no control URL", snap.Validator.Name))
				continue
			}
			if err := m.client.PutRule(*snap.Validator.ControlURL, phase, action, nil, nil, delay); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", snap.Validator.Name, err))
				continue
			}
			applied++
		}

		status := fmt.Sprintf("set %s %s on %s", phase, action, targetLabel)
		if delay != "" {
			status = fmt.Sprintf("set %s delay=%s on %s", phase, delay, targetLabel)
		}
		return batchActionResult(status, applied, failures)
	}
}

func (m tuiModel) ruleTargets() []valcontrol.ValidatorSnapshot {
	return pickRuleTargets(m.snapshots, m.table.Cursor(), m.selectedValidators)
}

func (m tuiModel) selectedCount() int {
	count := 0
	for _, snap := range m.snapshots {
		if snap.Validator == nil {
			continue
		}
		if _, ok := m.selectedValidators[snap.Validator.Name]; ok {
			count++
		}
	}
	return count
}

func (m tuiModel) ruleTargetSummary() string {
	targets := m.ruleTargets()
	if len(targets) == 0 {
		return "none"
	}
	names := targetNames(targets)
	if len(m.selectedValidators) == 0 {
		return names[0]
	}
	if len(names) <= 3 {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%d selected validators", len(names))
}

func pickRuleTargets(snapshots []valcontrol.ValidatorSnapshot, cursor int, selected map[string]struct{}) []valcontrol.ValidatorSnapshot {
	if len(selected) > 0 {
		targets := make([]valcontrol.ValidatorSnapshot, 0, len(selected))
		for _, snap := range snapshots {
			if snap.Validator == nil {
				continue
			}
			if _, ok := selected[snap.Validator.Name]; ok {
				targets = append(targets, snap)
			}
		}
		if len(targets) > 0 {
			return targets
		}
	}
	if cursor < 0 || cursor >= len(snapshots) {
		return nil
	}
	return []valcontrol.ValidatorSnapshot{snapshots[cursor]}
}

func shouldClearDropForTargets(targets []valcontrol.ValidatorSnapshot, phase string) bool {
	if len(targets) == 0 {
		return false
	}
	for _, snap := range targets {
		rule := (*valcontrol.RuleView)(nil)
		if snap.Signer != nil {
			rule = snap.Signer.Rules[phase]
		}
		if rule == nil || rule.Action != "drop" {
			return false
		}
	}
	return true
}

func targetNames(targets []valcontrol.ValidatorSnapshot) []string {
	names := make([]string, 0, len(targets))
	for _, snap := range targets {
		if snap.Validator != nil {
			names = append(names, snap.Validator.Name)
		}
	}
	return names
}

func describeTargetCount(targets []valcontrol.ValidatorSnapshot) string {
	names := targetNames(targets)
	if len(names) == 1 {
		return names[0]
	}
	return fmt.Sprintf("%d validators", len(names))
}

func batchActionResult(status string, applied int, failures []string) actionResultMsg {
	if len(failures) == 0 {
		return actionResultMsg{status: status}
	}
	if applied > 0 {
		return actionResultMsg{err: fmt.Errorf("%s; failures: %s", status, strings.Join(failures, "; "))}
	}
	return actionResultMsg{err: fmt.Errorf("failures: %s", strings.Join(failures, "; "))}
}

func (m tuiModel) stopValidatorCmd() tea.Cmd {
	snap := m.selectedSnapshot()
	if snap == nil || snap.Validator == nil {
		return nil
	}
	inv := m.inv
	name := snap.Validator.Name
	lib := m.scenarioLib
	return func() tea.Msg {
		libPath, err := resolveScenarioLib(lib)
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("stop %s: %w", name, err)}
		}
		script := buildScenarioFuncScript(libPath, inv, fmt.Sprintf("stop_validator %q", name))
		if out, err := runBashScript(script); err != nil {
			return actionResultMsg{err: fmt.Errorf("stop %s: %w\n%s", name, err, out)}
		}
		return actionResultMsg{status: fmt.Sprintf("stopped %s", name)}
	}
}

func (m tuiModel) startValidatorCmd() tea.Cmd {
	snap := m.selectedSnapshot()
	if snap == nil || snap.Validator == nil {
		return nil
	}
	inv := m.inv
	name := snap.Validator.Name
	return func() tea.Msg {
		if err := restartlessStartValidator(inv, m.scenarioLib, name); err != nil {
			return actionResultMsg{err: fmt.Errorf("start %s: %w", name, err)}
		}
		return actionResultMsg{status: fmt.Sprintf("started %s", name)}
	}
}

func (m tuiModel) resetValidatorCmd(safe bool) tea.Cmd {
	snap := m.selectedSnapshot()
	if snap == nil || snap.Validator == nil {
		return func() tea.Msg { return actionResultMsg{err: fmt.Errorf("no validator selected")} }
	}
	inv := m.inv
	name := snap.Validator.Name
	lib := m.scenarioLib
	return func() tea.Msg {
		libPath, err := resolveScenarioLib(lib)
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("reset %s: %w", name, err)}
		}
		fn := "reset_validator"
		if safe {
			fn = "safe_reset_validator"
		}
		script := buildScenarioFuncScript(libPath, inv, fmt.Sprintf("%s %q", fn, name))
		if out, err := runBashScript(script); err != nil {
			return actionResultMsg{err: fmt.Errorf("reset %s: %w\n%s", name, err, out)}
		}
		label := "clean reset"
		if safe {
			label = "safe reset"
		}
		return actionResultMsg{status: fmt.Sprintf("%s %s done", label, name)}
	}
}

func restartlessStartValidator(inv *valcontrol.Inventory, scenarioLib, name string) error {
	libPath, err := resolveScenarioLib(scenarioLib)
	if err != nil {
		return err
	}
	script := buildScenarioFuncScript(libPath, inv, fmt.Sprintf("start_validator %q", name))
	if out, err := runBashScript(script); err != nil {
		return fmt.Errorf("%w\n%s", err, out)
	}
	return nil
}

func (m tuiModel) destroyNetworkCmd() tea.Cmd {
	inv := m.inv
	lib := m.scenarioLib
	return func() tea.Msg {
		libPath, err := resolveScenarioLib(lib)
		if err != nil {
			return destroyDoneMsg{err: err}
		}
		body := "scenario_finish\nrm -rf \"${SCENARIO_DIR}\""
		script := buildScenarioFuncScript(libPath, inv, body)
		if out, err := runBashScript(script); err != nil {
			return destroyDoneMsg{err: fmt.Errorf("destroy: %w\n%s", err, out)}
		}
		return destroyDoneMsg{}
	}
}

// buildScenarioFuncScript sources scenario.sh and reconstructs the minimal state
// from the inventory so that scenario functions (stop_validator, start_validator,
// scenario_finish, …) work without re-running the full bootstrap.
func buildScenarioFuncScript(libPath string, inv *valcontrol.Inventory, body string) string {
	var b strings.Builder
	fmt.Fprintln(&b, "#!/usr/bin/env bash")
	fmt.Fprintln(&b, "set -euo pipefail")
	fmt.Fprintf(&b, "source %q\n", libPath)
	fmt.Fprintf(&b, "SCENARIO_NAME=%q\n", inv.Scenario)
	fmt.Fprintf(&b, "PROJECT_NAME=%q\n", filepath.Base(inv.WorkDir))
	fmt.Fprintf(&b, "SCENARIO_DIR=%q\n", inv.WorkDir)
	fmt.Fprintf(&b, "COMPOSE_FILE=%q\n", inv.ComposeFile)
	for _, v := range inv.Validators {
		svc := v.Service
		if svc == "" {
			svc = v.Name
		}
		fmt.Fprintf(&b, "NODE_SERVICE[%s]=%q\n", v.Name, svc)
		if u, err := url.Parse(v.RPCURL); err == nil {
			if port := u.Port(); port != "" {
				fmt.Fprintf(&b, "NODE_RPC_PORT[%s]=%q\n", v.Name, port)
			}
		}
		fmt.Fprintf(&b, "NODE_DATA_DIR[%s]=%q\n", v.Name,
			filepath.Join(inv.WorkDir, "nodes", v.Name))
	}
	fmt.Fprintln(&b, body)
	return b.String()
}

// ── Shared commands ───────────────────────────────────────────────────────────

func fetchSnapshotsCmd(inv *valcontrol.Inventory, client *valcontrol.Client) tea.Cmd {
	return func() tea.Msg {
		snaps := make([]valcontrol.ValidatorSnapshot, 0, len(inv.Validators))
		for _, validator := range inv.Validators {
			snaps = append(snaps, client.Snapshot(validator))
		}
		return snapshotsMsg(snaps)
	}
}

func refreshTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}
