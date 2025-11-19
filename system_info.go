package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
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

// getImageSize determines the image size by reading the MBR partition table.
// It calculates the end of the last partition to determine the actual image size.
func getImageSize() (int64, error) {
	// Support direct size via environment variable for testing
	if sizeStr := os.Getenv("SEEDSAFE_IMAGE_SIZE"); sizeStr != "" {
		size, err := strconv.ParseInt(strings.TrimSpace(sizeStr), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse SEEDSAFE_IMAGE_SIZE: %w", err)
		}
		return size, nil
	}

	device := os.Getenv("SEEDSAFE_IMAGE_DEVICE")
	if device == "" {
		device = defaultImageDevice
	}

	f, err := os.Open(device)
	if err != nil {
		return 0, fmt.Errorf("open device %s: %w", device, err)
	}
	defer f.Close()

	// Read MBR (first 512 bytes)
	mbr := make([]byte, 512)
	if _, err := io.ReadFull(f, mbr); err != nil {
		return 0, fmt.Errorf("read MBR: %w", err)
	}

	// Verify MBR signature (0x55AA at offset 510)
	if mbr[510] != 0x55 || mbr[511] != 0xAA {
		return 0, errors.New("invalid MBR signature")
	}

	// Parse partition table entries (4 entries at offset 446, 16 bytes each)
	var maxEnd int64
	for i := 0; i < 4; i++ {
		offset := 446 + (i * 16)

		// Partition type at offset 4 (0 = unused)
		partType := mbr[offset+4]
		if partType == 0 {
			continue
		}

		// LBA start at offset 8 (4 bytes, little-endian)
		lbaStart := binary.LittleEndian.Uint32(mbr[offset+8 : offset+12])
		// Number of sectors at offset 12 (4 bytes, little-endian)
		numSectors := binary.LittleEndian.Uint32(mbr[offset+12 : offset+16])

		partEnd := int64(lbaStart+numSectors) * 512
		if partEnd > maxEnd {
			maxEnd = partEnd
		}
	}

	if maxEnd == 0 {
		return 0, errors.New("no partitions found in MBR")
	}

	return maxEnd, nil
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

	// Get expected image size (only hash this many bytes, not entire SD card)
	imageSize, err := getImageSize()
	if err != nil {
		return "", fmt.Errorf("get image size: %w", err)
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
	remaining := imageSize
	start := time.Now()

	for remaining > 0 {
		toRead := int64(len(buf))
		if toRead > remaining {
			toRead = remaining
		}

		n, rerr := reader.Read(buf[:toRead])
		if n > 0 {
			remaining -= int64(n)
			total += int64(n)
			if _, werr := h.Write(buf[:n]); werr != nil {
				return "", fmt.Errorf("hash write: %w", werr)
			}
			if total%(32*1024*1024) == 0 {
				fmt.Printf("  Hashing image... %d / %d MB\r", total/(1024*1024), imageSize/(1024*1024))
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return "", fmt.Errorf("read device: %w", rerr)
		}
	}
	fmt.Printf("  Hashing image... done in %s (%d MB)\n", time.Since(start).Round(time.Second), total/(1024*1024))
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
