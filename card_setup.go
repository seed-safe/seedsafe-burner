package main

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/tyler-smith/go-bip39"
)

const AbandonMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

// CardData contains all information needed to generate and burn a card
type CardData struct {
	Name        string    // 11 chars from boot partition
	Mnemonic    string    // Always abandon mnemonic
	Fingerprint [4]byte   // BIP32 master key fingerprint
	PubKey      [33]byte  // Compressed public key
	ChainCode   [32]byte  // BIP32 chain code
	Xpub        string    // BIP32 extended public key (base58)
}

// GetBootPartitionName reads the 11-char name from boot partition
// Platform-specific implementation in card_setup_linux.go and card_setup_darwin.go
func GetBootPartitionName() (string, error) {
	name, err := getBootPartitionNamePlatform()
	if err != nil {
		return "", err
	}
	return padOrTruncate(name, 11), nil
}

// GenerateCardData creates all data for the card (in memory)
func GenerateCardData() (*CardData, error) {
	name, err := GetBootPartitionName()
	if err != nil {
		return nil, fmt.Errorf("failed to get boot partition name: %w", err)
	}

	// Generate BIP32 master key from abandon mnemonic
	seed := bip39.NewSeed(AbandonMnemonic, "")
	master, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	// Extract xpub string
	xpub := master.String()

	// Extract pubkey and chaincode
	pubKeyBytes, err := master.ECPubKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}
	pubKey := pubKeyBytes.SerializeCompressed() // 33 bytes
	chainCode := master.ChainCode()              // 32 bytes

	// Calculate fingerprint: first 4 bytes of HASH160(pubkey)
	hash160 := btcutil.Hash160(pubKey)
	var fingerprint [4]byte
	copy(fingerprint[:], hash160[0:4])

	var pubKey33 [33]byte
	var chainCode32 [32]byte
	copy(pubKey33[:], pubKey)
	copy(chainCode32[:], chainCode)

	return &CardData{
		Name:        name,
		Mnemonic:    AbandonMnemonic,
		Fingerprint: fingerprint,
		PubKey:      pubKey33,
		ChainCode:   chainCode32,
		Xpub:        xpub,
	}, nil
}

// EncodeMetadata creates xpub:name string for MetadataQR
// Format: xpub (111 chars) + colon + name (11 chars) = ~122 chars
// Standard wallets will parse xpub up to colon, custom tools can extract both
func (c *CardData) EncodeMetadata() string {
	return fmt.Sprintf("%s:%s", c.Xpub, c.Name)
}

// padOrTruncate ensures string is exactly the specified length
func padOrTruncate(s string, length int) string {
	if len(s) > length {
		return s[:length]
	}
	for len(s) < length {
		s += " "
	}
	return s
}
