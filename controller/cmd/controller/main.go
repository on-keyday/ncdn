package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flynn/go-shlex"
	"github.com/yzp0n/ncdn/controller/api"
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
	c := &channels{
		fileSender:  make(chan fileSendRequest, 10),
		wasmSender:  make(chan wasmDeployRequest, 10),
		wasmRemover: make(chan wasmRemoveRequest, 10),
		vipSender:   make(chan api.VIPUpdate, 10),
	}
	p := tea.NewProgram(initialModel(c))
	go func() {
		for req := range c.fileSender {
			func(req fileSendRequest) {
				file, err := os.Open(req.source)
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to open file %s: %w", req.source, err)))
					return
				}
				defer file.Close()
				upload := fmt.Sprintf("%s/upload", *controlPlane)
				p.Send(logUpdate(fmt.Sprintf("Sending file %s to %s", req.source, upload)))
				resp, err := http.Post(upload, "application/octet-stream", file)
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to send file %s: %w", req.source, err)))
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					p.Send(errMsg(fmt.Errorf("failed to upload file %s, status: %s", req.source, resp.Status)))
					return
				}
				var uploadResp api.UploadResponse
				if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
					p.Send(errMsg(fmt.Errorf("failed to decode upload response: %w", err)))
					return
				}
				p.Send(logUpdate(fmt.Sprintf("File %s uploaded successfully, ID: %d", req.source, uploadResp.ID)))
				fileTransferInfo := api.FileUploadBody{
					FileID: uploadResp.ID,
					Path:   req.path,
					Mode:   req.permission,
					Dest: control.DestInfo{
						DestEntries: req.dests,
					},
				}
				jsonData, err := json.Marshal(fileTransferInfo)
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to marshal file transfer info: %w", err)))
					return
				}
				transferURL := fmt.Sprintf("%s/file/transfer", *controlPlane)
				p.Send(logUpdate(fmt.Sprintf("Transferring file %s to %s", req.source, transferURL)))
				transferResp, err := http.Post(transferURL, "application/json", strings.NewReader(string(jsonData)))
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to transfer file %s: %w", req.source, err)))
					return
				}
				defer transferResp.Body.Close()
				if transferResp.StatusCode != http.StatusAccepted {
					p.Send(errMsg(fmt.Errorf("failed to transfer file %s, status: %s", req.source, transferResp.Status)))
					return
				}
				p.Send(logUpdate(fmt.Sprintf("File %s transferred successfully", req.source)))
			}(req)
		}
	}()
	go func() {
		for req := range c.wasmSender {
			func(req wasmDeployRequest) {
				file, err := os.Open(req.wasmFile)
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to open WASM file %s: %w", req.wasmFile, err)))
					return
				}
				defer file.Close()
				upload := fmt.Sprintf("%s/upload", *controlPlane)
				p.Send(logUpdate(fmt.Sprintf("Sending WASM file %s to %s", req.wasmFile, upload)))
				resp, err := http.Post(upload, "application/octet-stream", file)
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to send WASM file %s: %w", req.wasmFile, err)))
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					p.Send(errMsg(fmt.Errorf("failed to upload WASM file %s, status: %s", req.wasmFile, resp.Status)))
					return
				}
				var uploadResp api.UploadResponse
				if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
					p.Send(errMsg(fmt.Errorf("failed to decode upload response: %w", err)))
					return
				}
				p.Send(logUpdate(fmt.Sprintf("WASM file %s uploaded successfully, ID: %d", req.wasmFile, uploadResp.ID)))
				wasmInstallInfo := api.WasmInstallBody{
					FileID: uploadResp.ID,
					WasmID: req.wasmID,
					Method: req.method,
					Path:   req.path,
					Dest: control.DestInfo{
						DestEntries: req.dests,
					},
				}
				jsonData, err := json.Marshal(wasmInstallInfo)
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to marshal WASM install info: %w", err)))
					return
				}
				installURL := fmt.Sprintf("%s/wasm/install", *controlPlane)
				p.Send(logUpdate(fmt.Sprintf("Installing WASM %s with ID %d to %s", req.wasmFile, req.wasmID, installURL)))
				installResp, err := http.Post(installURL, "application/json", strings.NewReader(string(jsonData)))
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to install WASM %s: %w", req.wasmFile, err)))
					return
				}
				defer installResp.Body.Close()
				if installResp.StatusCode != http.StatusAccepted {
					p.Send(errMsg(fmt.Errorf("failed to install WASM %s, status: %s", req.wasmFile, installResp.Status)))
					return
				}
				p.Send(logUpdate(fmt.Sprintf("WASM %s with ID %d installed successfully", req.wasmFile, req.wasmID)))
			}(req)
		}
	}()
	go func() {
		for req := range c.wasmRemover {
			func(req wasmRemoveRequest) {
				wasmUninstallInfo := api.WasmUninstallBody{
					WasmID: req.wasmID,
					Dest: control.DestInfo{
						DestEntries: req.dests,
					},
				}
				jsonData, err := json.Marshal(wasmUninstallInfo)
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to marshal WASM uninstall info: %w", err)))
					return
				}
				uninstallURL := fmt.Sprintf("%s/wasm/uninstall", *controlPlane)
				p.Send(logUpdate(fmt.Sprintf("Uninstalling WASM with ID %d from %s", req.wasmID, uninstallURL)))
				resp, err := http.Post(uninstallURL, "application/json", strings.NewReader(string(jsonData)))
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to uninstall WASM with ID %d: %w", req.wasmID, err)))
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusAccepted {
					p.Send(errMsg(fmt.Errorf("failed to uninstall WASM with ID %d, status: %s", req.wasmID, resp.Status)))
					return
				}
				p.Send(logUpdate(fmt.Sprintf("WASM with ID %d uninstalled successfully", req.wasmID)))
			}(req)
		}
	}()
	go func() {
		for req := range c.vipSender {
			func(req api.VIPUpdate) {
				vip := req.VIP
				serverIDs := req.Dest.DestEntries
				if !vip.IsValid() {
					p.Send(errMsg(fmt.Errorf("invalid VIP address: %v", vip)))
					return
				}
				jsonData, err := json.Marshal(req)
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to marshal VIP update info: %w", err)))
					return
				}
				updateURL := fmt.Sprintf("%s/vip/update", *controlPlane)
				p.Send(logUpdate(fmt.Sprintf("Updating VIP to %s for servers %v at %s", vip, serverIDs, updateURL)))
				resp, err := http.Post(updateURL, "application/json", strings.NewReader(string(jsonData)))
				if err != nil {
					p.Send(errMsg(fmt.Errorf("failed to update VIP %s: %w", vip, err)))
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusAccepted {
					p.Send(errMsg(fmt.Errorf("failed to update VIP %s, status: %s", vip, resp.Status)))
					return
				}
				p.Send(logUpdate(fmt.Sprintf("VIP %s notified successfully for servers %v", vip, serverIDs)))
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
	path       string
	permission uint16
	dests      []control.DestEntry
}

type wasmDeployRequest struct {
	wasmFile string
	wasmID   uint32
	method   string
	path     string
	dests    []control.DestEntry
}

type wasmRemoveRequest struct {
	wasmID uint32
	dests  []control.DestEntry
}

type channels struct {
	fileSender  chan fileSendRequest
	wasmSender  chan wasmDeployRequest
	wasmRemover chan wasmRemoveRequest
	vipSender   chan api.VIPUpdate
}

type model struct {
	cmdline           textinput.Model
	logOutput         viewport.Model
	statView          table.Model
	connectionEntries table.Model
	commandOutput     viewport.Model
	err               error
	buffer            []string
	c                 *channels
}

type logUpdate string

type tableUpdate struct {
	stat   *control.ControllerStatus
	uptime time.Duration
}

func initialModel(c *channels) model {
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
		c:                 c,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink)
}

func parseServerIDs(serverIDs []string) ([]control.DestEntry, error) {
	var dests []control.DestEntry
	for _, serverID := range serverIDs {
		parts := strings.Split(serverID, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid server ID format: %s", serverID)
		}
		lbType := parts[0]
		if parts[1] == "*" {
			dests = append(dests, control.DestEntry{
				LBType:    control.LBType(lbType),
				ServerIDs: nil,
				Broadcast: true,
			})
			continue
		}
		idParts := strings.Split(parts[1], ",")
		var ids []uint32
		for _, idPart := range idParts {
			id, err := strconv.Atoi(idPart)
			if err != nil {
				return nil, fmt.Errorf("invalid server ID: %s", idPart)
			}
			if id > int(^uint32(0)) {
				return nil, fmt.Errorf("invalid server ID: %s", idPart)
			}
			ids = append(ids, uint32(id))
		}
		dests = append(dests, control.DestEntry{
			LBType:    control.LBType(lbType),
			ServerIDs: ids,
			Broadcast: false,
		})
	}
	if len(dests) == 0 {
		return nil, fmt.Errorf("no valid server IDs provided")
	}
	return dests, nil
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

	finalUpdate := func() (tea.Model, tea.Cmd) {
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
						setContent("Usage: sendfile <source> <dest> <permission> <serverID(lbType:id1,id2,...)>...")
					} else {
						source := parsedCmd[1]
						dest := parsedCmd[2]
						permission := parsedCmd[3]
						parsedPermission, err := strconv.ParseUint(permission, 8, 16)
						if err != nil {
							setContent(fmt.Sprintf("Invalid permission: %v", err))
							return finalUpdate()
						}
						serverIDs, err := parseServerIDs(parsedCmd[4:])
						if err != nil {
							setContent(fmt.Sprintf("Error parsing server IDs: %v", err))
							return finalUpdate()
						}
						setContent(fmt.Sprintf("sending file %s to %s with permission %o to servers %v", source, dest, parsedPermission, serverIDs))
						fileSender := m.c.fileSender
						go func() {
							fileSender <- fileSendRequest{
								source:     source,
								dests:      serverIDs,
								path:       dest,
								permission: uint16(parsedPermission),
							}
						}()
					}
				case "wasmdeploy":
					if len(parsedCmd) < 5 {
						setContent("Usage: wasmdeploy <wasmFile> <wasmID> <httpMethod> <httpPath> <serverID(lbType:id1,id2,...)>...")
					} else {
						wasmFile := parsedCmd[1]
						wasmID := parsedCmd[2]
						wasmIDUint, err := strconv.ParseUint(wasmID, 10, 32)
						if err != nil {
							setContent(fmt.Sprintf("Invalid WASM ID: %v", err))
							return finalUpdate()
						}
						httpMethod := parsedCmd[3]
						httpPath := parsedCmd[4]
						serverIDs, err := parseServerIDs(parsedCmd[5:])
						if err != nil {
							setContent(fmt.Sprintf("Error parsing server IDs: %v", err))
							return finalUpdate()
						}
						setContent(fmt.Sprintf("Deploying WASM file %s with ID %d using method %s at path %s to servers %v", wasmFile, wasmIDUint, httpMethod, httpPath, serverIDs))
						// Here you would implement the actual deployment logic
						wasmSender := m.c.wasmSender
						go func() {
							wasmSender <- wasmDeployRequest{
								wasmFile: wasmFile,
								wasmID:   uint32(wasmIDUint),
								method:   httpMethod,
								path:     httpPath,
								dests:    serverIDs,
							}
						}()
					}
				case "wasmremove":
					if len(parsedCmd) < 3 {
						setContent("Usage: wasmremove <wasmID> <serverID(lbType:id1,id2,...)>...")
					} else {
						wasmID := parsedCmd[1]
						wasmIDUint, err := strconv.ParseUint(wasmID, 10, 32)
						if err != nil {
							setContent(fmt.Sprintf("Invalid WASM ID: %v", err))
							return finalUpdate()
						}
						serverIDs, err := parseServerIDs(parsedCmd[2:])
						if err != nil {
							setContent(fmt.Sprintf("Error parsing server IDs: %v", err))
							return finalUpdate()
						}
						setContent(fmt.Sprintf("Removing WASM with ID %s from servers %v", wasmID, serverIDs))
						// Here you would implement the actual removal logic
						wasmRemover := m.c.wasmRemover
						go func() {
							wasmRemover <- wasmRemoveRequest{
								wasmID: uint32(wasmIDUint),
								dests:  serverIDs,
							}
						}()
					}
				case "vipupdate":
					if len(parsedCmd) < 3 {
						setContent("Usage: vipupdate <vip> <serverID(lbType:id1,id2,...)>...")
					} else {
						vipStr := parsedCmd[1]
						vip, err := netip.ParseAddr(vipStr)
						if err != nil {
							setContent(fmt.Sprintf("Invalid VIP address: %v", err))
							return finalUpdate()
						}
						serverIDs, err := parseServerIDs(parsedCmd[2:])
						if err != nil {
							setContent(fmt.Sprintf("Error parsing server IDs: %v", err))
							return finalUpdate()
						}
						setContent(fmt.Sprintf("Updating VIP to %s for servers %v", vip, serverIDs))
						// Here you would implement the actual VIP update logic
						vipSender := m.c.vipSender
						go func() {
							vipSender <- api.VIPUpdate{
								VIP: vip,
								Dest: control.DestInfo{
									DestEntries: serverIDs,
								},
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
		m.cmdline.Width = msg.Width/2 - 10
	}
	return finalUpdate()
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
