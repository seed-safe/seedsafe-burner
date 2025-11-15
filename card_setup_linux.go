//go:build linux

package main

import (
	"os"
	"os/exec"
	"strings"
)

// getBootPartitionNamePlatform reads boot partition label on Linux (RPi)
func getBootPartitionNamePlatform() (string, error) {
	// Method 1: Try lsblk command for /boot partition
	cmd := exec.Command("lsblk", "-no", "LABEL", "/dev/mmcblk0p1")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		label := strings.TrimSpace(string(output))
		if label != "" {
			return label, nil
		}
	}

	// Method 2: Try blkid
	cmd = exec.Command("blkid", "-s", "LABEL", "-o", "value", "/dev/mmcblk0p1")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		label := strings.TrimSpace(string(output))
		if label != "" {
			return label, nil
		}
	}

	// Method 3: Fallback to hostname
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		return hostname, nil
	}

	// Last resort default
	return "RPiDefault1", nil
}
