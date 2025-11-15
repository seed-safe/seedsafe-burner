package main

import (
	"crypto/hmac"
	"crypto/sha512"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/tyler-smith/go-bip39"
)

// BIP85DeriveBIP39 derives a child BIP39 mnemonic from a master extended key
// Path: m/83696968'/39'/0'/words'/index'
// words: 12 or 24
// index: child index (1-based in the spec, but we use 0-based internally)
func BIP85DeriveBIP39(masterKey *hdkeychain.ExtendedKey, index int, words int) (string, error) {
	if words != 12 && words != 24 {
		return "", fmt.Errorf("words must be 12 or 24, got %d", words)
	}

	// BIP-85 path: m/83696968'/39'/0'/words'/index'
	const bip85Purpose = uint32(0x80000000 + 83696968)
	const application = uint32(0x80000000 + 39)
	const reserved = uint32(0x80000000 + 0)

	// Derive the path
	key := masterKey
	var err error

	// m/83696968'
	key, err = key.Derive(bip85Purpose)
	if err != nil {
		return "", fmt.Errorf("derive purpose: %w", err)
	}

	// m/83696968'/39'
	key, err = key.Derive(application)
	if err != nil {
		return "", fmt.Errorf("derive application: %w", err)
	}

	// m/83696968'/39'/0'
	key, err = key.Derive(reserved)
	if err != nil {
		return "", fmt.Errorf("derive reserved: %w", err)
	}

	// m/83696968'/39'/0'/words'
	wordsHardened := uint32(0x80000000) + uint32(words)
	key, err = key.Derive(wordsHardened)
	if err != nil {
		return "", fmt.Errorf("derive words: %w", err)
	}

	// m/83696968'/39'/0'/words'/index'
	indexHardened := uint32(0x80000000) + uint32(index)
	key, err = key.Derive(indexHardened)
	if err != nil {
		return "", fmt.Errorf("derive index: %w", err)
	}

	// Extract entropy from HMAC-SHA512(key="bip-entropy-from-k", data=derived_private_key)
	privKey, err := key.ECPrivKey()
	if err != nil {
		return "", fmt.Errorf("get private key: %w", err)
	}

	h := hmac.New(sha512.New, []byte("bip-entropy-from-k"))
	h.Write(privKey.Serialize())
	entropy := h.Sum(nil)

	// Use first 16 bytes (128 bits) for 12 words, 32 bytes (256 bits) for 24 words
	entropyLen := 16
	if words == 24 {
		entropyLen = 32
	}
	entropy = entropy[:entropyLen]

	// Generate mnemonic from entropy
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("generate mnemonic: %w", err)
	}

	return mnemonic, nil
}

// DeriveAccount derives BIP86 Taproot account xpub at m/86'/0'/account'
func DeriveAccount(masterKey *hdkeychain.ExtendedKey, account uint32) (*hdkeychain.ExtendedKey, error) {
	// m/86'
	key, err := masterKey.Derive(hdkeychain.HardenedKeyStart + 86)
	if err != nil {
		return nil, fmt.Errorf("derive purpose: %w", err)
	}

	// m/86'/0'
	key, err = key.Derive(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return nil, fmt.Errorf("derive coin type: %w", err)
	}

	// m/86'/0'/account'
	key, err = key.Derive(hdkeychain.HardenedKeyStart + account)
	if err != nil {
		return nil, fmt.Errorf("derive account: %w", err)
	}

	return key, nil
}

// XpubFromAccount derives account xpub and returns base58 string
func XpubFromAccount(masterKey *hdkeychain.ExtendedKey, account uint32) (string, error) {
	accountKey, err := DeriveAccount(masterKey, account)
	if err != nil {
		return "", err
	}

	// Convert to public key
	pubKey, err := accountKey.Neuter()
	if err != nil {
		return "", fmt.Errorf("neuter key: %w", err)
	}

	return pubKey.String(), nil
}

// GetFingerprint returns the 4-byte fingerprint of a master key
func GetFingerprint(key *hdkeychain.ExtendedKey) ([4]byte, error) {
	pubKey, err := key.ECPubKey()
	if err != nil {
		return [4]byte{}, err
	}

	// Get first 4 bytes of HASH160(pubkey)
	hash160 := btcutil.Hash160(pubKey.SerializeCompressed())
	var fp [4]byte
	copy(fp[:], hash160[:4])
	return fp, nil
}

// MasterKeyFromMnemonic creates a master extended key from mnemonic
func MasterKeyFromMnemonic(mnemonic string) (*hdkeychain.ExtendedKey, error) {
	seed := bip39.NewSeed(mnemonic, "")
	return hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
}
