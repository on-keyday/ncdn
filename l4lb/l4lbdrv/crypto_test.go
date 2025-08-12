package l4lbdrv_test

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/yzp0n/ncdn/l4lb/l4lbdrv"
)

func TestCrypto(t *testing.T) {
	mustDecodeHex := func(s string) []byte {
		b, err := hex.DecodeString(s)
		if err != nil {
			t.Fatalf("Failed to decode hex string %q: %v", s, err)
		}
		return b
	}
	key := mustDecodeHex("8f95f09245765f80256934e50c66207f")
	err := l4lbdrv.InitCrypto("/mnt/ncdn/l4lb/c/init_crypto.o", "/sys/fs/bpf/__crypto_ctx_map", "/sys/fs/bpf/", key)
	if err != nil {
		t.Fatalf("Failed to initialize crypto: %v", err)
	}
	t.Log("Crypto initialized successfully")
	bound, err := l4lbdrv.BindBalancer("/mnt/ncdn/l4lb/c/lb.o", "", "/sys/fs/bpf/")
	if err != nil {
		t.Fatalf("Failed to bind balancer: %v", err)
	}

	fatal := func(fmt string, a ...any) {
		t.Helper()
		bound.Close()
		t.Fatalf(fmt, a...)
	}
	runTest := func(encrypted, decrypted []byte, isShortServerID bool) {
		test := &l4lbdrv.TestDecrypt{}
		copy(test.ConnectionId[:], encrypted)
		test.Len = uint8(len(encrypted))
		if isShortServerID {
			test.IsShortServerId = 1
		} else {
			test.IsShortServerId = 0
		}
		binData, err := binary.Append(nil, binary.NativeEndian, test)
		if err != nil {
			fatal("Failed to encode test data: %v", err)
		}
		errNo, outData, err := bound.TestDecrypt.Test(binData)
		if err != nil {
			fatal("Failed to run test decrypt: %v", err)
		}
		if errNo != 0 {
			fatal("Test decrypt returned error number: %d", int32(errNo))
		}
		_, err = binary.Decode(outData, binary.NativeEndian, test)
		if err != nil {
			fatal("Failed to decode output data: %v", err)
		}
		for i := range test.ConnectionId[:len(decrypted)] {
			if test.ConnectionId[i] != decrypted[i] {
				fatal("Decrypted connection ID mismatch at index %d: got %x, want %x", i, test.ConnectionId[i], decrypted[i])
			}
		}
		t.Logf("Decrypted connection ID matches expected: %x", test.ConnectionId)
	}

	runTest(mustDecodeHex("0720b1d07b359d3c"), mustDecodeHex("07ed793aee080dbf"), false)
	runTest(mustDecodeHex("2fcc381bc74cb4fbad2823a3d1f8fed2"), mustDecodeHex("2fed793a51d49b8f5fab65ee080dbf48"), false)
	runTest(mustDecodeHex("504dd2d05a7b0de9b2b9907afb5ecf8cc3"), mustDecodeHex("50ed793a51d49b8f5fee080dbf48c0d1e5"), false)
	runTest(mustDecodeHex("125779c9cc86beb3a3a4a3ca96fce4bfe0cdbc"), mustDecodeHex("12ed793a51d49b8f5fabee080dbf48c0d1e55d"), false)
}
