package main

// A simple program demonstrating the text input component from the Bubbles
// component library.

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flynn/go-shlex"
	"github.com/yzp0n/ncdn/controller/control"
)

var controlPlane = flag.String("controlPlane", "http://localhost:8080", "Control plane URL for the controller")

func main() {
	flag.Parse()
	cplaneURL, err := url.Parse(*controlPlane)
	if err != nil {
		log.Fatalf("Invalid control plane URL: %v", err)
	}
	if cplaneURL.Scheme != "http" && cplaneURL.Scheme != "https" {
		log.Fatalf("Control plane URL must use http or https scheme, got: %s", cplaneURL.Scheme)
	}
	fileSendChan := make(chan fileSendRequest)
	p := tea.NewProgram(initialModel(fileSendChan))
	go func() {
		for req := range fileSendChan {
			func(req fileSendRequest) {
				file, err := os.Open(req.source)
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to open file %s: %w", req.source, err)))
					return
				}
				defer file.Close()
				target := fmt.Sprintf("%s/file?lbType=%s&serverID=%s&path=%s&permission=%s",
					*controlPlane, url.QueryEscape(req.lbType),
					url.QueryEscape(req.serverID), url.QueryEscape(req.path),
					url.QueryEscape(req.permission))
				p.Send(logUpdate(fmt.Sprintf("Sending file %s to %s", req.source, target)))
				resp, err := http.Post(target, "application/octet-stream", file)
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to send file %s: %w", req.source, err)))
					return
				}
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to read response body: %w", err)))
					return
				}
				if resp.StatusCode != http.StatusAccepted {
					p.Send(errMsg(fmt.Errorf("failed to send file %s, server returned status: %s, %s", req.source, resp.Status, body)))
					return
				}
				p.Send(logUpdate(fmt.Sprintf("Control plane accepted to send %s to %s/%s/%s with permission bit %s",
					req.source, req.lbType, req.serverID, req.path, req.permission)))
			}(req)
		}
	}()
	go func() {
		p.Send(logUpdate("Connecting to control plane..."))
		for {
			logStream, err := http.Get(cplaneURL.String() + "/logs")
			if err != nil {
				p.Send(errMsg(fmt.Errorf("failed to connect to control plane: %w", err)))
				return
			}
			defer logStream.Body.Close()
			scanner := bufio.NewScanner(logStream.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					p.Send(logUpdate(line[6:])) // Remove "data: " prefix
				} else {
					p.Send(logUpdate(line))
				}
			}
			if err := scanner.Err(); err != nil {
				p.Send(errMsg(fmt.Errorf("error reading log stream: %w", err)))
				return
			}
			time.Sleep(1 * time.Second) // wait before reconnecting
		}
	}()
	go func() {
		startTime := time.Now()
		p.Send(tableUpdate{
			stat:   &control.ControllerStatus{},
			uptime: time.Since(startTime),
		})
		for {
			resp, err := http.Get(cplaneURL.String() + "/status")
			if err != nil {
				p.Send(errMsg(fmt.Errorf("failed to connect to control plane: %w", err)))
				return
			}
			if resp.StatusCode != http.StatusOK {
				p.Send(errMsg(fmt.Errorf("control plane returned status: %s", resp.Status)))
				return
			}
			var status control.ControllerStatus
			if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
				p.Send(errMsg(fmt.Errorf("failed to decode control plane response: %w", err)))
				return
			}
			resp.Body.Close()
			uptime := time.Since(startTime)
			p.Send(tableUpdate{
				stat:   &status,
				uptime: uptime,
			})
			time.Sleep(1 * time.Second) // Poll every 1 second
		}
	}()
	if _, err := p.Run(); err != nil {
		log.Fatalf("Bubble Tea program failed: %v", err)
	}
}

type (
	errMsg error
)

type fileSendRequest struct {
	source     string
	lbType     string
	serverID   string
	path       string
	permission string
}

type model struct {
	cmdline           textinput.Model
	logOutput         viewport.Model
	statView          table.Model
	connectionEntries table.Model
	commandOutput     viewport.Model
	err               error
	buffer            []string
	fileSender        chan fileSendRequest
}

type logUpdate string
type commandUpdate string
type tableUpdate struct {
	stat   *control.ControllerStatus
	uptime time.Duration
}

func initialModel(fileSender chan fileSendRequest) model {
	ti := textinput.New()
	ti.Placeholder = "Command?"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20
	ti.Prompt = "> "
	vp := viewport.New(80, 20)
	tableModel := table.New()
	tableModel.SetColumns([]table.Column{
		{Title: "Metric", Width: 20},
		{Title: "Value", Width: 20},
	})
	connEntries := table.New()
	connEntries.SetColumns([]table.Column{
		{Title: "Type", Width: 4},
		{Title: "Server ID", Width: 10},
		{Title: "Address", Width: 20},
		{Title: "Uptime", Width: 10},
		{Title: "Load", Width: 10},
	})
	cmdVp := viewport.New(80, 5)
	cmdVp.SetContent("Command output will appear here.")

	return model{
		cmdline:           ti,
		logOutput:         vp,
		statView:          tableModel,
		connectionEntries: connEntries,
		commandOutput:     cmdVp,
		err:               nil,
		buffer:            make([]string, 0, 100),
		fileSender:        fileSender,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updateLog := func() {
		var width = m.logOutput.Width
		var buf strings.Builder
		for _, line := range m.buffer {
			for len(line) > width {
				buf.WriteString(line[:width] + "\n")
				line = line[width:]
			}
			buf.WriteString(line)
		}
		m.logOutput.SetContent(buf.String())
		m.logOutput.GotoBottom()
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			cmd := m.cmdline.Value()
			m.cmdline.Reset()
			parsedCmd, err := shlex.Split(cmd)
			setContent := func(content string) {
				m.commandOutput.SetContent(fmt.Sprintf("command> %s\n%s", cmd, content))
				m.commandOutput.GotoTop()
			}
			if err != nil {
				setContent(fmt.Sprintf("Error parsing command: %v", err))
			} else if len(parsedCmd) == 0 {
			} else {
				switch parsedCmd[0] {
				case "quit", "exit":
					return m, tea.Quit
				case "pwd":
					wd, err := os.Getwd()
					if err != nil {
						setContent(fmt.Sprintf("Error getting current directory: %v", err))
					} else {
						setContent(wd)
					}
				case "cd":
					if len(parsedCmd) < 2 {
						setContent("Usage: cd <directory>")
					} else {
						if err := os.Chdir(parsedCmd[1]); err != nil {
							setContent(fmt.Sprintf("Error changing directory: %v", err))
						} else {
							setContent(fmt.Sprintf("Changed directory to %s", parsedCmd[1]))
						}
					}
				case "why":
					setContent("Because you were a spoiled kid...")
				case "sendfile":
					if len(parsedCmd) < 5 {
						setContent("Usage: sendfile <source> <lbType> <serverID> <path> <permission>")

					} else {
						source := parsedCmd[1]
						lbType := parsedCmd[2]
						serverID := parsedCmd[3]
						path := parsedCmd[4]
						permission := parsedCmd[5]
						setContent(fmt.Sprintf("sending file %s to %s/%s/%s with permission %s requested", source, lbType, serverID, path, permission))
						fileSender := m.fileSender
						go func() {
							fileSender <- fileSendRequest{
								source:     source,
								lbType:     lbType,
								serverID:   serverID,
								path:       path,
								permission: permission,
							}
						}()
					}
				default:
					setContent(fmt.Sprintf("Unknown command: %s", parsedCmd[0]))
				}
			}
		}
	case logUpdate:
		if msg != "" {
			m.buffer = append(m.buffer, string(msg)+"\n")
			if len(m.buffer) > 100 {
				m.buffer = m.buffer[len(m.buffer)-100:]
			}
			updateLog()
		}
	case errMsg:
		m.buffer = append(m.buffer, fmt.Sprintf("Error: %v\n", msg))
		if len(m.buffer) > 100 {
			m.buffer = m.buffer[len(m.buffer)-100:]
		}
		updateLog()
	case tableUpdate:
		m.statView.SetRows([]table.Row{
			{"L4 Load Balancers", fmt.Sprintf("%d", len(msg.stat.L4LBData))},
			{"L7 Load Balancers", fmt.Sprintf("%d", len(msg.stat.L7LBData))},
			{"Uptime", msg.uptime.String()},
		})
		m.statView.SetHeight(5)
		m.statView.SetWidth(m.logOutput.Width)
		rows := make([]table.Row, 0, len(msg.stat.L7LBData))
		for _, lb := range msg.stat.L4LBData {
			rows = append(rows, table.Row{
				"L4",
				fmt.Sprintf("%d", lb.Data.Data.ServerID),
				netip.AddrFrom4(lb.Data.Data.Address).String(),
				time.Duration(lb.Stat.Uptime).String(),
				fmt.Sprintf("%.2f", lb.Stat.LoadAvg),
			})
		}
		for _, lb := range msg.stat.L7LBData {
			rows = append(rows, table.Row{
				"L7",
				fmt.Sprintf("%d", lb.Data.Data.ServerID),
				netip.AddrFrom4(lb.Data.Data.Address).String(),
				time.Duration(lb.Stat.Uptime).String(),
				fmt.Sprintf("%.2f", lb.Stat.LoadAvg),
			})
		}
		m.connectionEntries.SetRows(rows)
		m.connectionEntries.SetHeight(10)
		m.connectionEntries.SetWidth(m.logOutput.Width)
	case tea.WindowSizeMsg:
		m.logOutput.Width = msg.Width / 2   // ウィンドウの幅をセット
		m.logOutput.Height = msg.Height - 9 // 必要なら他のUI分を引く
		updateLog()
		m.commandOutput.Width = msg.Width/2 - 10
		m.commandOutput.Height = 5
	}

	var (
		cmd1 tea.Cmd
		cmd2 tea.Cmd
		cmd3 tea.Cmd
		cmd4 tea.Cmd
		cmd5 tea.Cmd
	)
	m.cmdline, cmd1 = m.cmdline.Update(msg)
	m.logOutput, cmd2 = m.logOutput.Update(msg)
	m.statView, cmd3 = m.statView.Update(msg)
	m.connectionEntries, cmd4 = m.connectionEntries.Update(msg)
	m.commandOutput, cmd5 = m.commandOutput.Update(msg)
	return m, tea.Batch(
		cmd1,
		cmd2,
		cmd3,
		cmd4,
		cmd5,
	)
}

func (m model) View() string {
	style := lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	return fmt.Sprintf(
		"%s\n\nNCDN Controller\n\n%s\n\n%s",
		lipgloss.JoinHorizontal(lipgloss.Top, style.Render(m.logOutput.View()),
			lipgloss.JoinVertical(lipgloss.Left, m.statView.View(), m.connectionEntries.View(), style.Render(m.commandOutput.View()))),
		m.cmdline.View(),
		"(esc to quit)",
	) + "\n"
}
