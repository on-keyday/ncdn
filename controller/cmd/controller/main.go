package main

// A simple program demonstrating the text input component from the Bubbles
// component library.

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

func main() {
	lis, err := wstransport.NewWebSocketListener(":8080")
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
	if _, err := p.Run(); err != nil {
		log.Fatalf("Bubble Tea program failed: %v", err)
	}
}

type (
	errMsg error
)

type model struct {
	textInput textinput.Model
	err       error
	buffer    []string
}

type writerUpdate string

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Command?"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20

	return model{
		textInput: ti,
		err:       nil,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmd2 tea.Cmd
	var cmd3 tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			cmd := m.textInput.Value()
			m.textInput.Reset()
			switch cmd {
			case "quit", "exit":
				return m, tea.Quit
			}
		}
	case writerUpdate:
		m.buffer = append(m.buffer, string(msg))
	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, tea.Batch(cmd, cmd2, cmd3)
}

func (m model) View() string {
	return fmt.Sprintf(
		"%s\n\nNCDN Controller\n\n%s\n\n%s",
		strings.Join(m.buffer, ""),
		m.textInput.View(),
		"(esc to quit)",
	) + "\n"
}
