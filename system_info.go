package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	imageHashOnce sync.Once
	imageHash     string
	imageHashErr  error
)

const defaultImageDevice = "/dev/mmcblk0"

// GetImageHash returns the SHA256 of the SD card image the Pi booted from.
func GetImageHash() (string, error) {
	imageHashOnce.Do(func() {
		imageHash, imageHashErr = computeImageHash()
	})
	return imageHash, imageHashErr
}

func computeImageHash() (string, error) {
	if os.Getenv("SEEDSAFE_SKIP_FULL_HASH") == "1" {
		return "", errors.New("full image hash disabled")
	}
	if runtime.GOOS != "linux" {
		return "", errors.New("full image hash only supported on linux")
	}
	device := os.Getenv("SEEDSAFE_IMAGE_DEVICE")
	if device == "" {
		device = defaultImageDevice
	}

	f, err := os.Open(device)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", device, err)
	}
	defer f.Close()

	h := sha256.New()
	reader := bufio.NewReaderSize(f, 4*1024*1024)
	buf := make([]byte, 4*1024*1024)
	var total int64
	start := time.Now()

	for {
		n, rerr := reader.Read(buf)
		if n > 0 {
			total += int64(n)
			if _, werr := h.Write(buf[:n]); werr != nil {
				return "", fmt.Errorf("hash write: %w", werr)
			}
			if total%(256*1024*1024) == 0 {
				fmt.Printf("  Hashing SD card... %d MB processed\r", total/(1024*1024))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", fmt.Errorf("read device: %w", rerr)
		}
	}
	fmt.Printf("  Hashing SD card... done in %s (%d MB)\n", time.Since(start).Round(time.Second), total/(1024*1024))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// GetDeviceSerial returns the Raspberry Pi board serial number.
func GetDeviceSerial() string {
	paths := []string{
		"/sys/firmware/devicetree/base/serial-number",
		"/proc/cpuinfo",
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if p == "/proc/cpuinfo" {
			if serial := parseCPUInfoSerial(string(data)); serial != "" {
				return serial
			}
			continue
		}
		return strings.TrimSpace(string(data))
	}
	return "unknown"
}

func parseCPUInfoSerial(contents string) string {
	for _, line := range strings.Split(contents, "\n") {
		if strings.HasPrefix(strings.ToLower(line), "serial") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// GetBurnDate returns today's date in YYYY-MM-DD (UTC).
func GetBurnDate() string {
	return time.Now().UTC().Format("2006-01-02")
}
