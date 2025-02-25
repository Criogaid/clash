package vmess

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"hash/crc32"
	"time"

	"github.com/Dreamacro/clash/common/pool"
)

const (
	kdfSaltConstAuthIDEncryptionKey             = "AES Auth ID Encryption"
	kdfSaltConstAEADRespHeaderLenKey            = "AEAD Resp Header Len Key"
	kdfSaltConstAEADRespHeaderLenIV             = "AEAD Resp Header Len IV"
	kdfSaltConstAEADRespHeaderPayloadKey        = "AEAD Resp Header Key"
	kdfSaltConstAEADRespHeaderPayloadIV         = "AEAD Resp Header IV"
	kdfSaltConstVMessAEADKDF                    = "VMess AEAD KDF"
	kdfSaltConstVMessHeaderPayloadAEADKey       = "VMess Header AEAD Key"
	kdfSaltConstVMessHeaderPayloadAEADIV        = "VMess Header AEAD Nonce"
	kdfSaltConstVMessHeaderPayloadLengthAEADKey = "VMess Header AEAD Key_Length"
	kdfSaltConstVMessHeaderPayloadLengthAEADIV  = "VMess Header AEAD Nonce_Length"
)

func kdf(key []byte, path ...string) []byte {
	hmacCreator := &hMacCreator{value: []byte(kdfSaltConstVMessAEADKDF)}
	for _, v := range path {
		hmacCreator = &hMacCreator{value: []byte(v), parent: hmacCreator}
	}
	hmacf := hmacCreator.Create()
	hmacf.Write(key)
	return hmacf.Sum(nil)
}

type hMacCreator struct {
	parent *hMacCreator
	value  []byte
}

func (h *hMacCreator) Create() hash.Hash {
	if h.parent == nil {
		return hmac.New(sha256.New, h.value)
	}
	return hmac.New(h.parent.Create, h.value)
}

func createAuthID(cmdKey []byte, time int64) [16]byte {
	buf := pool.BufferWriter{}
	buf.PutUint64be(uint64(time))
	_ = buf.ReadFull(rand.Reader, 4)
	zero := crc32.ChecksumIEEE(buf.Bytes())
	buf.PutUint32be(zero)

	aesBlock, _ := aes.NewCipher(kdf(cmdKey[:], kdfSaltConstAuthIDEncryptionKey)[:16])
	var result [16]byte
	aesBlock.Encrypt(result[:], buf.Bytes())
	return result
}

func sealVMessAEADHeader(key [16]byte, data []byte, t time.Time) []byte {
	generatedAuthID := createAuthID(key[:], t.Unix())
	connectionNonce := make([]byte, 8)
	_, _ = rand.Read(connectionNonce)

	aeadPayloadLengthSerializedByte := make([]byte, 2)
	binary.BigEndian.PutUint16(aeadPayloadLengthSerializedByte, uint16(len(data)))

	var payloadHeaderLengthAEADEncrypted []byte

	{
		payloadHeaderLengthAEADKey := kdf(key[:], kdfSaltConstVMessHeaderPayloadLengthAEADKey, string(generatedAuthID[:]), string(connectionNonce))[:16]
		payloadHeaderLengthAEADNonce := kdf(key[:], kdfSaltConstVMessHeaderPayloadLengthAEADIV, string(generatedAuthID[:]), string(connectionNonce))[:12]
		payloadHeaderLengthAEADAESBlock, _ := aes.NewCipher(payloadHeaderLengthAEADKey)
		payloadHeaderAEAD, _ := cipher.NewGCM(payloadHeaderLengthAEADAESBlock)
		payloadHeaderLengthAEADEncrypted = payloadHeaderAEAD.Seal(nil, payloadHeaderLengthAEADNonce, aeadPayloadLengthSerializedByte, generatedAuthID[:])
	}

	var payloadHeaderAEADEncrypted []byte

	{
		payloadHeaderAEADKey := kdf(key[:], kdfSaltConstVMessHeaderPayloadAEADKey, string(generatedAuthID[:]), string(connectionNonce))[:16]
		payloadHeaderAEADNonce := kdf(key[:], kdfSaltConstVMessHeaderPayloadAEADIV, string(generatedAuthID[:]), string(connectionNonce))[:12]
		payloadHeaderAEADAESBlock, _ := aes.NewCipher(payloadHeaderAEADKey)
		payloadHeaderAEAD, _ := cipher.NewGCM(payloadHeaderAEADAESBlock)
		payloadHeaderAEADEncrypted = payloadHeaderAEAD.Seal(nil, payloadHeaderAEADNonce, data, generatedAuthID[:])
	}

	outputBuffer := pool.BufferWriter{}

	outputBuffer.PutSlice(generatedAuthID[:])
	outputBuffer.PutSlice(payloadHeaderLengthAEADEncrypted)
	outputBuffer.PutSlice(connectionNonce)
	outputBuffer.PutSlice(payloadHeaderAEADEncrypted)

	return outputBuffer.Bytes()
}
