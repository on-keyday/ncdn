package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"fmt"

	"github.com/quic-go/quic-go"
	"github.com/yzp0n/ncdn/tool/util"
)

type QUICLBConnIDGenerator struct {
	server_id uint8 // <= 0xf
	sharedKey cipher.Block
}

func NewQUICLBConnIDGenerator(server_id uint8, sharedKey []byte) *QUICLBConnIDGenerator {
	if server_id > 0xf {
		panic("server_id must be less than or equal to 0xf")
	}
	aesKeyBytes, err := util.DeriveKey(sharedKey, "quic-lb")
	if err != nil {
		panic(fmt.Errorf("failed to derive AES key: %w", err))
	}
	block, err := aes.NewCipher(aesKeyBytes[:])
	if err != nil {
		panic(fmt.Errorf("failed to create AES cipher: %w", err))
	}
	return &QUICLBConnIDGenerator{
		server_id: server_id,
		sharedKey: block,
	}
}

//
//
/*
format FirstByteOfQUICLB:
  config_rotation :u3
  random :u5
format QUICLBIDForThisImplementationv2: # quic-lb準拠に近く
  first_octet :FirstByteOfQUICLB # cid_len_or_randomはランダムビット
  server_id :u4 # server id (今回は通常は2台だし増やすとしても16台も使うことはないのでこれでよし)
  opaque_1 :u4
  opaque :[18]u8 # とりあえず現時点の互換性用(消すかも)
*/

/*
# 'input_bytes' is drawn from one half of the plaintext.  It forms
# the N most significant octets of the output, where N is half the
# 'length' argument, rounded up, and thus a number between 3 and 10,
#  inclusive.
# 'Zeropad' is a set of 14-N octets set to zero.
# 'length' is an 8-bit integer that reports the sum of the
# configured nonce length and server id length in octets, and forms
# the fifteenth octet of the output.  The 'length' argument MUST NOT
# exceed 19 and MUST NOT be less than 5.
# 'pass' is an 8-bit integer that reports the 'pass' argument of the
# algorithm, and forms the sixteenth (least significant) octet of
# the output.  It guarantees that the cryptographic input of every
# pass of the algorithm is unique.
format ExpandResult:

	input_bytes :[3..=10]u8
	zero_padding :[14 - input_bytes.length]u8
	full_connection_id_len :u8 # exclude the first octet
	full_connection_id_len >= 5 && full_connection_id_len <= 19
	pass :u8 # for cryptographic input uniqueness
*/
func expand(conn_id_len uint8, pass uint8, input []byte) [16]byte {
	if len(input) < 3 || len(input) > 10 {
		panic("input must be between 3 and 10 bytes")
	}
	if conn_id_len < 5 || conn_id_len > 19 {
		panic("conn_id_len must be between 5 and 19")
	}
	var output [16]byte
	copy(output[:], input)
	output[14] = conn_id_len
	output[15] = pass
	return output
}

func truncate(input [16]byte, length uint8) []byte {
	if length > 16 {
		panic("length must not exceed 16")
	}
	return input[:length]
}

func (g *QUICLBConnIDGenerator) GenerateConnectionID() (quic.ConnectionID, error) {
	var cid [20]byte
	if _, err := rand.Read(cid[:]); err != nil {
		return quic.ConnectionID{}, fmt.Errorf("generating conn ID failed: %w", err)
	}
	cid[0] &= 0x1f // 5 bits for random. 3 bit rotation id is currently 0
	cid[1] &= 0x0f // 4 bits for server id
	cid[1] |= g.server_id << 4
	// Encrypt the connection ID using AES
	const connectionIDLen = 19 // exclude the first octet
	var left [10]byte
	var right [10]byte
	copy(left[:], cid[1:11])   // left part
	left[9] &= 0xf0            // clear the last 4 bits of left part
	copy(right[:], cid[10:20]) // right part
	right[0] &= 0x0f           // clear the first 4 bits of right part
	round := func(output []byte, input []byte, pass uint8) {
		temporary := expand(connectionIDLen, pass, input[:])
		g.sharedKey.Encrypt(temporary[:], temporary[:]) // Encrypt left part
		subtle.XORBytes(output[:], output[:], truncate(temporary, 10))
		if len(output)%2 != 0 {
			if pass%2 == 0 {
				output[len(output)-1] &= 0xf0
			} else {
				output[0] &= 0x0f
			}
		}
	}
	round(left[:], right[:], 1)
	round(right[:], left[:], 2)
	round(left[:], right[:], 3)
	round(right[:], left[:], 4)
	copy(cid[1:10], left[:9])              // 0-9
	copy(cid[11:20], right[:9])            // 11-19
	cid[10] = left[9]&0xf0 | right[0]&0x0f // 10

	return quic.ConnectionIDFromBytes(cid[:]), nil
}

func (g *QUICLBConnIDGenerator) ConnectionIDLen() int {
	return 20 // currently 20
}
