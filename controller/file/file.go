package file

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/yzp0n/ncdn/controller/chunk"
)

func MaySaveFile(chunked *chunk.Chunk) (bool, string, fs.FileMode, error) {
	if chunked == nil || chunked.Msg == nil {
		return false, "", 0, nil
	}
	if file := chunked.Msg.FileTransfer(); file != nil {
		err := os.WriteFile(string(file.Path), chunked.Data, os.FileMode(file.Permission))
		if err != nil {
			return true, "", 0, fmt.Errorf("failed to write file %s: %w", file.Path, err)
		}
		err = os.Chmod(string(file.Path), os.FileMode(file.Permission))
		if err != nil {
			return true, "", 0, fmt.Errorf("failed to set permission for file %s: %w", file.Path, err)
		}
		return true, string(file.Path), os.FileMode(file.Permission), nil
	}
	return false, "", 0, nil
}
