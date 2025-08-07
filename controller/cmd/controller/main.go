package main

// A simple program demonstrating the text input component from the Bubbles
// component library.

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yzp0n/ncdn/controller/control"
	wstransport "github.com/yzp0n/ncdn/controller/transport/websocket"
)

type lockedWriter struct {
	l sync.Mutex
	p *tea.Program
}

func (lw *lockedWriter) Write(p []byte) (n int, err error) {
	lw.l.Lock()
	defer lw.l.Unlock()
	lw.p.Send(writerUpdate(string(p)))
	return len(p), nil
}

var port = flag.String("port", ":8080", "Port to run the controller on")

func main() {
	flag.Parse()
	lis, err := wstransport.NewWebSocketListener(*port)
	if err != nil {
		log.Fatalf("Failed to create WebSocket listener: %v", err)
	}
	defer lis.Close()
	p := tea.NewProgram(initialModel())
	lw := &lockedWriter{p: p}
	h := slog.New(slog.NewTextHandler(lw, nil))
	controller := control.NewController(h)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		lw.Write([]byte("Starting controller...\n"))
		if err := controller.Run(ctx, lis); err != nil {
			log.Fatalf("Controller run failed: %v", err)
		}
	}()
	go func() {
		startTime := time.Now()
		for {
			status := controller.Status()
			p.Send(tableUpdate{
				stat:   status,
				uptime: time.Since(startTime),
			})
			time.Sleep(1 * time.Second) // Update every 1 second
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
	return textinput.Blink
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
		m.buffer = append(m.buffer, string(msg))
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
			})
		}
		for _, lb := range msg.stat.L7LBData {
			rows = append(rows, table.Row{
				fmt.Sprintf("%d", lb.Data.Data.ServerID),
				netip.AddrFrom4(lb.Data.Data.Address).String(),
				"L7",
				time.Duration(lb.Stat.Uptime).String(),
			})
		}
		m.connectionEntries.SetRows(rows)
		m.connectionEntries.SetHeight(10)
		m.connectionEntries.SetWidth(m.logOutput.Width)
	case tea.WindowSizeMsg:
		m.logOutput.Width = msg.Width / 2    // ウィンドウの幅をセット
		m.logOutput.Height = msg.Height - 10 // 必要なら他のUI分を引く
		updateLog()
	case errMsg:
		m.err = msg
		return m, nil
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
