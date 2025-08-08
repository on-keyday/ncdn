package control

import (
	"errors"

	"github.com/yzp0n/ncdn/controller/protocol"
)

func (c *controller) ShareKey(key []byte) {
	var l4lblist *l4list
	var l7lbList *l7list
	var keyGeneration uint64
	c.withLock(func() {
		c.sharedKey.generation = c.msgSeqNum
		c.sharedKey.key = key
		keyGeneration = c.sharedKey.generation
		c.msgSeqNum++
		l4lblist = c.l4lblist.Clone()
		l7lbList = c.l7lbList.Clone()
	})
	for _, l4lb := range l4lblist.list {
		l4lb.SendMessage(message{
			seqNum: keyGeneration,
			data:   protocol.KeyShare(protocol.KeyType_Quiclb, key),
		})
	}
	for _, l7lb := range l7lbList.list {
		l7lb.SendMessage(message{
			seqNum: keyGeneration,
			data:   protocol.KeyShare(protocol.KeyType_Quiclb, key),
		})
	}
	c.logger.Info("Shared key with L4LB and L7LBs", "generation", keyGeneration)
}

func (c *controller) FileTransfer(typ LBType, serverID uint32, path string, permission uint16, file ReaderAtCloser) error {
	if len(path) > 65535-12 { // 12 bytes for permission and file size
		return errors.New("file path exceeds maximum length of 65535 bytes")
	}
	msg := protocol.TransferFile(path, permission, uint64(file.Size()))
	var seqNum uint64
	switch typ {
	case LBTypeL7:
		var l7lbConn *L7LB
		c.withLock(func() {
			for _, l7lb := range c.l7lbList.list {
				if l7lb.data.data.Data.Data.ServerID == serverID {
					seqNum = c.msgSeqNum
					c.msgSeqNum++
					l7lbConn = l7lb
					break
				}
			}
		})
		if l7lbConn != nil {
			file.AddRef() // Ensure the file remains open until the message is sent
			l7lbConn.SendMessage(message{
				seqNum: seqNum,
				data:   msg,
				reader: file,
			})
		}
	case LBTypeL4:
		var l4lbConn *L4LB
		c.withLock(func() {
			for _, l4lb := range c.l4lblist.list {
				if l4lb.data.data.Data.Data.ServerID == serverID {
					seqNum = c.msgSeqNum
					c.msgSeqNum++
					l4lbConn = l4lb
					break
				}
			}
		})
		if l4lbConn != nil {
			file.AddRef() // Ensure the file remains open until the message is sent
			l4lbConn.SendMessage(message{
				seqNum: seqNum,
				data:   msg,
				reader: file,
			})
		}
	}
	return nil
}

func (c *controller) WasmInstall(id uint32, method, path string, file ReaderAtCloser) error {
	msg := protocol.WasmInstall(id, method, path, uint64(file.Size()))
	var seqNum uint64
	var l7lblist *l7list
	c.withLock(func() {
		seqNum = c.msgSeqNum
		c.msgSeqNum++
		l7lblist = c.l7lbList.Clone()
	})
	for _, l7lb := range l7lblist.list {
		file.AddRef() // Ensure the file remains open until the message is sent
		l7lb.SendMessage(message{
			seqNum: seqNum,
			data:   msg,
			reader: file,
		})
	}
	return nil
}

func (c *controller) WasmUninstall(id uint32) error {
	msg := protocol.WasmUninstall(id)
	var seqNum uint64
	var l7lblist *l7list
	c.withLock(func() {
		seqNum = c.msgSeqNum
		c.msgSeqNum++
		l7lblist = c.l7lbList.Clone()
	})
	for _, l7lb := range l7lblist.list {
		l7lb.SendMessage(message{
			seqNum: seqNum,
			data:   msg,
		})
	}
	return nil
}
