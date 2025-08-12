package l4lbdrv

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/cilium/ebpf"
)

type CryptoInitBinding struct {
	CryptoInit   *ebpf.Program `ebpf:"crypto_init"`
	CryptoCtxMap *ebpf.Map     `ebpf:"__crypto_ctx_map"`
}

func InitCrypto(binPath string, cryptoMap, cryptoMapDir string, sharedKey []byte) error {
	m, err := ReadDWARFStructs(binPath)
	if err != nil {
		return fmt.Errorf("ReadDWARFStructs(%q): %w", binPath, err)
	}
	if err := Init_cryptoAssertLayout(m); err != nil {
		return fmt.Errorf("Init_cryptoAssertLayout: %w", err)
	}
	slog.Info("Go binding type assertions for crypto passed")
	f, err := os.Open(binPath)
	if err != nil {
		return fmt.Errorf("failed to open balancer bin %q: %w", binPath, err)
	}
	defer f.Close()

	spec, err := ebpf.LoadCollectionSpecFromReader(f)
	if err != nil {
		return fmt.Errorf("failed to read spec %q: %w", binPath, err)
	}

	var cryptoMapExists bool
	var cryptoMapPass string

	if _, exists := os.Stat(cryptoMap); !os.IsNotExist(exists) {
		spec.Maps["__crypto_ctx_map"].Pinning = ebpf.PinByName
		cryptoMapExists = true
		cryptoMapPass = cryptoMapDir
	}

	var bindings CryptoInitBinding
	if err := spec.LoadAndAssign(&bindings, &ebpf.CollectionOptions{
		Programs: ebpf.ProgramOptions{
			LogLevel:     0,
			LogSizeStart: 1 * 1024 * 1024,
		},
		Maps: ebpf.MapOptions{
			PinPath: cryptoMapPass,
		},
	}); err != nil {
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			for _, line := range ve.Log {
				slog.Error("Full verifier log", slog.String("line", line))
			}
		}
		return fmt.Errorf("failed to bind spec: %w", err)
	}

	if len(sharedKey) != 16 {
		return fmt.Errorf("shared key must be 16 bytes, got %d bytes", len(sharedKey))
	}
	var quicLBContext QuiclbSharedKey
	copy(quicLBContext.Key[:], sharedKey)

	_, err = bindings.CryptoInit.Run(&ebpf.RunOptions{
		Context: &quicLBContext,
	})
	if err != nil {
		return fmt.Errorf("failed to test crypto init: %w", err)
	}

	if !cryptoMapExists {
		err = bindings.CryptoCtxMap.Pin(cryptoMap)
		if err != nil {
			return fmt.Errorf("failed to pin crypto context map %q: %w", cryptoMap, err)
		}
		slog.Info("Crypto context map pinned", slog.String("path", cryptoMap))
	} else {
		slog.Info("Crypto context map already exists, using existing map", slog.String("path", cryptoMap))
	}

	if err := bindings.CryptoInit.Close(); err != nil {
		return fmt.Errorf("failed to close crypto init program: %w", err)
	}
	if err := bindings.CryptoCtxMap.Close(); err != nil {
		return fmt.Errorf("failed to close crypto context map: %w", err)
	}

	return nil
}
