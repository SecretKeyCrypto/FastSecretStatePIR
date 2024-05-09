package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"math"
)

type Encryptor struct {
	cipher  cipher.Block
	q       uint64
	maxModq uint64
	plaintext []byte
	ciphertext []byte
}

func NewEncryptor(key []byte, q int) (Encryptor, error) {
	var encryptor Encryptor

	blockcipher, err := aes.NewCipher(key)

	if err != nil {
		return encryptor, err
	}

	encryptor.cipher = blockcipher
	encryptor.q = uint64(q)
	encryptor.maxModq = (math.MaxUint64%encryptor.q + 1) % encryptor.q
	encryptor.plaintext = make([]byte, aes.BlockSize)
	encryptor.ciphertext = make([]byte, aes.BlockSize)

	return encryptor, nil
}

func generatePlaintextFromCoordinate(row, col int, plaintext []byte) []byte {
	binary.LittleEndian.PutUint64(plaintext[0:8], uint64(row))
	binary.LittleEndian.PutUint64(plaintext[8:16], uint64(col))
	return plaintext
}

// encryptBlock encrypts a single block of plaintext using AES in ECB mode.
func (e Encryptor) Encrypt(row, col, data int) int {

	return (e.EncryptPosition(row, col) + (data)) % int(e.q)
}

func (e Encryptor) EncryptPosition(row, col int) int {
	generatePlaintextFromCoordinate(row, col, e.plaintext)

	e.cipher.Encrypt(e.ciphertext, e.plaintext)

	A1 := binary.BigEndian.Uint64(e.ciphertext[:8])
	A2 := binary.BigEndian.Uint64(e.ciphertext[8:])

	// For demonstration, let's perform a modulus operation on A1 and A2 with q
	q := uint64(e.q) // Example modulus

	// Calculate (A1 * 2^64 + A2) % q
	result := ((A1%q)*e.maxModq + A2%q) % q

	return int(result)
}

func (e Encryptor) Decrypt(row, col, data int) int {
	return (-e.EncryptPosition(row, col) + (data) + int(e.q)) % int(e.q)
}
