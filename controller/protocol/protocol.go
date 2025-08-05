package protocol

import "time"

type L7LBData struct {
	ServerID   uint32
	Address    [4]byte
	MacAddress [6]byte
	Ports      []uint16
}

func L4LBInfoToData(info *L4Lbinfo) *L4LBData {
	return &L4LBData{
		ServerID:       info.ServerId,
		VirtualAddress: info.VirtualAddress,
		Address:        info.Address,
		MacAddress:     info.MacAddress,
	}
}

func L7LBInfoToData(info *L7Lbinfo) *L7LBData {
	return &L7LBData{
		ServerID:   info.ServerId,
		Address:    info.Address,
		MacAddress: info.MacAddress,
		Ports:      info.Port,
	}
}

func L7LBHello(lb *L7LBData) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         uint16(4 + 4 + 6 + 1 + len(lb.Ports)*2),
			MessageType: ControlMessageType_L7LbHello,
		},
	}
	msg.SetL7LbHello(L7Lbinfo{
		Address:    lb.Address,
		MacAddress: lb.MacAddress,
		PortLen:    uint8(len(lb.Ports)),
		Port:       lb.Ports,
		ServerId:   lb.ServerID,
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
			MessageType: ControlMessageType_L7LbUpdate,
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

type L4LBData struct {
	ServerID       uint32
	VirtualAddress [4]byte
	Address        [4]byte
	MacAddress     [6]byte
}

func L4LBHello(data *L4LBData) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         uint16(4 + 4 + 4 + 6),
			MessageType: ControlMessageType_L4LbHello,
		},
	}
	msg.SetL4LbHello(L4Lbinfo{
		ServerId:       data.ServerID,
		VirtualAddress: data.VirtualAddress,
		Address:        data.Address,
		MacAddress:     data.MacAddress,
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

func KeepAlive(nextPeriod time.Duration) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         8,
			MessageType: ControlMessageType_Keepalive,
		},
	}
	msg.SetKeepAlive(KeepAliveInfo{
		NextPeriod: uint64(nextPeriod),
	})
	return msg
}

func WasmInstall(id uint32, method string, path string, code_len uint64) *ControlMessage {
	msg := &ControlMessage{
		Header: ControlMessageHeader{
			Version:     0,
			Len:         uint16(4 + 4 + len(method) + 4 + len(path) + 8),
			MessageType: ControlMessageType_WasmInstall,
		},
	}
	msg.SetWasmInstall(WasmInstallInfo{
		Id:         id,
		MethodLen:  uint8(len(method)),
		Method:     []byte(method),
		PathLen:    uint16(len(path)),
		Path:       []byte(path),
		BinarySize: uint64(code_len),
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
