package main

// A simple program demonstrating the text input component from the Bubbles
// component library.

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yzp0n/ncdn/controller/control"
)

var controlPlane = flag.String("controlPlane", "http://localhost:8080", "Control plane URL for the controller")

func main() {
	flag.Parse()
	url, err := url.Parse(*controlPlane)
	if err != nil {
		log.Fatalf("Invalid control plane URL: %v", err)
	}
	if url.Scheme != "http" && url.Scheme != "https" {
		log.Fatalf("Control plane URL must use http or https scheme, got: %s", url.Scheme)
	}
	p := tea.NewProgram(initialModel())
	go func() {
		p.Send(writerUpdate("Connecting to control plane..."))
		for {
			logStream, err := http.Get(url.String() + "/logs")
			if err != nil {
				p.Send(errMsg(fmt.Errorf("failed to connect to control plane: %w", err)))
				return
			}
			defer logStream.Body.Close()
			scanner := bufio.NewScanner(logStream.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					p.Send(writerUpdate(line[6:])) // Remove "data: " prefix
				} else {
					p.Send(writerUpdate(line))
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
			resp, err := http.Get(url.String() + "/status")
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

type model struct {
	cmdline           textinput.Model
	logOutput         viewport.Model
	statView          table.Model
	connectionEntries table.Model
	err               error
	buffer            []string
}

type writerUpdate string
type tableUpdate struct {
	stat   *control.ControllerStatus
	uptime time.Duration
}

func initialModel() model {
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
		{Title: "Server ID", Width: 10},
		{Title: "Address", Width: 20},
		{Title: "Type", Width: 4},
		{Title: "Uptime", Width: 10},
		{Title: "Load", Width: 10},
	})

	return model{
		cmdline:           ti,
		logOutput:         vp,
		statView:          tableModel,
		connectionEntries: connEntries,
		err:               nil,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmd2 tea.Cmd
	var cmd3 tea.Cmd
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
			switch cmd {
			case "quit", "exit":
				return m, tea.Quit
			}
		}
	case writerUpdate:
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
				fmt.Sprintf("%d", lb.Data.Data.ServerID),
				netip.AddrFrom4(lb.Data.Data.Address).String(),
				"L4",
				time.Duration(lb.Stat.Uptime).String(),
				fmt.Sprintf("%.2f%%", lb.Stat.LoadAvg),
			})
		}
		for _, lb := range msg.stat.L7LBData {
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", lb.Data.Data.ServerID),
				netip.AddrFrom4(lb.Data.Data.Address).String(),
				"L7",
				time.Duration(lb.Stat.Uptime).String(),
				fmt.Sprintf("%.2f%%", lb.Stat.LoadAvg),
			})
		}
		m.connectionEntries.SetRows(rows)
		m.connectionEntries.SetHeight(10)
		m.connectionEntries.SetWidth(m.logOutput.Width)
	case tea.WindowSizeMsg:
		m.logOutput.Width = msg.Width / 2    // ウィンドウの幅をセット
		m.logOutput.Height = msg.Height - 10 // 必要なら他のUI分を引く
		updateLog()
	}

	m.cmdline, cmd = m.cmdline.Update(msg)
	return m, tea.Batch(cmd, cmd2, cmd3)
}

func (m model) View() string {
	style := lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	return fmt.Sprintf(
		"%s\n\nNCDN Controller\n\n%s\n\n%s",
		lipgloss.JoinHorizontal(lipgloss.Top, style.Render(m.logOutput.View()),
			lipgloss.JoinVertical(lipgloss.Left, m.statView.View(), m.connectionEntries.View())),
		m.cmdline.View(),
		"(esc to quit)",
	) + "\n"
}
