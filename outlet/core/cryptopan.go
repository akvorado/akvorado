package core

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"net"
)

// CryptoPAN implements prefix-preserving anonymization for IPv4 and IPv6.
type CryptoPAN struct {
	block cipher.Block
}

// NewCryptoPAN constructs a CryptoPAN with a 16/24/32 byte key (AES-128/192/256).
func NewCryptoPAN(key []byte) (*CryptoPAN, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("cryptopan: key must be 16, 24 or 32 bytes")
	}
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return &CryptoPAN{block: b}, nil
}

// AnonymizeIPv4 returns an anonymized IPv4 address or nil if ip is not IPv4.
func (c *CryptoPAN) AnonymizeIPv4(ip net.IP) net.IP {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}
	orig := binary.BigEndian.Uint32(ip4)
	var anon uint32

	var block [16]byte
	var out [16]byte

	for i := 0; i < 32; i++ {
		zeroBlock(&block)
		writePrefixBitsToBlock(&block, anon, i)
		c.block.Encrypt(out[:], block[:])
		prfBit := (out[0] >> 7) & 1
		origBit := (orig >> (31 - i)) & 1
		newBit := origBit ^ uint32(prfBit)
		anon = (anon << 1) | newBit
	}

	outIP := make(net.IP, 4)
	binary.BigEndian.PutUint32(outIP, anon)
	return outIP
}

// AnonymizeIPv6 returns an anonymized IPv6 address or nil if ip is not IPv6.
// This extends the same bit-by-bit construction to 128 bits. It consumes
// multiple AES outputs as needed.
func (c *CryptoPAN) AnonymizeIPv6(ip net.IP) net.IP {
	ip16 := ip.To16()
	if ip16 == nil || ip.To4() != nil {
		return nil
	}
	// read original 128 bits as two uint64
	origHigh := binary.BigEndian.Uint64(ip16[:8])
	origLow := binary.BigEndian.Uint64(ip16[8:])
	var anonHigh uint64
	var anonLow uint64

	var block [16]byte
	var out [16]byte
	// we will build anon bits one by one, tracking index 0..127
	for i := 0; i < 128; i++ {
		zeroBlock(&block)
		// write anonymized prefix so far: combine anonHigh/anonLow for bits built
		writePrefixBitsToBlock128(&block, anonHigh, anonLow, i)
		c.block.Encrypt(out[:], block[:])
		prfBit := (out[0] >> 7) & 1
		var origBit uint64
		if i < 64 {
			origBit = (origHigh >> (63 - i)) & 1
		} else {
			origBit = (origLow >> (127 - i)) & 1
		}
		newBit := origBit ^ uint64(prfBit)
		if i < 64 {
			anonHigh = (anonHigh << 1) | newBit
		} else {
			anonLow = (anonLow << 1) | newBit
		}
	}

	outIP := make(net.IP, 16)
	binary.BigEndian.PutUint64(outIP[:8], anonHigh)
	binary.BigEndian.PutUint64(outIP[8:], anonLow)
	return outIP
}

func zeroBlock(b *[16]byte) {
	for i := range b {
		b[i] = 0
	}
}

// writePrefixBitsToBlock writes the top 'bits' of anonSoFar (32-bit) into block.
func writePrefixBitsToBlock(block *[16]byte, anonSoFar uint32, bits int) {
	for j := 0; j < bits; j++ {
		srcIdx := bits - 1 - j
		bit := (anonSoFar >> srcIdx) & 1
		byteIdx := j / 8
		bitIdx := 7 - (j % 8)
		if bit == 1 {
			block[byteIdx] |= (1 << bitIdx)
		}
	}
}

// writePrefixBitsToBlock128 writes the top 'bits' of anonSoFar (128-bit via two uint64) into block.
func writePrefixBitsToBlock128(block *[16]byte, high, low uint64, bits int) {
	for j := 0; j < bits; j++ {
		srcIdx := bits - 1 - j
		var bit uint64
		if srcIdx < 64 {
			bit = (high >> srcIdx) & 1
		} else {
			bit = (low >> (srcIdx - 64)) & 1
		}
		byteIdx := j / 8
		bitIdx := 7 - (j % 8)
		if bit == 1 {
			block[byteIdx] |= (1 << bitIdx)
		}
	}
}
