package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"github.com/atharvamhaske/flowtel/internal/daemon"
)

var (
	statusLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	statusValue = lipgloss.NewStyle().Bold(true)
	statusError = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	statusBox   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func runStatus(args []string) error {
	defaults, err := daemon.DefaultConfig()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	socketPath := flags.String("socket", defaults.SocketPath, "local Unix socket path")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse status flags: %w", err)
	}

	client := &statusClient{socketPath: *socketPath}
	if !term.IsTerminal(os.Stdout.Fd()) {
		status, err := client.fetch()
		if err != nil {
			return err
		}
		fmt.Println(formatStatusLine(status))
		return nil
	}

	program := tea.NewProgram(newStatusModel(client))
	_, err = program.Run()
	return err
}

type statusClient struct {
	socketPath string
}

func (c *statusClient) fetch() (daemon.Status, error) {
	connection, err := net.DialTimeout("unix", c.socketPath, 2*time.Second)
	if err != nil {
		return daemon.Status{}, fmt.Errorf("daemon not running at %s", c.socketPath)
	}
	defer func() { _ = connection.Close() }()
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))

	scanner := newDaemonScanner(connection)

	if err := sendRequest(connection, 1, "initialize", map[string]any{
		"protocol_version": daemon.ProtocolVersion,
		"client":           daemon.Client{Source: "status-cli"},
	}); err != nil {
		return daemon.Status{}, err
	}
	if _, err := readResponse(scanner); err != nil {
		return daemon.Status{}, fmt.Errorf("initialize daemon connection: %w", err)
	}

	if err := sendRequest(connection, 2, "status.get", nil); err != nil {
		return daemon.Status{}, err
	}
	result, err := readResponse(scanner)
	if err != nil {
		return daemon.Status{}, fmt.Errorf("fetch daemon status: %w", err)
	}
	var status daemon.Status
	if err := json.Unmarshal(result, &status); err != nil {
		return daemon.Status{}, fmt.Errorf("decode daemon status: %w", err)
	}
	return status, nil
}

func newDaemonScanner(connection net.Conn) *bufio.Scanner {
	scanner := bufio.NewScanner(connection)
	scanner.Buffer(make([]byte, 4096), daemon.DefaultMaxLineBytes)
	return scanner
}

func sendRequest(connection net.Conn, id int, method string, params any) error {
	request := daemon.Request{JSONRPC: "2.0", ID: json.RawMessage(fmt.Sprintf("%d", id)), Method: method}
	if params != nil {
		encoded, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("encode %s params: %w", method, err)
		}
		request.Params = encoded
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode %s request: %w", method, err)
	}
	if _, err := connection.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("send %s request: %w", method, err)
	}
	return nil
}

func readResponse(scanner *bufio.Scanner) (json.RawMessage, error) {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("daemon closed connection")
	}
	var response daemon.Response
	if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if response.Error != nil {
		return nil, fmt.Errorf("daemon error: %s", response.Error.Message)
	}
	encoded, err := json.Marshal(response.Result)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func formatStatusLine(status daemon.Status) string {
	if status.LastError != "" {
		return fmt.Sprintf("flowctl daemon %s: queued=%d events_stored=%d last_error=%q",
			status.DaemonVersion, status.Queued, status.EventsStored, status.LastError)
	}
	return fmt.Sprintf("flowctl daemon %s: queued=%d events_stored=%d",
		status.DaemonVersion, status.Queued, status.EventsStored)
}

const statusPollInterval = time.Second

type statusModel struct {
	client *statusClient
	status daemon.Status
	err    error
}

type statusMsg struct {
	status daemon.Status
	err    error
}

func newStatusModel(client *statusClient) statusModel {
	return statusModel{client: client}
}

func (m statusModel) Init() tea.Cmd {
	return m.poll()
}

func (m statusModel) poll() tea.Cmd {
	return func() tea.Msg {
		status, err := m.client.fetch()
		return statusMsg{status: status, err: err}
	}
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.KeyMsg:
		if typed.String() == "q" || typed.String() == "ctrl+c" || typed.String() == "esc" {
			return m, tea.Quit
		}
	case statusMsg:
		m.status, m.err = typed.status, typed.err
		return m, tea.Tick(statusPollInterval, func(time.Time) tea.Msg { return m.poll()() })
	}
	return m, nil
}

func (m statusModel) View() string {
	if m.err != nil {
		return statusBox.Render(statusError.Render(m.err.Error())) + "\nq to quit\n"
	}
	lines := []string{
		statusLabel.Render("daemon  ") + statusValue.Render(m.status.DaemonVersion),
		statusLabel.Render("queued  ") + statusValue.Render(fmt.Sprintf("%d", m.status.Queued)),
		statusLabel.Render("stored  ") + statusValue.Render(fmt.Sprintf("%d", m.status.EventsStored)),
	}
	if m.status.LastError != "" {
		lines = append(lines, statusError.Render("error   "+m.status.LastError))
	}
	return statusBox.Render(strings.Join(lines, "\n")) + "\nq to quit\n"
}
