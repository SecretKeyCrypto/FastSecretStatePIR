package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"math"
)

type Encryptor struct {
	cipher  cipher.Block
	q       uint64
	maxModq uint64
}

func NewEncryptor(key []byte, q int) (Encryptor, error) {
	var encryptor Encryptor
	if q < 1 {
		return encryptor, fmt.Errorf("mask modulus must be positive, got %d", q)
	}

	blockcipher, err := aes.NewCipher(key)

	if err != nil {
		return encryptor, err
	}

	encryptor.cipher = blockcipher
	encryptor.q = uint64(q)
	encryptor.maxModq = (math.MaxUint64%encryptor.q + 1) % encryptor.q
	return encryptor, nil
}

func generatePlaintextFromIndex(slice, index int, plaintext []byte) []byte {
	binary.LittleEndian.PutUint64(plaintext[0:8], uint64(index))
	binary.LittleEndian.PutUint64(plaintext[8:16], uint64(slice))
	return plaintext
}

// encryptBlock encrypts a single block of plaintext using AES in ECB mode.
func (e Encryptor) Encrypt(index, data int) int {
	return (e.EncryptPosition(0, index) + (data)) % int(e.q)
}

func (e Encryptor) EncryptSlice(slice, index, data int) int {
	return (e.EncryptPosition(slice, index) + (data)) % int(e.q)
}

func (e Encryptor) EncryptPosition(slice, index int) int {
	var plaintext [aes.BlockSize]byte
	var ciphertext [aes.BlockSize]byte
	generatePlaintextFromIndex(slice, index, plaintext[:])

	e.cipher.Encrypt(ciphertext[:], plaintext[:])

	A1 := binary.BigEndian.Uint64(ciphertext[:8])
	A2 := binary.BigEndian.Uint64(ciphertext[8:])

	// For demonstration, let's perform a modulus operation on A1 and A2 with q
	q := uint64(e.q) // Example modulus

	// Calculate (A1 * 2^64 + A2) % q
	result := ((A1%q)*e.maxModq + A2%q) % q

	return int(result)
}

func (e Encryptor) Decrypt(index, data int) int {
	return (-e.EncryptPosition(0, index) + (data) + int(e.q)) % int(e.q)
}

func (e Encryptor) DecryptSlice(slice, index, data int) int {
	return (-e.EncryptPosition(slice, index) + (data) + int(e.q)) % int(e.q)
}
