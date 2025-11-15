package main

import (
	"strings"
)

// Bitcoin descriptor checksum algorithm (based on BCH codes)
// Reference: https://github.com/bitcoin/bitcoin/blob/master/src/script/descriptor.cpp

const inputCharset = "0123456789()[],'/*abcdefgh@:$%{}IJKLMNOPQRSTUVWXYZ&+-.;<=>?!^_|~ijklmnopqrstuvwxyzABCDEFGH`#\"\\ "
const checksumCharset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

var generator = []uint64{0xf5dee51989, 0xa9fdca3312, 0x1bab10e32d, 0x3706b1677a, 0x644d626ffd}

// polymod computes the descriptor checksum polymod
func polymod(symbols []uint64) uint64 {
	chk := uint64(1)
	for _, value := range symbols {
		top := chk >> 35
		chk = (chk&0x7ffffffff)<<5 ^ value
		for i := 0; i < 5; i++ {
			if (top>>uint(i))&1 == 1 {
				chk ^= generator[i]
			}
		}
	}
	return chk
}

// descriptorChecksum computes the checksum for a descriptor string
func descriptorChecksum(desc string) string {
	// Expand the descriptor string into symbols
	symbols := make([]uint64, 0, len(desc)+8)

	for _, c := range desc {
		pos := strings.IndexRune(inputCharset, c)
		if pos == -1 {
			// Invalid character, skip or handle error
			continue
		}
		symbols = append(symbols, uint64(pos))
	}

	// Append 8 zeros for the checksum
	for i := 0; i < 8; i++ {
		symbols = append(symbols, 0)
	}

	// Compute polymod
	poly := polymod(symbols) ^ 1

	// Convert polymod to checksum string (8 characters)
	checksum := make([]byte, 8)
	for i := 0; i < 8; i++ {
		checksum[i] = checksumCharset[(poly>>(5*(7-i)))&31]
	}

	return string(checksum)
}

// AddDescriptorChecksum adds a checksum to a descriptor string
func AddDescriptorChecksum(desc string) string {
	checksum := descriptorChecksum(desc)
	return desc + "#" + checksum
}

// VerifyDescriptorChecksum verifies a descriptor's checksum
func VerifyDescriptorChecksum(desc string) bool {
	parts := strings.Split(desc, "#")
	if len(parts) != 2 {
		return false
	}

	expectedChecksum := descriptorChecksum(parts[0])
	return parts[1] == expectedChecksum
}
