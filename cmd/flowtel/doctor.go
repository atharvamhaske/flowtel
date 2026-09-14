package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"github.com/atharvamhaske/flowtel/internal/config"
	"github.com/atharvamhaske/flowtel/internal/daemon"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

type checkStatus int

const (
	statusPass checkStatus = iota
	statusWarn
	statusFail
)

type checkResult struct {
	status  checkStatus
	message string
}

type doctorCheck struct {
	name string
	run  func() checkResult
}

func runDoctor(args []string) error {
	defaults, err := daemon.DefaultConfig()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	socketPath := flags.String("socket", defaults.SocketPath, "local Unix socket path")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse doctor flags: %w", err)
	}

	checks := []doctorCheck{
		{"daemon socket", func() checkResult { return checkDaemonSocket(*socketPath) }},
		{"pi binary", checkPiBinary},
		{"collector endpoint", checkCollectorEndpoint},
		{"FLOWTEL_* env vars", checkEnvVars},
	}

	if !term.IsTerminal(os.Stdout.Fd()) {
		return runDoctorPlain(checks)
	}
	model := newDoctorModel(checks)
	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return err
	}
	if final, ok := finalModel.(doctorModel); ok && final.anyFailed() {
		os.Exit(1)
	}
	return nil
}

func runDoctorPlain(checks []doctorCheck) error {
	failed := false
	for _, check := range checks {
		result := check.run()
		fmt.Printf("%s %s: %s\n", statusGlyph(result.status), check.name, result.message)
		if result.status == statusFail {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
	return nil
}

func statusGlyph(status checkStatus) string {
	switch status {
	case statusPass:
		return "PASS"
	case statusWarn:
		return "WARN"
	default:
		return "FAIL"
	}
}

// --- individual checks, each read-only, no side effects ---

func checkDaemonSocket(socketPath string) checkResult {
	client := &statusClient{socketPath: socketPath}
	status, err := client.fetch()
	if err != nil {
		return checkResult{status: statusWarn, message: fmt.Sprintf("not running at %s (start it with `flowctl daemon serve`)", socketPath)}
	}
	return checkResult{status: statusPass, message: fmt.Sprintf("reachable, version %s, queued=%d", status.DaemonVersion, status.Queued)}
}

func checkPiBinary() checkResult {
	bin := os.Getenv("FLOWTEL_PI")
	if bin != "" {
		if _, err := os.Stat(bin); err != nil {
			return checkResult{status: statusFail, message: fmt.Sprintf("FLOWTEL_PI=%s but it doesn't exist: %v", bin, err)}
		}
		return checkResult{status: statusPass, message: fmt.Sprintf("using FLOWTEL_PI override: %s", bin)}
	}
	path, err := exec.LookPath("pi")
	if err != nil {
		return checkResult{status: statusFail, message: "not found on PATH (install pi, or set FLOWTEL_PI to its path)"}
	}
	return checkResult{status: statusPass, message: path}
}

func checkCollectorEndpoint() checkResult {
	endpoint := os.Getenv(config.EnvOTLPEndpoint)
	if endpoint == "" {
		return checkResult{status: statusWarn, message: fmt.Sprintf("%s is not set — required for `flowtel ingest`, optional for the daemon path", config.EnvOTLPEndpoint)}
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return checkResult{status: statusFail, message: fmt.Sprintf("%s=%q is not a valid URL", config.EnvOTLPEndpoint, endpoint)}
	}
	host := parsed.Host
	if parsed.Port() == "" {
		if parsed.Scheme == "https" {
			host = net.JoinHostPort(parsed.Hostname(), "443")
		} else {
			host = net.JoinHostPort(parsed.Hostname(), "80")
		}
	}
	connection, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		return checkResult{status: statusFail, message: fmt.Sprintf("cannot reach %s: %v", host, err)}
	}
	_ = connection.Close()
	return checkResult{status: statusPass, message: fmt.Sprintf("reachable at %s", host)}
}

func checkEnvVars() checkResult {
	var problems []string
	if os.Getenv(config.EnvHarness) == "" {
		problems = append(problems, config.EnvHarness+" is unset")
	}
	if raw := os.Getenv(config.EnvProfile); raw != "" && !model.Profile(raw).Valid() {
		problems = append(problems, fmt.Sprintf("%s=%q is not one of openinference, gen_ai, both", config.EnvProfile, raw))
	}
	if raw := os.Getenv(config.EnvExportTimeout); raw != "" {
		if seconds, err := strconv.Atoi(raw); err != nil || seconds < 0 {
			problems = append(problems, fmt.Sprintf("%s=%q is not a non-negative integer", config.EnvExportTimeout, raw))
		}
	}
	if len(problems) == 0 {
		return checkResult{status: statusPass, message: "all set correctly"}
	}
	encoded, _ := json.Marshal(problems)
	return checkResult{status: statusFail, message: string(encoded)}
}

// --- TUI model ---

var (
	doctorPass  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	doctorWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	doctorFail  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	doctorLabel = lipgloss.NewStyle().Bold(true)
	doctorMuted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	doctorBox   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

type doctorModel struct {
	checks  []doctorCheck
	results []*checkResult
	current int
	spin    spinner.Model
}

func newDoctorModel(checks []doctorCheck) doctorModel {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = doctorMuted
	return doctorModel{checks: checks, results: make([]*checkResult, len(checks)), spin: spin}
}

func (m doctorModel) anyFailed() bool {
	for _, result := range m.results {
		if result != nil && result.status == statusFail {
			return true
		}
	}
	return false
}

type checkDoneMsg struct {
	index  int
	result checkResult
}

func (m doctorModel) runCheck(index int) tea.Cmd {
	return func() tea.Msg {
		return checkDoneMsg{index: index, result: m.checks[index].run()}
	}
}

func (m doctorModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.runCheck(0))
}

func (m doctorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.KeyMsg:
		if typed.String() == "q" || typed.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case checkDoneMsg:
		result := typed.result
		m.results[typed.index] = &result
		m.current = typed.index + 1
		if m.current >= len(m.checks) {
			return m, tea.Quit
		}
		return m, m.runCheck(m.current)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(typed)
		return m, cmd
	}
	return m, nil
}

func (m doctorModel) View() string {
	lines := make([]string, 0, len(m.checks))
	for i, check := range m.checks {
		result := m.results[i]
		switch {
		case result != nil && result.status == statusPass:
			lines = append(lines, doctorPass.Render("✓")+" "+doctorLabel.Render(check.name)+"  "+doctorMuted.Render(result.message))
		case result != nil && result.status == statusWarn:
			lines = append(lines, doctorWarn.Render("!")+" "+doctorLabel.Render(check.name)+"  "+doctorMuted.Render(result.message))
		case result != nil && result.status == statusFail:
			lines = append(lines, doctorFail.Render("✗")+" "+doctorLabel.Render(check.name)+"  "+doctorMuted.Render(result.message))
		case i == m.current:
			lines = append(lines, m.spin.View()+" "+doctorLabel.Render(check.name))
		default:
			lines = append(lines, doctorMuted.Render("○")+" "+doctorMuted.Render(check.name))
		}
	}
	return doctorBox.Render(strings.Join(lines, "\n")) + "\n"
}
