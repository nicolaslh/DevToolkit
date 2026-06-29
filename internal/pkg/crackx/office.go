package crackx

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"hash"
	"io"
	"unicode/utf16"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/richardlehane/mscfb"
)

// Block keys defined by [MS-OFFCRYPTO] for agile encryption verifiers.
var (
	blockVerifierInput = []byte{0xfe, 0xa7, 0xd2, 0x76, 0x3b, 0x4b, 0x9e, 0x79}
	blockVerifierValue = []byte{0xd7, 0xaa, 0x0f, 0x6d, 0x30, 0x61, 0x34, 0x4e}
)

// officeVerifier validates passwords for OLE-encrypted OOXML documents
// (password-protected .docx/.xlsx/.pptx). It supports the two schemes Office
// uses: agile encryption (2010+) and standard encryption (2007).
type officeVerifier struct {
	verify func(password string) bool
}

func newOfficeVerifier(data []byte) (Verifier, error) {
	info, err := readEncryptionInfo(data)
	if err != nil {
		return nil, err
	}
	if len(info) < 8 {
		return nil, apperr.New(apperr.ParseError, "EncryptionInfo 数据异常")
	}
	major := binary.LittleEndian.Uint16(info[0:2])
	minor := binary.LittleEndian.Uint16(info[2:4])

	switch {
	case minor == 4 && major == 4: // agile
		v, err := newAgileVerifier(info[8:])
		if err != nil {
			return nil, err
		}
		return &officeVerifier{verify: v}, nil
	case minor == 2 && (major == 2 || major == 3 || major == 4): // standard
		v, err := newStandardVerifier(info)
		if err != nil {
			return nil, err
		}
		return &officeVerifier{verify: v}, nil
	default:
		return nil, apperr.Newf(apperr.Unsupported, "不支持的 Office 加密版本 %d.%d", major, minor)
	}
}

func (v *officeVerifier) Verify(password string) bool { return v.verify(password) }

// readEncryptionInfo extracts the "EncryptionInfo" stream from the OLE compound
// file. Its presence is what marks a document as password-protected.
func readEncryptionInfo(data []byte) ([]byte, error) {
	doc, err := mscfb.New(bytes.NewReader(data))
	if err != nil {
		return nil, apperr.New(apperr.ParseError, "无法读取 Office 文件：不是受密码保护的 OOXML 文档")
	}
	for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
		if entry.Name == "EncryptionInfo" {
			return io.ReadAll(entry)
		}
	}
	return nil, apperr.New(apperr.Unsupported, "未找到加密信息：该文档可能未设置打开密码")
}

// ---------------------------------------------------------------------------
// Agile encryption
// ---------------------------------------------------------------------------

type agileEncryptedKey struct {
	SpinCount                  int    `xml:"spinCount,attr"`
	SaltSize                   int    `xml:"saltSize,attr"`
	BlockSize                  int    `xml:"blockSize,attr"`
	KeyBits                    int    `xml:"keyBits,attr"`
	HashSize                   int    `xml:"hashSize,attr"`
	CipherAlgorithm            string `xml:"cipherAlgorithm,attr"`
	HashAlgorithm              string `xml:"hashAlgorithm,attr"`
	SaltValue                  string `xml:"saltValue,attr"`
	EncryptedVerifierHashInput string `xml:"encryptedVerifierHashInput,attr"`
	EncryptedVerifierHashValue string `xml:"encryptedVerifierHashValue,attr"`
}

type agileEncryptionXML struct {
	KeyEncryptors struct {
		KeyEncryptor []struct {
			EncryptedKey agileEncryptedKey `xml:"encryptedKey"`
		} `xml:"keyEncryptor"`
	} `xml:"keyEncryptors"`
}

func newAgileVerifier(xmlData []byte) (func(string) bool, error) {
	var doc agileEncryptionXML
	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		return nil, apperr.New(apperr.ParseError, "无法解析 Office 加密描述信息")
	}
	if len(doc.KeyEncryptors.KeyEncryptor) == 0 {
		return nil, apperr.New(apperr.Unsupported, "未找到口令密钥编码器")
	}
	k := doc.KeyEncryptors.KeyEncryptor[0].EncryptedKey

	salt := decodeB64(k.SaltValue)
	encInput := decodeB64(k.EncryptedVerifierHashInput)
	encValue := decodeB64(k.EncryptedVerifierHashValue)
	keyLen := k.KeyBits / 8
	newHash, ok := hashFactory(k.HashAlgorithm)
	if !ok {
		return nil, apperr.Newf(apperr.Unsupported, "不支持的哈希算法 %q", k.HashAlgorithm)
	}
	if keyLen == 0 || len(salt) == 0 || len(encInput) == 0 || len(encValue) == 0 {
		return nil, apperr.New(apperr.ParseError, "Office 加密参数不完整")
	}

	return func(password string) bool {
		// Iterated hash: H = hash(salt || pw); H = hash(LE32(i) || H) * spinCount.
		h := newHash()
		h.Write(salt)
		h.Write(utf16le(password))
		digest := h.Sum(nil)
		var counter [4]byte
		for i := 0; i < k.SpinCount; i++ {
			binary.LittleEndian.PutUint32(counter[:], uint32(i))
			h.Reset()
			h.Write(counter[:])
			h.Write(digest)
			digest = h.Sum(nil)
		}

		keyInput := deriveAgileKey(newHash, digest, blockVerifierInput, keyLen)
		keyValue := deriveAgileKey(newHash, digest, blockVerifierValue, keyLen)

		verifierInput, err := aesCBCDecrypt(keyInput, salt, encInput)
		if err != nil {
			return false
		}
		verifierHash, err := aesCBCDecrypt(keyValue, salt, encValue)
		if err != nil {
			return false
		}

		h.Reset()
		h.Write(verifierInput)
		expected := h.Sum(nil)
		n := k.HashSize
		if n == 0 || n > len(expected) {
			n = len(expected)
		}
		if n > len(verifierHash) {
			return false
		}
		return bytes.Equal(expected[:n], verifierHash[:n])
	}, nil
}

// deriveAgileKey computes hash(digest || blockKey) sized to keyLen (padded with
// 0x36 or truncated as required by [MS-OFFCRYPTO]).
func deriveAgileKey(newHash func() hash.Hash, digest, blockKey []byte, keyLen int) []byte {
	h := newHash()
	h.Write(digest)
	h.Write(blockKey)
	k := h.Sum(nil)
	if len(k) >= keyLen {
		return k[:keyLen]
	}
	out := make([]byte, keyLen)
	copy(out, k)
	for i := len(k); i < keyLen; i++ {
		out[i] = 0x36
	}
	return out
}

// ---------------------------------------------------------------------------
// Standard encryption (Office 2007, AES + SHA-1, ECB)
// ---------------------------------------------------------------------------

func newStandardVerifier(info []byte) (func(string) bool, error) {
	// Layout: [0:8] version+flags, [8:12] header size, then EncryptionHeader,
	// then EncryptionVerifier.
	if len(info) < 12 {
		return nil, apperr.New(apperr.ParseError, "标准加密信息长度异常")
	}
	headerSize := int(binary.LittleEndian.Uint32(info[8:12]))
	headerStart := 12
	if headerStart+headerSize > len(info) {
		return nil, apperr.New(apperr.ParseError, "标准加密头部越界")
	}
	header := info[headerStart : headerStart+headerSize]
	// EncryptionHeader: Flags(4) SizeExtra(4) AlgID(4) AlgIDHash(4) KeySize(4) ...
	if len(header) < 32 {
		return nil, apperr.New(apperr.ParseError, "标准加密头部过短")
	}
	keySizeBits := int(binary.LittleEndian.Uint32(header[20:24]))
	if keySizeBits == 0 {
		keySizeBits = 128
	}
	keyLen := keySizeBits / 8

	verifier := info[headerStart+headerSize:]
	if len(verifier) < 4 {
		return nil, apperr.New(apperr.ParseError, "标准加密校验块缺失")
	}
	saltSize := int(binary.LittleEndian.Uint32(verifier[0:4]))
	p := 4
	if p+saltSize+16+4 > len(verifier) {
		return nil, apperr.New(apperr.ParseError, "标准加密校验块越界")
	}
	salt := verifier[p : p+saltSize]
	p += saltSize
	encVerifier := verifier[p : p+16]
	p += 16
	verifierHashSize := int(binary.LittleEndian.Uint32(verifier[p : p+4]))
	p += 4
	// EncryptedVerifierHash is padded to a multiple of the block size (32 bytes
	// for AES).
	if p > len(verifier) {
		return nil, apperr.New(apperr.ParseError, "标准加密校验哈希越界")
	}
	encVerifierHash := verifier[p:]

	return func(password string) bool {
		key := deriveStandardKey(salt, password, keyLen)
		block, err := aes.NewCipher(key)
		if err != nil {
			return false
		}
		dv := ecbDecrypt(block, encVerifier)
		dh := ecbDecrypt(block, encVerifierHash)
		sum := sha1.Sum(dv)
		n := verifierHashSize
		if n == 0 || n > len(sum) {
			n = len(sum)
		}
		if n > len(dh) {
			return false
		}
		return bytes.Equal(sum[:n], dh[:n])
	}, nil
}

func deriveStandardKey(salt []byte, password string, keyLen int) []byte {
	h := sha1.New()
	h.Write(salt)
	h.Write(utf16le(password))
	digest := h.Sum(nil)
	var counter [4]byte
	for i := 0; i < 50000; i++ {
		binary.LittleEndian.PutUint32(counter[:], uint32(i))
		h.Reset()
		h.Write(counter[:])
		h.Write(digest)
		digest = h.Sum(nil)
	}
	// Final block 0.
	h.Reset()
	binary.LittleEndian.PutUint32(counter[:], 0)
	h.Write(digest)
	h.Write(counter[:])
	digest = h.Sum(nil)

	// Derive key via the X1/X2 expansion.
	x1in := make([]byte, 64)
	x2in := make([]byte, 64)
	for i := 0; i < 64; i++ {
		var b byte
		if i < len(digest) {
			b = digest[i]
		}
		x1in[i] = 0x36 ^ b
		x2in[i] = 0x5c ^ b
	}
	x1 := sha1.Sum(x1in)
	x2 := sha1.Sum(x2in)
	key := append(x1[:], x2[:]...)
	if keyLen > len(key) {
		keyLen = len(key)
	}
	return key[:keyLen]
}

// ---------------------------------------------------------------------------
// Crypto helpers
// ---------------------------------------------------------------------------

func aesCBCDecrypt(key, iv, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	if len(data) == 0 || len(data)%bs != 0 {
		return nil, apperr.New(apperr.ParseError, "密文长度非法")
	}
	realIV := make([]byte, bs)
	copy(realIV, iv) // salt is truncated/zero-padded to the block size
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, realIV).CryptBlocks(out, data)
	return out, nil
}

func ecbDecrypt(block cipher.Block, data []byte) []byte {
	bs := block.BlockSize()
	out := make([]byte, len(data)/bs*bs)
	for i := 0; i+bs <= len(data); i += bs {
		block.Decrypt(out[i:i+bs], data[i:i+bs])
	}
	return out
}

func hashFactory(algo string) (func() hash.Hash, bool) {
	switch algo {
	case "SHA512", "SHA-512":
		return sha512.New, true
	case "SHA384", "SHA-384":
		return sha512.New384, true
	case "SHA256", "SHA-256":
		return sha256.New, true
	case "SHA1", "SHA-1":
		return sha1.New, true
	default:
		return nil, false
	}
}

func utf16le(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2)
	for i, r := range u {
		binary.LittleEndian.PutUint16(b[i*2:], r)
	}
	return b
}

func decodeB64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}
