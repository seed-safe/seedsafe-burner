//go:build darwin

package main

import (
	"os"
	"os/exec"
	"strings"
)

// getBootPartitionNamePlatform reads boot partition label on macOS
func getBootPartitionNamePlatform() (string, error) {
	// Method 1: Try to find boot volume label using diskutil
	cmd := exec.Command("diskutil", "info", "/")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Volume Name:") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					label := strings.TrimSpace(parts[1])
					if label != "" {
						return label, nil
					}
				}
			}
		}
	}

	// Method 2: Try computer name
	cmd = exec.Command("scutil", "--get", "ComputerName")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		name := strings.TrimSpace(string(output))
		if name != "" {
			return name, nil
		}
	}

	// Method 3: Fallback to hostname
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		return hostname, nil
	}

	// Last resort default
	return "MacDefault1", nil
}
