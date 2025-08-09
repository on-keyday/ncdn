package chunk

import (
	"fmt"

	"github.com/yzp0n/ncdn/controller/protocol"
)

type Chunk struct {
	Msg  *protocol.ControlMessage
	Data []byte
}

type ChunkMap struct {
	chunks map[uint32]*Chunk
}

func NewChunkMap() *ChunkMap {
	return &ChunkMap{
		chunks: make(map[uint32]*Chunk),
	}
}

func (cm *ChunkMap) registerChunk(msg *protocol.ControlMessage, id uint32) error {
	if _, exists := cm.chunks[id]; exists {
		return fmt.Errorf("chunk with id %d already exists", id)
	}
	cm.chunks[id] = &Chunk{
		Msg:  msg,
		Data: make([]byte, 0),
	}
	return nil
}

func (cm *ChunkMap) addData(id uint32, data []byte, ended bool) (*Chunk, error) {
	ch, exists := cm.chunks[id]
	if !exists {
		return nil, fmt.Errorf("chunk with id %d does not exist", id)
	}
	ch.Data = append(ch.Data, data...)
	if ended {
		delete(cm.chunks, id) // Remove the chunk after it is ended
		return ch, nil
	}
	return nil, nil
}

func (c *ChunkMap) Clear() {
	clear(c.chunks)
}

type Receiver interface {
	ReceiveSize(size int) ([]byte, error)
}

// return chunked or chunk
func (c *ChunkMap) ReadChunked(conn Receiver, msg *protocol.ControlMessage) (*Chunk, error) {
	readFromChunkInfo := func(info protocol.ChunkInfo) (*Chunk, error) {
		if info.IsChunkd() {
			err := c.registerChunk(msg, info.LenOrId())
			if err != nil {
				return nil, fmt.Errorf("failed to add data for chunk id %d: %w", info.LenOrId(), err)
			}
			return nil, nil // Chunk registered, no data to return yet
		}
		data, err := conn.ReceiveSize(int(info.LenOrId()))
		if err != nil {
			return nil, fmt.Errorf("failed to receive data length %d: %w", info.LenOrId(), err)
		}
		return &Chunk{
			Msg:  msg,
			Data: data,
		}, nil
	}
	switch msg.Header.MessageType {
	case protocol.ControlMessageType_FileTransfer:
		info := msg.FileTransfer()
		return readFromChunkInfo(info.ChunkInfo)
	case protocol.ControlMessageType_WasmInstall:
		info := msg.WasmInstall()
		return readFromChunkInfo(info.ChunkInfo)
	case protocol.ControlMessageType_CmdlineOut:
		info := msg.CmdlineOut()
		return readFromChunkInfo(info.ChunkInfo)
	case protocol.ControlMessageType_LargeChunk:
		info := msg.LargeChunk()
		data, err := conn.ReceiveSize(int(info.ChunkLen))
		if err != nil {
			return nil, fmt.Errorf("failed to receive large chunk data with length %d: %w", info.ChunkLen, err)
		}
		return c.addData(info.ChunkId(), data, info.Eof())
	default:
		return &Chunk{
			Msg:  msg,
			Data: nil,
		}, nil
	}
}
