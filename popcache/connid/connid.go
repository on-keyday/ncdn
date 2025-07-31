package lbconnid

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"os"

	"github.com/quic-go/quic-go"
)

type QUICLBConnIDGenerator struct {
	serverID  uint32
	sharedKey cipher.Block
	connIDLen uint8
	rotation  uint8 // Rotation counter for the first byte
}

const QUICLB_DEBUG_ENV = "QUICLB_DEBUG"

func printDebugInfo(format string, args ...interface{}) {
	if v, ok := os.LookupEnv(QUICLB_DEBUG_ENV); ok && v == "1" {
		fmt.Printf(format, args...)
	}
}

func NewQUICLBConnIDGenerator(serverID uint32, sharedKey []byte, connIDLen uint8) *QUICLBConnIDGenerator {
	if connIDLen < 5+4 || connIDLen > 20 {
		panic(fmt.Errorf("connIDLen must be between 9 and 20, got %d", connIDLen))
	}
	printDebugInfo("DEBUG: Derived AES key: %x\n", sharedKey)
	block, err := aes.NewCipher(sharedKey)
	if err != nil {
		panic(fmt.Errorf("failed to create AES cipher: %w", err))
	}
	return &QUICLBConnIDGenerator{
		serverID:  serverID,
		sharedKey: block,
		connIDLen: connIDLen,
		rotation:  0, // Initial rotation value
	}
}

/*
**
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

func generateConnectionID(key cipher.Block, plainCID []byte) ([]byte, error) {
	if len(plainCID) < 6 || len(plainCID) > 20 {
		return nil, fmt.Errorf("invalid connection ID length: %d", len(plainCID))
	}
	printDebugInfo("DEBUG: Plain CID: %x\n", plainCID)
	var fullLen uint8 = uint8(len(plainCID) - 1) // Exclude the first octet
	if fullLen == 16 {
		var buffer [17]byte
		buffer[0] = plainCID[0]               // First byte remains unchanged
		key.Encrypt(buffer[1:], plainCID[1:]) // Encrypt the rest
		return buffer[:], nil                 // Return the full connection ID including the first octet
	}
	var isOdd bool = fullLen&1 == 1
	var halfLen uint8 = (fullLen + 1) / 2 // Round up to the nearest whole number
	var rightStart uint8 = fullLen - halfLen
	var encryptTarget = plainCID[1:]
	var left [10]byte
	var right [10]byte
	copy(left[:], encryptTarget[:halfLen])     // left part
	copy(right[:], encryptTarget[rightStart:]) // right part
	if isOdd {
		left[halfLen-1] &= 0xf0 // clear the last 4 bits of left part
		right[0] &= 0x0f        // clear the first 4 bits of right part
	}
	round := func(input []byte, output []byte, pass uint8, name_in, name_out string) {
		temporary := expand(fullLen, pass, input[:])
		printDebugInfo("DEBUG: pass_%d_key = %032x\n", pass, temporary)
		key.Encrypt(temporary[:], temporary[:]) // Encrypt left part
		subtle.XORBytes(output[:], output[:], truncate(temporary, 10))
		if pass%2 == 0 {
			output[len(output)-1] &= 0xf0
		} else {
			output[0] &= 0x0f
		}
		printDebugInfo("DEBUG: round %d: %s=%020x %s=%020x\n", pass, name_in, input, name_out, output)
	}
	printDebugInfo("DEBUG: Initial left: %020x, right: %020x\n", left, right)
	round(left[:halfLen], right[:halfLen], 1, "left_0", "right_1")
	round(right[:halfLen], left[:halfLen], 2, "right_1", "left_1")
	round(left[:halfLen], right[:halfLen], 3, "left_1", "right_2")
	round(right[:halfLen], left[:halfLen], 4, "right_2", "left_2")
	var cipherCID [20]byte
	cipherCID[0] = plainCID[0]                      // First byte remains unchanged
	copy(cipherCID[1:], left[:halfLen])             // Copy the left part
	copy(cipherCID[1+rightStart:], right[:halfLen]) // Copy the right part
	if isOdd {
		cipherCID[halfLen] = (left[halfLen-1] & 0xf0) | (right[0] & 0x0f)
	}
	printDebugInfo("DEBUG: Final left: %020x, right: %020x\n", left, right)
	printDebugInfo("DEBUG: Final CID: %040x\n", cipherCID)
	return cipherCID[:fullLen+1], nil // Return the full connection ID including the first octet
}

func (g *QUICLBConnIDGenerator) GenerateConnectionID() (quic.ConnectionID, error) {
	var cid [20]byte
	if _, err := rand.Read(cid[:g.connIDLen]); err != nil {
		return quic.ConnectionID{}, fmt.Errorf("generating conn ID failed: %w", err)
	}
	v := &LBConnectionIDHead{}
	v.Decode(cid[:])
	v.FirstByte.SetConfigRotation(g.rotation)
	v.ServerId = g.serverID
	plainCIDHead := v.MustEncode()
	copy(cid[:], plainCIDHead)
	cipherCID, err := generateConnectionID(g.sharedKey, cid[:g.connIDLen])
	if err != nil {
		return quic.ConnectionID{}, fmt.Errorf("generating conn ID failed: %w", err)
	}
	return quic.ConnectionIDFromBytes(cipherCID[:]), nil
}

func (g *QUICLBConnIDGenerator) ConnectionIDLen() int {
	return int(g.connIDLen)
}

func (g *QUICLBConnIDGenerator) ServerID() uint32 {
	return g.serverID
}

func (g *QUICLBConnIDGenerator) Rotation() uint8 {
	return g.rotation
}

// Caller must gurantee the rotated shared key is shared with the LB.
// and the new shared key must be the same length as the original one.
func (g *QUICLBConnIDGenerator) RotateKey(newSharedKey []byte) error {
	if len(newSharedKey) != g.sharedKey.BlockSize() {
		return fmt.Errorf("new shared key must be %d bytes long", g.sharedKey.BlockSize())
	}
	block, err := aes.NewCipher(newSharedKey)
	if err != nil {
		return fmt.Errorf("failed to create new AES cipher: %w", err)
	}
	g.sharedKey = block
	/*
		   spec says
			```
			2.2.  Configuration Failover

			A server that is configured to use QUIC-LB might be forced to accept
			new connections without having received a current configuration.  A
			server without QUIC-LB configuration can accept connections, but it
			SHOULD generate initial connection IDs with the config rotation bits
			set to 0b111 and avoid sending the client connection IDs in
			NEW_CONNECTION_ID frames or the preferred_address transport
			parameter.  Servers in this state SHOULD use the
			"disable_active_migration" transport parameter until a valid
			configuration is received.

			A load balancer that sees a connection ID with config rotation bits
			set to 0b111 MUST route using an algorithm based solely on the
			address/port 4-tuple, which is consistent well beyond the QUIC
			handshake.  However, a load balancer MAY observe the connection IDs
			used during the handshake and populate a connection ID table that
			allows the connection to survive a NAT rebinding, and reduces the
			probability of connection failure due to a change in the number of
			servers.
			```
			so rotation bit should be less than 7= 0b111
	*/
	g.rotation = (g.rotation + 1) % 7
	fmt.Printf("DEBUG: New AES key: %x\n", newSharedKey)
	return nil
}
