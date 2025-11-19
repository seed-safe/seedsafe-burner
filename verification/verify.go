package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// VerifyBootFiles hashes each expected boot file under bootMountPath and returns
// a slice of human-readable error messages for mismatches or read failures.
func VerifyBootFiles(bootMountPath string) []string {
	if len(ExpectedFileHashes) == 0 {
		return nil
	}

	var failures []string
	for file, expected := range ExpectedFileHashes {
		target := filepath.Join(bootMountPath, file)
		actual, err := hashFile(target)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", file, err))
			continue
		}
		if !strings.EqualFold(actual, expected) {
			failures = append(failures, fmt.Sprintf("%s mismatch (expected %s, got %s)", file, expected, actual))
		}
	}
	return failures
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
