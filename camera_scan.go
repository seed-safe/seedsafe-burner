package main

import (
	"bufio"
	"fmt"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/liyue201/goqr"
)

const (
	defaultCaptureTool = "libcamera-still"
	defaultCaptureDir  = "/tmp"
)

// CollectXpubsViaCamera captures xpub:name QR codes with the Pi camera until targetCount unique values are collected.
func CollectXpubsViaCamera(targetCount int, scanner *bufio.Scanner) ([]string, error) {
	if targetCount <= 0 {
		return nil, fmt.Errorf("target count must be positive")
	}

	if _, err := exec.LookPath(defaultCaptureTool); err != nil {
		return nil, fmt.Errorf("%s not found in PATH: %w", defaultCaptureTool, err)
	}

	fmt.Println("\n=== Descriptor Generation (Pi Camera Input) ===")
	fmt.Println("Hold each SeedSafe card so the camera sees the metadata QR (xpub:name).")
	fmt.Println("Press Enter to capture each card, or type 'q' to cancel.")

	collected := make([]string, 0, targetCount)
	seen := make(map[string]bool)

	for len(collected) < targetCount {
		fmt.Printf("\nReady to scan card %d/%d. Press Enter to capture (or 'q' to quit): ", len(collected)+1, targetCount)
		if !scanner.Scan() {
			return nil, fmt.Errorf("input aborted while waiting to scan")
		}
		input := strings.TrimSpace(scanner.Text())
		if strings.EqualFold(input, "q") {
			return nil, fmt.Errorf("scan cancelled by user")
		}

		xpub, name, err := captureXpubFromCamera()
		if err != nil {
			fmt.Printf("✗ Scan failed: %v\n", err)
			continue
		}

		if seen[xpub] {
			fmt.Println("⚠ Duplicate xpub detected, ignoring. Present a different card.")
			continue
		}

		seen[xpub] = true
		collected = append(collected, xpub)

		displayName := name
		if displayName == "" {
			displayName = "(no label)"
		}

		fmt.Printf("✓ Captured: %s… (%s)\n", shortenString(xpub, 12), displayName)
	}

	fmt.Println("\nCaptured xpubs:")
	for i, xp := range collected {
		fp, err := GetXpubFingerprint(xp)
		if err != nil {
			fp = "????????"
		}
		fmt.Printf("  %d) %s… fp=%s\n", i+1, shortenString(xp, 16), fp)
	}

	return collected, nil
}

func captureXpubFromCamera() (string, string, error) {
	filename := filepath.Join(defaultCaptureDir, fmt.Sprintf("seedsafe-xpub-%d.jpg", time.Now().UnixNano()))
	defer os.Remove(filename)

	args := []string{
		"-n",           // no preview
		"--timeout", "800", // ms exposure
		"--immediate",
		"--width", "1280",
		"--height", "720",
		"-o", filename,
	}

	cmd := exec.Command(defaultCaptureTool, args...)
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to capture image with %s: %w", defaultCaptureTool, err)
	}

	file, err := os.Open(filename)
	if err != nil {
		return "", "", fmt.Errorf("failed to open capture: %w", err)
	}
	defer file.Close()

	img, err := jpeg.Decode(file)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode capture: %w", err)
	}

	codes, err := goqr.Recognize(img)
	if err != nil {
		return "", "", fmt.Errorf("failed to recognize QR: %w", err)
	}
	if len(codes) == 0 {
		return "", "", fmt.Errorf("no QR code detected")
	}

	for _, code := range codes {
		payload := strings.TrimSpace(code.PayloadString())
		xpub, name, err := ExtractXpubFromCard(payload)
		if err == nil {
			return xpub, name, nil
		}
	}

	return "", "", fmt.Errorf("no xpub:name QR found in capture")
}
