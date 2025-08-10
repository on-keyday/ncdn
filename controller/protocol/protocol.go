package protocol

import (
	"time"
)

func MakeChunkInfo(isChunkd bool, lenOrID uint32) ChunkInfo {
	info := ChunkInfo{}
	info.SetIsChunkd(isChunkd)
	info.SetLenOrId(lenOrID)
	return info
}

type DataWithStat[T interface {
	Clone() T
	Update(U)
}, U any] struct {
	Data        T
	Stat        *MachineStat
	MemoryTotal uint64
	DiskTotal   uint64
}

func (d *DataWithStat[T, U]) Clone() *DataWithStat[T, U] {
	clone := &DataWithStat[T, U]{
		Data:        d.Data.Clone(),
		Stat:        &MachineStat{},
		MemoryTotal: d.MemoryTotal,
		DiskTotal:   d.DiskTotal,
	}
	clone.Stat.Uptime = d.Stat.Uptime
	clone.Stat.CPUUsages = make([]float64, len(d.Stat.CPUUsages))
	copy(clone.Stat.CPUUsages, d.Stat.CPUUsages)
	clone.Stat.MemoryUsage = d.Stat.MemoryUsage
	clone.Stat.DiskUsage = d.Stat.DiskUsage
	clone.Stat.IOUtilization = d.Stat.IOUtilization
	clone.Stat.DiskSwap = d.Stat.DiskSwap
	clone.Stat.LoadAvg = d.Stat.LoadAvg
	return clone
}

func (d *DataWithStat[T, U]) Update(machine *MachineStat, lb U) {
	d.Stat.Uptime = machine.Uptime
	d.Stat.CPUUsages = make([]float64, len(machine.CPUUsages))
	copy(d.Stat.CPUUsages, machine.CPUUsages)
	d.Stat.MemoryUsage = machine.MemoryUsage
	d.Stat.DiskUsage = machine.DiskUsage
	d.Stat.IOUtilization = machine.IOUtilization
	d.Stat.DiskSwap = machine.DiskSwap
	d.Stat.LoadAvg = machine.LoadAvg
	d.Data.Update(lb)
}

type L7LBData struct {
	ServerID   uint32
	Address    [4]byte
	MacAddress [6]byte
	Ports      []uint16
}

type L7LBWithStat struct {
	Data        L7LBData
	Throughput  uint64
	PacketTotal uint64
	DropCount   uint64
	PortStats   []*PortStat
}

func (l *L7LBWithStat) Clone() *L7LBWithStat {
	clone := &L7LBWithStat{
		Data: L7LBData{
			ServerID:   l.Data.ServerID,
			Address:    l.Data.Address,
			MacAddress: l.Data.MacAddress,
			Ports:      make([]uint16, len(l.Data.Ports)),
		},
		PortStats: make([]*PortStat, len(l.PortStats)),
	}
	clone.Throughput = l.Throughput
	clone.PacketTotal = l.PacketTotal
	clone.DropCount = l.DropCount
	copy(clone.Data.Ports, l.Data.Ports)
	for i, stat := range l.PortStats {
		clone.PortStats[i] = &PortStat{
			HandshakeFailure:   stat.HandshakeFailure,
			RPS:                stat.RPS,
			Success:            stat.Success,
			ClientFailure:      stat.ClientFailure,
			ServerFailure:      stat.ServerFailure,
			Disconnect:         stat.Disconnect,
			OriginResponseTime: stat.OriginResponseTime,
		}
	}
	return clone
}

type L4LBData struct {
	ServerID       uint32
	VirtualAddress [4]byte
	Address        [4]byte
	MacAddress     [6]byte
}

type L4LBWithStat struct {
	Data     L4LBData
	EbpfData []byte
}

func (l *L4LBWithStat) Clone() *L4LBWithStat {
	clone := &L4LBWithStat{
		Data: L4LBData{
			ServerID:       l.Data.ServerID,
			VirtualAddress: l.Data.VirtualAddress,
			Address:        l.Data.Address,
			MacAddress:     l.Data.MacAddress,
		},
		EbpfData: make([]byte, len(l.EbpfData)),
	}
	copy(clone.EbpfData, l.EbpfData)
	return clone
}

func (l *L4LBWithStat) Update(ebpfData *L4UpdateInfo) {
	l.EbpfData = ebpfData.EbpfData
}

type L4UpdateInfo struct {
	EbpfData []byte
}

type L4LBControlState = DataWithStat[*L4LBWithStat, *L4UpdateInfo]

func L4LBHelloToControlState(hello *L4Lbhello) *L4LBControlState {
	info := &hello.Info
	machine := &hello.Machine
	return &L4LBControlState{
		Data: &L4LBWithStat{
			Data: L4LBData{
				ServerID:       info.ServerId,
				VirtualAddress: info.VirtualAddress,
				Address:        info.Address,
				MacAddress:     info.MacAddress,
			},
			EbpfData: make([]byte, 0), // Placeholder, actual data should be filled
		},
		Stat: &MachineStat{
			CPUUsages:     make([]float64, machine.CpuCount),
			MemoryUsage:   0,
			DiskUsage:     0,
			IOUtilization: 0, // Placeholder, actual value should be calculated
			DiskSwap:      0, // Placeholder, actual value should be calculated
			LoadAvg:       0, // Placeholder, actual value should be calculated
		},
		MemoryTotal: machine.MemoryTotal,
		DiskTotal:   machine.DiskTotal,
	}
}

type L7UpdateInfo struct {
	Throughput  uint64
	PacketTotal uint64
	DropCount   uint64
	PortStats   []*PortStat
}

type L7LBControlState = DataWithStat[*L7LBWithStat, *L7UpdateInfo]

func (l *L7LBWithStat) Update(stat *L7UpdateInfo) {
	l.PacketTotal = stat.PacketTotal
	l.Throughput = stat.Throughput
	l.DropCount = stat.DropCount
	l.PortStats = stat.PortStats
}

func L7LBHelloToControlState(hello *L7Lbhello) *L7LBControlState {
	info := &hello.Info
	machine := &hello.Machine
	return &L7LBControlState{
		Data: &L7LBWithStat{
			Data: L7LBData{
				ServerID:   info.ServerId,
				Address:    info.Address,
				MacAddress: info.MacAddress,
				Ports:      info.Port,
			},
			PortStats: make([]*PortStat, info.PortLen),
		},
		Stat: &MachineStat{
			CPUUsages:     make([]float64, machine.CpuCount),
			MemoryUsage:   0,
			DiskUsage:     0,
			IOUtilization: 0, // Placeholder, actual value should be calculated
			DiskSwap:      0, // Placeholder, actual value should be calculated
			LoadAvg:       0, // Placeholder, actual value should be calculated
		},
		MemoryTotal: machine.MemoryTotal,
		DiskTotal:   machine.DiskTotal,
	}
}

func L7LBHello(lb *L7LBData, machine *MachineData) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         machineInfoLen + uint16(4+4+6+1+len(lb.Ports)*2),
			MessageType: ControlMessageType_L7LbHello,
		},
	}
	msg.SetL7LbHello(L7Lbhello{
		Machine: MachineInfo{
			CpuCount:    machine.CPUCount,
			MemoryTotal: machine.MemoryTotal,
			DiskTotal:   machine.DiskTotal,
		},
		Info: L7Lbinfo{
			Address:    lb.Address,
			MacAddress: lb.MacAddress,
			PortLen:    uint8(len(lb.Ports)),
			Port:       lb.Ports,
			ServerId:   lb.ServerID,
		},
	})
	return msg
}

func L7LBUpdate(server_id uint32) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         uint16(4),
			MessageType: ControlMessageType_L7LbUpdate,
		},
	}
	msg.SetL7LbUpdate(L7Lbupdate{
		ServerId: server_id,
	})
	return msg
}

func L4L7LBUpdate(info []*L7LBData) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         uint16(1),
			MessageType: ControlMessageType_L4LbL7LbUpdate,
		},
	}
	var l7lbInfo []L7Lbinfo
	for _, v := range info {
		l7lbInfo = append(l7lbInfo, L7Lbinfo{
			Address:    v.Address,
			MacAddress: v.MacAddress,
			PortLen:    uint8(len(v.Ports)),
			Port:       v.Ports,
			ServerId:   v.ServerID,
		})
		msg.Header.Len += uint16(4 + 4 + 6 + 1 + len(v.Ports)*2)
	}
	msg.SetL4LbL7LbUpdate(L4Lbl7Lbupdate{
		Len:  uint8(len(info)),
		Info: l7lbInfo,
	})
	return msg
}

const machineInfoLen = 1 + 8 + 8

func L4LBHello(data *L4LBData, machine *MachineData) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         machineInfoLen + uint16(4+4+4+6),
			MessageType: ControlMessageType_L4LbHello,
		},
	}
	msg.SetL4LbHello(L4Lbhello{
		Info: L4Lbinfo{
			ServerId:       data.ServerID,
			VirtualAddress: data.VirtualAddress,
			Address:        data.Address,
			MacAddress:     data.MacAddress,
		},
		Machine: MachineInfo{
			CpuCount:    machine.CPUCount,
			MemoryTotal: machine.MemoryTotal,
			DiskTotal:   machine.DiskTotal,
		},
	})
	return msg
}

func L4LBUpdate(virtual_address [4]byte) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         uint16(4 + 4),
			MessageType: ControlMessageType_L4LbUpdate,
		},
	}
	msg.SetL4LbUpdate(L4Lbupdate{
		VirtualAddress: virtual_address,
	})
	return msg
}

func KeyShare(typ KeyType, key []byte) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         uint16(1 + 2 + len(key)),
			MessageType: ControlMessageType_KeyShare,
		},
	}
	msg.SetKeyShare(KeyShareInfo{
		Type: typ,
		Len:  uint16(len(key)),
		Key:  key,
	})
	return msg
}

type MachineData struct {
	CPUCount    uint8
	MemoryTotal uint64
	DiskTotal   uint64
}

type MachineStat struct {
	Uptime        time.Duration
	CPUUsages     []float64
	MemoryUsage   uint64
	DiskUsage     uint64
	IOUtilization float64
	DiskSwap      float64
	LoadAvg       float64
}

func calcKeepAliveInfoLen(stat *MachineStat) uint16 {
	return uint16(8 + 8 + 1 + len(stat.CPUUsages)*8 + 8 + 8 + 8 + 8 + 8)
}

func L4LBKeepAlive(nextPeriod time.Duration, stat *MachineStat, ebpf_data *L4UpdateInfo) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         uint16(calcKeepAliveInfoLen(stat) + 1 + uint16(len(ebpf_data.EbpfData))),
			MessageType: ControlMessageType_L4LbKeepalive,
		},
	}
	msg.SetL4LbKeepAlive(L4LbkeepAlive{
		Info: KeepAliveInfo{
			NextPeriod:    uint64(nextPeriod),
			Uptime:        uint64(stat.Uptime),
			CpuLen:        uint8(len(stat.CPUUsages)),
			CpuUsage:      stat.CPUUsages,
			MemoryUsage:   stat.MemoryUsage,
			DiskUsage:     stat.DiskUsage,
			IoUtilization: stat.IOUtilization,
			DiskSwap:      stat.DiskSwap,
			LoadAvg:       stat.LoadAvg,
		},
		EbpfLen:  uint8(len(ebpf_data.EbpfData)),
		EbpfData: ebpf_data.EbpfData,
	})
	return msg
}

type PortStat struct {
	HandshakeFailure   uint64
	RPS                float64
	Success            uint64
	ClientFailure      uint64
	ServerFailure      uint64
	Disconnect         uint64
	OriginResponseTime uint64
}

func calcPortStatLen(portStats []*PortStat) uint16 {
	return uint16(1 + len(portStats)*56)
}

func convertPortStatsToL7LBPortInfo(portStats []*PortStat) []L7LbportInfo {
	l7PortInfos := make([]L7LbportInfo, len(portStats))
	for i, stat := range portStats {
		l7PortInfos[i] = L7LbportInfo{
			HandshakeFailure:   stat.HandshakeFailure,
			Rps:                stat.RPS,
			Success:            stat.Success,
			ClientFailure:      stat.ClientFailure,
			ServerFailure:      stat.ServerFailure,
			Disconnect:         stat.Disconnect,
			OriginResponseTime: stat.OriginResponseTime,
		}
	}
	return l7PortInfos
}

func ConvertL7PortInfoToPortStat(l7PortInfos []L7LbportInfo) []*PortStat {
	portStats := make([]*PortStat, len(l7PortInfos))
	for i, info := range l7PortInfos {
		portStats[i] = &PortStat{
			HandshakeFailure:   info.HandshakeFailure,
			RPS:                info.Rps,
			Success:            info.Success,
			ClientFailure:      info.ClientFailure,
			ServerFailure:      info.ServerFailure,
			Disconnect:         info.Disconnect,
			OriginResponseTime: info.OriginResponseTime,
		}
	}
	return portStats
}

func L7LBKeepAlive(nextPeriod time.Duration, stat *MachineStat, d *L7UpdateInfo) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         uint16(calcKeepAliveInfoLen(stat) + 8*3 + calcPortStatLen(d.PortStats)),
			MessageType: ControlMessageType_L7LbKeepalive,
		},
	}
	msg.SetL7LbKeepAlive(L7LbkeepAlive{
		Info: KeepAliveInfo{
			NextPeriod:    uint64(nextPeriod),
			Uptime:        uint64(stat.Uptime),
			CpuLen:        uint8(len(stat.CPUUsages)),
			CpuUsage:      stat.CPUUsages,
			MemoryUsage:   stat.MemoryUsage,
			DiskUsage:     stat.DiskUsage,
			IoUtilization: stat.IOUtilization,
			DiskSwap:      stat.DiskSwap,
			LoadAvg:       stat.LoadAvg,
		},
		PacketTotal: d.PacketTotal,
		Throughput:  d.Throughput,
		DropCount:   d.DropCount,
		PortLen:     uint8(len(d.PortStats)),
		Ports:       convertPortStatsToL7LBPortInfo(d.PortStats),
	})
	return msg
}

func WasmInstall(id uint32, method string, path string, chunkInfo ChunkInfo) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         uint16(4 + 1 + len(method) + 2 + len(path) + 4),
			MessageType: ControlMessageType_WasmInstall,
		},
	}
	msg.SetWasmInstall(WasmInstallInfo{
		Id:        id,
		MethodLen: uint8(len(method)),
		Method:    []byte(method),
		PathLen:   uint16(len(path)),
		Path:      []byte(path),
		ChunkInfo: chunkInfo,
	})
	return msg
}

func WasmUninstall(id uint32) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         4,
			MessageType: ControlMessageType_WasmUninstall,
		},
	}
	msg.SetWasmUninstall(WasmUninstallInfo{
		Id: id,
	})
	return msg
}

func UpdateL4WithKeepAlive(kl *L4LbkeepAlive, d *L4LBControlState) {
	d.Update(&MachineStat{
		Uptime:        time.Duration(kl.Info.Uptime),
		CPUUsages:     kl.Info.CpuUsage,
		MemoryUsage:   kl.Info.MemoryUsage,
		DiskUsage:     kl.Info.DiskUsage,
		IOUtilization: kl.Info.IoUtilization,
		DiskSwap:      kl.Info.DiskSwap,
		LoadAvg:       kl.Info.LoadAvg,
	}, &L4UpdateInfo{
		EbpfData: kl.EbpfData,
	})
}

func UpdateL7WithKeepAlive(kl *L7LbkeepAlive, d *L7LBControlState) {
	d.Update(&MachineStat{
		Uptime:        time.Duration(kl.Info.Uptime),
		CPUUsages:     kl.Info.CpuUsage,
		MemoryUsage:   kl.Info.MemoryUsage,
		DiskUsage:     kl.Info.DiskUsage,
		IOUtilization: kl.Info.IoUtilization,
		DiskSwap:      kl.Info.DiskSwap,
		LoadAvg:       kl.Info.LoadAvg,
	}, &L7UpdateInfo{
		PacketTotal: kl.PacketTotal,
		Throughput:  kl.Throughput,
		DropCount:   kl.DropCount,
		PortStats:   ConvertL7PortInfoToPortStat(kl.Ports),
	})
}

func LogMessage(level LogLevel, msg string) *ControlMessage {
	msgBytes := []byte(msg)
	if len(msgBytes) > 65535 {
		msgBytes = msgBytes[:65535] // Limit to 65535 bytes
	}
	msgLen := uint16(len(msgBytes))

	controlMsg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         msgLen + 2 + 1, // 2 bytes for msg_len
			MessageType: ControlMessageType_Message,
		},
	}
	controlMsg.SetMessage(Message{
		Level:  level,
		MsgLen: msgLen,
		Msg:    msgBytes,
	})
	return controlMsg
}

func CommandLineIn(cmdlineID uint32, input string) *ControlMessage {
	cmdlineBytes := []byte(input)
	if len(cmdlineBytes) > 65535 {
		cmdlineBytes = cmdlineBytes[:65535] // Limit to 65535 bytes
	}
	cmdlineLen := uint16(len(cmdlineBytes))

	controlMsg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         cmdlineLen + 6, // 4 bytes for cmdlineID + 2 bytes for cmdline_len
			MessageType: ControlMessageType_CmdlineIn,
		},
	}
	controlMsg.SetCmdlineIn(CmdlineIn{
		CmdlineId: cmdlineID,
		Cmdline:   cmdlineBytes,
		Len:       cmdlineLen,
	})
	return controlMsg
}

func CommandLineOutput(cmdlineID uint32, outputType OutputType, chunkInfo ChunkInfo) *ControlMessage {
	controlMsg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         4 + 1 + 4,
			MessageType: ControlMessageType_CmdlineOut,
		},
	}
	controlMsg.SetCmdlineOut(CmdlineOut{
		CmdlineId:  cmdlineID,
		OutputType: outputType,
		ChunkInfo:  chunkInfo,
	})
	return controlMsg
}

func CommandLineExit(cmdlineID uint32, exitCode uint32) *ControlMessage {
	controlMsg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         4 + 4, // 4 bytes for cmdlineID + 4 bytes for exit_code
			MessageType: ControlMessageType_CmdlineExit,
		},
	}
	controlMsg.SetCmdlineExit(CommandExit{
		CmdlineId: cmdlineID,
		ExitCode:  exitCode,
	})
	return controlMsg
}

func TransferFile(path string, permission uint16, chunkInfo ChunkInfo) *ControlMessage {
	pathBytes := []byte(path)
	if len(pathBytes) > 65535 {
		pathBytes = pathBytes[:65535] // Limit to 65535 bytes
	}
	pathLen := uint16(len(pathBytes))

	controlMsg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         2 + pathLen + 2 + 4, // 2 bytes for path_len + path_len + 2 bytes for permission + 8 bytes for file_len
			MessageType: ControlMessageType_FileTransfer,
		},
	}
	controlMsg.SetFileTransfer(FileTransfer{
		PathLen:    pathLen,
		Path:       pathBytes,
		Permission: permission,
		ChunkInfo:  chunkInfo,
	})
	return controlMsg
}

func CommandLineInstruction(cmdlineID uint32, instruction CmdInstructionType, arg byte) *ControlMessage {
	controlMsg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         1 + 4 + 1, // 4 bytes for cmdlineID + 1 byte for instruction + 1 byte for arg
			MessageType: ControlMessageType_CmdlineInstr,
		},
	}
	controlMsg.SetCmdlineInstr(CmdlineInstruction{
		CmdlineId: cmdlineID,
		Instr:     instruction,
		Arg:       arg,
	})
	return controlMsg
}

func CommandLineResize(cmdlineID uint32, col, row uint16) *ControlMessage {
	controlMsg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         4 + 2 + 2, // 4 bytes for cmdlineID + 2 bytes for x + 2 bytes for y
			MessageType: ControlMessageType_CmdlineResize,
		},
	}
	controlMsg.SetCmdlineResize(CmdlineResize{
		CmdlineId: cmdlineID,
		Col:       col,
		Row:       row,
	})
	return controlMsg
}

func LargeChunk(chunkID uint32, chunkLen uint32, eof bool) *ControlMessage {
	controlMsg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         4 + 4, // 1 bit for eof + 31 bits for chunkID + 4 bytes for chunkLen
			MessageType: ControlMessageType_LargeChunk,
		},
	}
	hdr := LargeChunkHeader{
		ChunkLen: chunkLen,
	}
	hdr.SetChunkId(chunkID)
	hdr.SetEof(eof)
	controlMsg.SetLargeChunk(hdr)
	return controlMsg
}
