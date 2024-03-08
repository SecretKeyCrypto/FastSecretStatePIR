package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
)

// func NewEncryptor(radix, maxLen int, key, tweak []byte) (ff1.Cipher, error) {
// 	return ff1.NewCipher(radix, maxLen, key, tweak)
// }

// func Decrypt(encryptor ff1.Cipher, data, maxLen int) int {
// 	bit_q := maxLen / 2
// 	d := fmt.Sprintf("%0*s", maxLen, strconv.FormatInt(int64(data), 2))

// 	dec, err := encryptor.Decrypt(d)
// 	if err != nil {
// 		panic(err)
// 	}

// 	number, _ := strconv.ParseInt(dec[bit_q:], 2, 32)

// 	return int(number)
// }
// func Encrypt(encryptor ff1.Cipher, row, col, data, bit_q, q int) int {
// 	randc := fmt.Sprintf("%0*s", bit_q, strconv.FormatInt(int64((row+col)/2+1), 2))
// 	d := fmt.Sprintf("%0*s", bit_q, strconv.FormatInt(int64(data), 2))
// 	conc := randc + d

// 	enc, err := encryptor.Encrypt(conc)
// 	if err != nil {
// 		panic(err)
// 	}

// 	number, _ := strconv.ParseInt(enc, 2, 32)

// 	return int(number) + q
// }

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

	return int(binary.BigEndian.Uint16(ciphertext[0:2])) ^ data
}

func (e *Encryptor) Decrypt(i, j, data int) int {
	return e.Encrypt(i, j, data)
}

func TestMain(encryptor Encryptor, i, j, data int) {

	v := encryptor.Encrypt(i, j, data)
	fmt.Println(v)

	a := encryptor.Decrypt(i, j, v)
	fmt.Println(a)
}
