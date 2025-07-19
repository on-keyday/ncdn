package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"github.com/quic-go/quic-go"
)

type QUICLBConnIDGenerator struct {
	server_id uint8  // <= 0xf
	sharedKey []byte // shared key for MAC calculation
}

func NewQUICLBConnIDGenerator(server_id uint8, sharedKey []byte) *QUICLBConnIDGenerator {
	if server_id > 0xf {
		panic("server_id must be less than or equal to 0xf")
	}
	return &QUICLBConnIDGenerator{
		server_id: server_id,
		sharedKey: sharedKey,
	}
}

// format FirstByteOfQUICLB:
//  config_rotation :u3
//  random :u5
// format QUICLBIDForThisImplementation: # とりあえずこの構造で仮決定 (20バイト)
//  first_octet :FirstByteOfQUICLB # cid_len_or_randomはランダムビット
//  server_id :u4 # server id (今回は通常は2台だし増やすとしても16台も使うことはないのでこれでよし(増えるようならclient_id側で調整...))
//  client_id_high :u4
//  client_id_low :[2]u8 # also nonce for each connection id (2^20=1048576~=約100万)
//  mac :[16]u8 # mac calucluated with connection id exclude this field and server/lb shared secret
//              # ただしパケット長とのトレードオフ
//              # あとrandomビット数が5 + 4 + 16 = 25bitでQUIC-LB draftのnonce要件32bitに7bit分足りてない気がする
//              # あと1バイトくらいconnection idを増やすかどうか...
//              # しかしquic-go実装もといRFC9000が20バイト上限にしてやがる...(忘れてた...)

func (g *QUICLBConnIDGenerator) GenerateConnectionID() (quic.ConnectionID, error) {
	var cid [20]byte
	if _, err := rand.Read(cid[:4]); err != nil {
		return quic.ConnectionID{}, fmt.Errorf("generating conn ID failed: %w", err)
	}
	cid[0] &= 0x1f // 5 bits for random. 3 bit rotation id is currently 0
	cid[1] &= 0x0f // 4 bits for server id
	cid[1] |= g.server_id << 4
	hm := hmac.New(sha256.New, g.sharedKey)
	_, err := hm.Write(cid[:4])
	if err != nil {
		return quic.ConnectionID{}, fmt.Errorf("generating conn ID failed: %w", err)
	}
	sum := hm.Sum(nil)
	copy(cid[4:], sum[:16]) // 16 bytes for MAC
	return quic.ConnectionIDFromBytes(cid[:]), nil
}

func (g *QUICLBConnIDGenerator) ConnectionIDLen() int {
	return 20 // currently 20
}
