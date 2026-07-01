package crackx

// This file implements the legacy ZipCrypto (traditional PKWARE) stream
// cipher. It is the cryptographic foundation for the known-plaintext attack
// (Biham–Kocher) added in this package: the attack recovers the cipher's
// 96-bit internal state, after which these primitives decrypt the archive.
//
// The cipher keeps three 32-bit words (key0, key1, key2). Each processed
// plaintext byte advances the state; a keystream byte derived from key2 is
// XORed with the data. References: PKWARE APPNOTE section 6.1 and Info-ZIP's
// crypt.c. Implemented from the published algorithm; no third-party code.

// zipCryptoMult is the multiplier used when mixing key1. It is the well-known
// constant 134775813 (0x08088405). zipCryptoMultInv is its multiplicative
// inverse modulo 2^32, used to step the cipher backward during the attack.
const (
	zipCryptoMult    = 0x08088405
	zipCryptoMultInv = 0xd94fa8cd
)

// crc32Tab is the standard reflected CRC-32 table (polynomial 0xEDB88320) that
// the cipher uses to fold bytes into key0 and key2.
var crc32Tab = makeCRC32Tab()

func makeCRC32Tab() [256]uint32 {
	const poly = 0xEDB88320
	var t [256]uint32
	for i := range t {
		c := uint32(i)
		for range 8 {
			if c&1 != 0 {
				c = poly ^ (c >> 1)
			} else {
				c >>= 1
			}
		}
		t[i] = c
	}
	return t
}

// crc32Byte folds one byte into a running CRC-32 value, matching the cipher's
// CRC32(c, b) macro: (c >> 8) ^ tab[(c ^ b) & 0xff].
func crc32Byte(crc uint32, b byte) uint32 {
	return (crc >> 8) ^ crc32Tab[byte(crc)^b]
}

// crc32InvTab inverts crc32Byte: it is indexed by the most significant byte of
// a CRC value. The known-plaintext attack steps the cipher backward, which
// requires undoing the CRC-32 folding.
var crc32InvTab = makeCRC32InvTab()

func makeCRC32InvTab() [256]uint32 {
	var t [256]uint32
	for b := 0; b < 256; b++ {
		crc := crc32Tab[b]
		t[crc>>24] = (crc << 8) ^ uint32(b)
	}
	return t
}

// crc32ByteInv inverts crc32Byte: crc32ByteInv(crc32Byte(c, b), b) == c.
func crc32ByteInv(crc uint32, b byte) uint32 {
	return (crc << 8) ^ crc32InvTab[crc>>24] ^ uint32(b)
}

// zipCryptoKeys is the 96-bit internal state of the ZipCrypto cipher.
type zipCryptoKeys struct {
	key0, key1, key2 uint32
}

// newZipCryptoKeys returns the fixed initial state defined by the format,
// before the password (or encryption header) is mixed in.
func newZipCryptoKeys() zipCryptoKeys {
	return zipCryptoKeys{key0: 0x12345678, key1: 0x23456789, key2: 0x34567890}
}

// initWithPassword resets the state and mixes in each password byte. This is
// how a decryptor that knows the password derives the starting keys; the
// known-plaintext attack instead recovers the keys directly.
func (k *zipCryptoKeys) initWithPassword(password string) {
	*k = newZipCryptoKeys()
	for i := 0; i < len(password); i++ {
		k.update(password[i])
	}
}

// update advances the key state with one plaintext byte.
func (k *zipCryptoKeys) update(p byte) {
	k.key0 = crc32Byte(k.key0, p)
	k.key1 = (k.key1+(k.key0&0xff))*zipCryptoMult + 1
	k.key2 = crc32Byte(k.key2, byte(k.key1>>24))
}

// streamByte returns the next keystream byte derived from key2.
func (k zipCryptoKeys) streamByte() byte {
	temp := uint16(k.key2) | 2
	return byte((uint32(temp) * uint32(temp^1)) >> 8)
}

// encryptByte encrypts one plaintext byte, advancing the key state.
func (k *zipCryptoKeys) encryptByte(p byte) byte {
	c := p ^ k.streamByte()
	k.update(p)
	return c
}

// decryptByte decrypts one ciphertext byte, advancing the key state.
func (k *zipCryptoKeys) decryptByte(c byte) byte {
	p := c ^ k.streamByte()
	k.update(p)
	return p
}

// updateBackward steps the key state back by one position given the ciphertext
// byte c that was produced there. After it returns, streamByte yields the
// keystream byte for the now-current (earlier) position, matching the byte used
// internally to invert key0. This is the inverse of update and is the
// workhorse of recovering the initial keys after the attack.
func (k *zipCryptoKeys) updateBackward(c byte) {
	k.key2 = crc32ByteInv(k.key2, byte(k.key1>>24))
	k.key1 = (k.key1-1)*zipCryptoMultInv - uint32(byte(k.key0))
	k.key0 = crc32ByteInv(k.key0, c^k.streamByte())
}

// updateBackwardPlaintext steps the state back by one position given the
// plaintext byte p that was processed there (as opposed to the ciphertext byte
// used by updateBackward). Used by password recovery.
func (k *zipCryptoKeys) updateBackwardPlaintext(p byte) {
	k.key2 = crc32ByteInv(k.key2, byte(k.key1>>24))
	k.key1 = (k.key1-1)*zipCryptoMultInv - uint32(byte(k.key0))
	k.key0 = crc32ByteInv(k.key0, p)
}

// updateBackwardRange steps the state backward over ciphertext[target:current],
// from index current-1 down to target. Used to walk from the start of the known
// plaintext back to the very beginning, recovering the password-derived keys.
func (k *zipCryptoKeys) updateBackwardRange(ciphertext []byte, current, target int) {
	for i := current - 1; i >= target; i-- {
		k.updateBackward(ciphertext[i])
	}
}
