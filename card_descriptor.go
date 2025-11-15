package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
)

// DescriptorData holds the multisig descriptor information
type DescriptorData struct {
	Descriptor string   // Full descriptor string
	Xpubs      []string // Sorted list of xpubs
	Quorum     int      // Required signatures (e.g., 3 for 3-of-5)
}

// CollectXpubsManual prompts user to paste 5 xpubs
func CollectXpubsManual() ([]string, error) {
	fmt.Println("\n=== Descriptor Generation (Phase 1: Manual Input) ===")
	fmt.Println("Please paste 5 xpubs (one per line):")
	fmt.Println("(Press Enter after each xpub)")

	scanner := bufio.NewScanner(os.Stdin)
	xpubs := make([]string, 0, 5)

	for i := 1; i <= 5; i++ {
		fmt.Printf("\nXpub %d: ", i)
		if !scanner.Scan() {
			return nil, fmt.Errorf("failed to read xpub %d", i)
		}

		xpub := strings.TrimSpace(scanner.Text())
		if xpub == "" {
			return nil, fmt.Errorf("empty xpub at position %d", i)
		}

		// Validate xpub format
		if err := validateXpub(xpub); err != nil {
			return nil, fmt.Errorf("invalid xpub %d: %w", i, err)
		}

		// Check for duplicates
		for j, existing := range xpubs {
			if existing == xpub {
				return nil, fmt.Errorf("duplicate xpub: position %d and %d are identical", j+1, i)
			}
		}

		xpubs = append(xpubs, xpub)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return xpubs, nil
}

// validateXpub checks if the xpub is valid BIP32 extended public key
func validateXpub(xpub string) error {
	// Try to decode as mainnet xpub
	_, err := hdkeychain.NewKeyFromString(xpub)
	if err != nil {
		return fmt.Errorf("invalid xpub format: %w", err)
	}
	return nil
}

// GenerateSortedMultisigDescriptor creates a 3-of-5 sorted multisig descriptor
func GenerateSortedMultisigDescriptor(xpubs []string) (*DescriptorData, error) {
	if len(xpubs) != 5 {
		return nil, fmt.Errorf("expected 5 xpubs, got %d", len(xpubs))
	}

	// Validate all xpubs
	for i, xpub := range xpubs {
		if err := validateXpub(xpub); err != nil {
			return nil, fmt.Errorf("xpub %d validation failed: %w", i+1, err)
		}
	}

	// Sort xpubs lexicographically (standard for sortedmulti)
	sortedXpubs := make([]string, len(xpubs))
	copy(sortedXpubs, xpubs)
	sort.Strings(sortedXpubs)

	// Build descriptor string: wsh(sortedmulti(3,xpub1,xpub2,xpub3,xpub4,xpub5))
	// Using standard derivation path /0/* for receive addresses
	descriptor := fmt.Sprintf("wsh(sortedmulti(3,%s/0/*,%s/0/*,%s/0/*,%s/0/*,%s/0/*))",
		sortedXpubs[0], sortedXpubs[1], sortedXpubs[2], sortedXpubs[3], sortedXpubs[4])

	return &DescriptorData{
		Descriptor: descriptor,
		Xpubs:      sortedXpubs,
		Quorum:     3,
	}, nil
}

// ExtractXpubFromCard reads xpub from a card's MetadataQR (format: "xpub:name")
func ExtractXpubFromCard(metadataQR string) (string, string, error) {
	parts := strings.SplitN(metadataQR, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid metadata format, expected 'xpub:name'")
	}

	xpub := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])

	if err := validateXpub(xpub); err != nil {
		return "", "", fmt.Errorf("invalid xpub in metadata: %w", err)
	}

	return xpub, name, nil
}

// GetXpubFingerprint extracts the first 8 hex chars from an xpub for display
func GetXpubFingerprint(xpubStr string) (string, error) {
	xpub, err := hdkeychain.NewKeyFromString(xpubStr)
	if err != nil {
		return "", err
	}

	// Get the parent fingerprint (first 4 bytes)
	pubKey, err := xpub.ECPubKey()
	if err != nil {
		return "", err
	}

	hash160 := btcutil.Hash160(pubKey.SerializeCompressed())
	return fmt.Sprintf("%08x", hash160[0:4]), nil
}
