package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
)

const (
	// Timelock values in blocks
	CSV_BLOCK_1_YEAR = 52596 // ~1 year (365.25 days)
	CSV_BLOCK_2_YEAR = 65535 // ~1 year, 3 months

	CSV_BLOCK_1_HOUR = 6 // ~1 hour
	CSV_BLOCK_2_HOUR = 3 // ~30 minutes (used as second delay, cumulative with first)
)

// XpubWithFingerprint holds an xpub and its fingerprint for sorting
type XpubWithFingerprint struct {
	Xpub           string
	Fingerprint    []byte
	FingerprintHex string
}

// Generate321Descriptor creates a 3-2-1 Taproot timelock descriptor from existing xpubs
// xpubs: List of cosigner xpubs (3-6, or 1-2 for testing)
// duration: "year" for production, "hour" for testing
func Generate321Descriptor(xpubs []string, duration string) (string, error) {
	if len(xpubs) < 1 || len(xpubs) > 6 {
		return "", fmt.Errorf("321 wallet requires 3-6 signers (1-2 allowed for testing); got %d", len(xpubs))
	}

	if duration != "year" && duration != "hour" {
		return "", fmt.Errorf("duration must be 'year' or 'hour', got %s", duration)
	}

	// Parse timelock values
	csvBlock1 := CSV_BLOCK_1_YEAR
	csvBlock2 := CSV_BLOCK_2_YEAR
	if duration == "hour" {
		csvBlock1 = CSV_BLOCK_1_HOUR
		csvBlock2 = CSV_BLOCK_2_HOUR
	}

	// Extract fingerprints for each xpub
	xpubsWithFp := make([]XpubWithFingerprint, len(xpubs))
	for i, xpubStr := range xpubs {
		if err := validateXpub(xpubStr); err != nil {
			return "", fmt.Errorf("invalid xpub at index %d: %w", i, err)
		}

		xpub, err := hdkeychain.NewKeyFromString(xpubStr)
		if err != nil {
			return "", fmt.Errorf("failed to parse xpub %d: %w", i, err)
		}

		// Get fingerprint from the xpub's public key
		pubKey, err := xpub.ECPubKey()
		if err != nil {
			return "", fmt.Errorf("failed to get pubkey from xpub %d: %w", i, err)
		}

		hash160 := btcutil.Hash160(pubKey.SerializeCompressed())
		fingerprint := hash160[:4]
		fingerprintHex := fmt.Sprintf("%08x", fingerprint)

		xpubsWithFp[i] = XpubWithFingerprint{
			Xpub:           xpubStr,
			Fingerprint:    fingerprint,
			FingerprintHex: fingerprintHex,
		}
	}

	// Sort by (fingerprint, xpub) - deterministic order
	sort.Slice(xpubsWithFp, func(i, j int) bool {
		// Compare fingerprints first
		cmp := strings.Compare(xpubsWithFp[i].FingerprintHex, xpubsWithFp[j].FingerprintHex)
		if cmp != 0 {
			return cmp < 0
		}
		// If fingerprints equal, compare xpubs
		return xpubsWithFp[i].Xpub < xpubsWithFp[j].Xpub
	})

	// Extract sorted xpubs
	sortedXpubs := make([]string, len(xpubsWithFp))
	for i, x := range xpubsWithFp {
		sortedXpubs[i] = x.Xpub
	}

	// Compute internal xpub (Liana-style)
	// Input: all leaf xpubs in order, repeated 3 times (for 3 spending paths)
	allLeafXpubs := make([]string, 0, len(sortedXpubs)*3)
	allLeafXpubs = append(allLeafXpubs, sortedXpubs...)
	allLeafXpubs = append(allLeafXpubs, sortedXpubs...)
	allLeafXpubs = append(allLeafXpubs, sortedXpubs...)

	internalXpub, err := InternalXpubLiana(allLeafXpubs)
	if err != nil {
		return "", fmt.Errorf("failed to compute internal xpub: %w", err)
	}

	// Build BIP-388 alias preamble (@1..@N)
	aliases := make([]string, len(xpubsWithFp))
	for idx, x := range xpubsWithFp {
		aliases[idx] = fmt.Sprintf("@%d=[%s/86'/0'/1']%s", idx+1, x.FingerprintHex, x.Xpub)
	}
	aliasPreamble := strings.Join(aliases, ";")

	// Build leaf entries for each spending path
	entries3of := make([]string, len(xpubsWithFp))
	entries2of := make([]string, len(xpubsWithFp))
	entries1of := make([]string, len(xpubsWithFp))
	for i := 0; i < len(xpubsWithFp); i++ {
		idx := i + 1
		entries3of[i] = fmt.Sprintf("@%d/<0;1>/*", idx)
		entries2of[i] = fmt.Sprintf("@%d/<2;3>/*", idx)
		entries1of[i] = fmt.Sprintf("@%d/<4;5>/*", idx)
	}

	// Build descriptor body - 3-2-1 Taproot structure
	descriptorBody := fmt.Sprintf(
		"tr(%s/<0;1>/*,{"+
			"{and_v(v:multi_a(2,%s),older(%d)),"+
			"and_v(v:multi_a(1,%s),older(%d))}"+
			",multi_a(3,%s)})",
		internalXpub,
		strings.Join(entries2of, ","), csvBlock1,
		strings.Join(entries1of, ","), csvBlock2,
		strings.Join(entries3of, ","),
	)

	// Combine alias preamble and body
	descriptorBip388 := fmt.Sprintf("%s;%s", aliasPreamble, descriptorBody)

	// Remove all spaces (descriptor spec requires no spaces)
	descriptorBip388 = strings.ReplaceAll(descriptorBip388, " ", "")

	// Add checksum
	descriptorWithChecksum := AddDescriptorChecksum(descriptorBip388)

	return descriptorWithChecksum, nil
}
