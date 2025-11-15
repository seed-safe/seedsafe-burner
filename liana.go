package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
)

// BIP341 NUMS (Nothing Up My Sleeve) point - compressed format
// This is an unspendable key with no known discrete logarithm
const numsPointHex = "0250929b74c1a04954b78b4b6035e97a5e078a5a0f28ec96d547bfee9ace803ac0"

// InternalXpubLiana creates a Liana-style unspendable internal xpub
// The chaincode is derived from SHA256(concat of all cosigner pubkeys in order)
// The public key is the BIP341 NUMS point
func InternalXpubLiana(cosignerXpubs []string) (string, error) {
	if len(cosignerXpubs) == 0 {
		return "", fmt.Errorf("provide at least one xpub")
	}

	// Parse all xpubs and extract their compressed public keys (33 bytes each)
	var pubkeysConcat []byte
	for i, xpubStr := range cosignerXpubs {
		xpub, err := hdkeychain.NewKeyFromString(xpubStr)
		if err != nil {
			return "", fmt.Errorf("invalid xpub at index %d: %w", i, err)
		}

		pubkey, err := xpub.ECPubKey()
		if err != nil {
			return "", fmt.Errorf("failed to get pubkey from xpub %d: %w", i, err)
		}

		// Append compressed public key (33 bytes)
		pubkeysConcat = append(pubkeysConcat, pubkey.SerializeCompressed()...)
	}

	// Compute chaincode = SHA256(concatenated pubkeys)
	chaincode := sha256.Sum256(pubkeysConcat)

	// Parse NUMS point
	numsBytes, err := hex.DecodeString(numsPointHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode NUMS point: %w", err)
	}

	numsPubkey, err := btcec.ParsePubKey(numsBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse NUMS pubkey: %w", err)
	}

	// Build extended key with:
	// - depth = 0
	// - parent fingerprint = 0x00000000
	// - child number = 0
	// - chaincode = computed above
	// - public key = NUMS point
	// - version = mainnet xpub
	internalKey := hdkeychain.NewExtendedKey(
		chaincfg.MainNetParams.HDPublicKeyID[:], // mainnet xpub version
		numsPubkey.SerializeCompressed(),        // NUMS point (33 bytes)
		chaincode[:],                             // chaincode (32 bytes)
		[]byte{0, 0, 0, 0},                       // parent fingerprint
		0,                                        // depth
		0,                                        // child number
		false,                                    // isPrivate = false
	)

	return internalKey.String(), nil
}
