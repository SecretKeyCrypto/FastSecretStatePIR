package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
)

type Encryptor struct {
	cipher cipher.Block
	q      int
}

func NewEncryptor(key []byte, q int) (*Encryptor, error) {

	blockcipher, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	return &Encryptor{
		cipher: blockcipher,
		q:      q,
	}, nil
}

func generatePlaintextFromCoordinate(i, j int16) []byte {
	plaintext := make([]byte, aes.BlockSize)
	binary.LittleEndian.PutUint64(plaintext[0:8], uint64(i))
	binary.LittleEndian.PutUint64(plaintext[8:16], uint64(j))
	return plaintext
}

// encryptBlock encrypts a single block of plaintext using AES in ECB mode.
func (e *Encryptor) Encrypt(ii, jj, data int) int {

	return (e.EncryptPosition(ii, jj) + data) % e.q
}

func (e *Encryptor) EncryptPosition(ii, jj int) int {
	i := int16(ii)
	j := int16(jj)

	plaintext := generatePlaintextFromCoordinate(i, j)

	if len(plaintext)%aes.BlockSize != 0 {
		panic("Plaintext length for aes encryption not valid!")
	}

	ciphertext := make([]byte, len(plaintext))
	for start := 0; start < len(plaintext); start += aes.BlockSize {
		end := start + aes.BlockSize
		e.cipher.Encrypt(ciphertext[start:end], plaintext[start:end])
	}

	// bigCipher := new(big.Int).SetBytes(ciphertext)
	// re := new(big.Int).Mod(bigCipher, big.NewInt(int64(e.q)))
	// return (int(re.Int64())) % e.q

	shortCipher := ciphertext[:8]

	// Convert the first 8 bytes to an int64
	cipherInt := int(binary.BigEndian.Uint64(shortCipher))

	// Perform the modulus operation
	return cipherInt % e.q
}

func (e *Encryptor) Decrypt(ii, jj, data int) int {
	return (-e.EncryptPosition(ii, jj) + data + e.q) % e.q
}
