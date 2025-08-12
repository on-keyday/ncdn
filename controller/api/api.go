package api

import (
	"net/netip"
	"sync"
	"time"

	"github.com/yzp0n/ncdn/controller/control"
)

type UploadedFile struct {
	ID      uint64
	File    []byte
	Expires time.Time
}

type UploaderManager struct {
	m        sync.Mutex
	payloads map[uint64]*UploadedFile
	counter  uint64
}

func NewUploaderManager() *UploaderManager {
	mgr := &UploaderManager{
		payloads: make(map[uint64]*UploadedFile),
		counter:  0,
	}
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			mgr.RemoveExpired()
		}
	}()
	return mgr
}

func (um *UploaderManager) AddPayload(payload []byte) uint64 {
	um.m.Lock()
	defer um.m.Unlock()
	if um.payloads == nil {
		um.payloads = make(map[uint64]*UploadedFile)
	}
	um.payloads[um.counter] = &UploadedFile{
		ID:      um.counter,
		File:    payload,
		Expires: time.Now().Add(3 * time.Minute),
	}
	um.counter++
	return um.counter - 1
}

func (um *UploaderManager) GetPayload(id uint64) ([]byte, bool) {
	um.m.Lock()
	defer um.m.Unlock()
	file, exists := um.payloads[id]
	if !exists || time.Now().After(file.Expires) {
		return nil, false
	}
	delete(um.payloads, id) // Remove after retrieval
	return file.File, true
}

func (um *UploaderManager) RemoveExpired() {
	um.m.Lock()
	defer um.m.Unlock()
	for id, file := range um.payloads {
		if time.Now().After(file.Expires) {
			delete(um.payloads, id)
		}
	}
}

type FileUploadBody struct {
	FileID uint64           `json:"file_id"`
	Path   string           `json:"path"`
	Mode   uint16           `json:"mode"`
	Dest   control.DestInfo `json:"dest"`
}

type WasmInstallBody struct {
	FileID uint64           `json:"file_id"`
	WasmID uint32           `json:"wasm_id"`
	Method string           `json:"method"`
	Path   string           `json:"path"`
	Dest   control.DestInfo `json:"dest"`
}

type WasmUninstallBody struct {
	WasmID uint32           `json:"wasm_id"`
	Dest   control.DestInfo `json:"dest"`
}

type UploadResponse struct {
	ID uint64 `json:"id"`
}

type VIPUpdate struct {
	VIP  netip.Addr       `json:"vip"`
	Dest control.DestInfo `json:"dests"`
}
