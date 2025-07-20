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

//	 format FirstByteOfQUICLB:
//	    config_rotation :u3
//		   random :u5
//
//	 format QUICLBIDForThisImplementation: # とりあえずこの構造で仮決定 (20バイト)
//		   first_octet :FirstByteOfQUICLB # cid_len_or_randomはランダムビット
//		   server_id :u4 # server id (今回は通常は2台だし増やすとしても16台も使うことはないのでこれでよし(増えるようならclient_id側で調整...))
//		   client_id_high :u4
//		   client_id_low :[2]u8 # also nonce for each connection id (2^20=1048576~=約100万)
//		   nonce_pad :u7 # nonce要件を満たすためのランダム値
//		   mac_high :u1
//		   mac_low :[15]u8 # mac calucluated with connection id exclude this field and server/lb shared secret
//		            # ただしパケット長とのトレードオフ
//		            # あとrandomビット数が5bit + 20(=4+2*8)bit + 7bit = 32bitでQUIC-LB draftのnonce要件32bitを満たす
//		            # quic-goもといRFC9000により長さは20バイトが限界であると判明
//		            # mac計算時にはQUICLBIDFortThisImplementationMACTargetとshared secretを使って128bit MACを計算し
//		            # そのうちの最上位バイトの下位1ビット及び残りの15バイトをMAC(121bit)とする
func (g *QUICLBConnIDGenerator) GenerateConnectionID() (quic.ConnectionID, error) {
	var cid [20]byte
	if _, err := rand.Read(cid[:5]); err != nil {
		return quic.ConnectionID{}, fmt.Errorf("generating conn ID failed: %w", err)
	}
	cid[0] &= 0x1f // 5 bits for random. 3 bit rotation id is currently 0
	cid[1] &= 0x0f // 4 bits for server id
	cid[1] |= g.server_id << 4
	cid[4] &= 0xfe // for mac calculation, set the least significant bit to 0
	hm := hmac.New(sha256.New, g.sharedKey)
	_, err := hm.Write(cid[:5]) // first 5 bytes are used for MAC calculation
	if err != nil {
		return quic.ConnectionID{}, fmt.Errorf("generating conn ID failed: %w", err)
	}
	sum := hm.Sum(nil)
	cid[4] |= sum[0] & 0x01  // set the least significant bit of the 5th byte
	copy(cid[5:], sum[1:16]) // remaining 15 bytes are used for MAC
	return quic.ConnectionIDFromBytes(cid[:]), nil
}

func (g *QUICLBConnIDGenerator) ConnectionIDLen() int {
	return 20 // currently 20
}
