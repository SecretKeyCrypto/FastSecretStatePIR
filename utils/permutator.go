package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"math"
)

const (
	feistelMin    = 100
	blockSize     = aes.BlockSize
	numRounds     = 1
	halfBlockSize = blockSize / 2
)

var (
	// For all AES-CBC calls, IV is always 0
	ivZero = make([]byte, blockSize)
)

type Permutator struct {
	tweak  []byte
	maxNum int
	radix  int
	maxLen int
	// Re-usable CBC encryptor with exported SetIV function
	cbcEncryptor cipher.BlockMode
	P            []byte
	PQ           []byte
	R            []byte
	buf64        []byte
	b            int
	d            int
	lenQ         int
	lenPQ        int
	numPad       int
	numModU      uint64
	numModV      uint64
}

// Need this for the SetIV function which CBCEncryptor has, but cipher.BlockMode interface doesn't.
type cbcMode interface {
	cipher.BlockMode
	SetIV([]byte)
}

func NewPermutator(maxNum int, key, tweak []byte) (Permutator, error) {
	radix, maxLen, _ := findBestRadixAndLength(maxNum)
	// 	return NewPermutatorWithParameters(numRange, maxLen, radix, key, tweak)
	// }

	// func NewPermutatorWithParameters(maxNum, maxLen, radix int, key, tweak []byte) (Permutator, error) {
	var newPermutator Permutator

	if len(tweak) > maxLen {
		tweak = tweak[:maxLen]
	}

	aesBlock, _ := aes.NewCipher(key)

	cbcEncryptor := cipher.NewCBCEncrypter(aesBlock, ivZero)

	u := maxLen / 2
	v := maxLen - u
	t := len(tweak)

	// Byte lengths
	b := int(math.Ceil(math.Ceil(float64(v)*math.Log2(float64(radix))) / 8))
	d := int(4*math.Ceil(float64(b)/4) + 4)

	numPad := (-t - b - 1) % 16
	if numPad < 0 {
		numPad += 16
	}

	// Calculate P, doesn't change in each loop iteration
	// P's length is always 16, so it can stay on the stack
	P := make([]byte, blockSize)

	P[0] = 0x01
	P[1] = 0x02
	P[2] = 0x01
	P[3] = 0x00
	binary.BigEndian.PutUint16(P[4:6], uint16(radix))
	P[6] = 0x0a
	P[7] = byte(maxLen / 2) // overflow automatically does the modulus
	binary.BigEndian.PutUint32(P[8:12], uint32(maxLen))
	binary.BigEndian.PutUint32(P[12:blockSize], uint32(len(tweak)))

	lenQ := t + b + 1 + numPad

	newPermutator.maxNum = maxNum
	newPermutator.radix = radix
	newPermutator.maxLen = maxLen
	newPermutator.cbcEncryptor = cbcEncryptor
	newPermutator.tweak = tweak
	newPermutator.b = b
	newPermutator.d = d
	newPermutator.lenQ = lenQ
	newPermutator.lenPQ = blockSize + lenQ
	newPermutator.numPad = numPad
	newPermutator.P = P
	newPermutator.PQ = make([]byte, newPermutator.lenPQ)
	newPermutator.R = make([]byte, newPermutator.lenPQ)
	newPermutator.buf64 = make([]byte, 8)
	newPermutator.numModU = uint64(math.Pow(float64(radix), float64(u)))
	newPermutator.numModV = uint64(math.Pow(float64(radix), float64(v)))

	return newPermutator, nil
}

func (p Permutator) Permute(X uint64) (uint64, error) {
	var ret uint64
	for {
		ret = p.Encrypt(X)
		if ret < uint64(p.maxNum) {
			return ret, nil
		}
		X = ret
	}
}

func (p Permutator) Encrypt(X uint64) uint64 {
	t := len(p.tweak)

	PQ := p.PQ
	R := p.R

	Q := PQ[blockSize:]
	copy(Q[:t], p.tweak)
	copy(PQ[:blockSize], p.P)

	var numA, numB, numC, numY uint64

	numUint64Bytes := p.buf64

	Y := R[p.lenPQ-blockSize:]

	numA = X / p.numModV
	numB = X % p.numModV

	// Main Feistel Round, 10 times
	for i := 0; i < numRounds; i++ {
		Q[t+p.numPad] = byte(i)

		binary.BigEndian.PutUint64(numUint64Bytes, numB)

		copy(Q[p.lenQ-p.b:], numUint64Bytes[8-p.b:])

		p.prf(PQ, R)

		numY = binary.BigEndian.Uint64(Y[:p.d])
		numC = numA + numY

		if i%2 == 0 {
			numC %= p.numModU
		} else {
			numC %= p.numModV
		}

		numA = numB
		numB = numC
	}
	return numA*p.numModV + numB
}

func findBestRadixAndLength(maxNum int) (bestRadix int, bestLength int, minDelta int) {
	minDelta = math.MaxInt

	for radix := 2; radix <= 62; radix++ {
		minLen := minLenByRadix(radix)
		logValue := math.Log(float64(maxNum)) / math.Log(float64(radix))
		length := int(math.Max(float64(minLen), (math.Ceil(logValue))))

		if length < math.MaxUint32 {
			delta := int(math.Pow(float64(radix), float64(length))) - maxNum
			if delta < minDelta {
				minDelta = delta
				bestRadix = radix
				bestLength = length
			}
		}
	}

	return
}

func minLenByRadix(radix int) (minLen int) {
	minLen = int(math.Ceil(math.Log(feistelMin) / math.Log(float64(radix))))
	return
}

func (p Permutator) ReturnParameters() (int, int, int) {
	return p.maxNum, p.maxLen, p.radix
}

func (p Permutator) Revert(X uint64) (uint64, error) {
	var ret uint64
	for {
		ret = p.Decrypt(X)
		if ret < uint64(p.maxNum) {
			return ret, nil
		}
		X = ret
	}
}

func (p Permutator) Decrypt(X uint64) uint64 {
	PQ := p.PQ
	buf_R := p.R
	Q := PQ[blockSize:]
	t := len(p.tweak)

	copy(Q[:t], p.tweak)
	copy(PQ[:blockSize], p.P)

	var numA, numB, numC, numY uint64

	numUint64Bytes := p.buf64

	Y := buf_R[p.lenPQ-blockSize:]

	numA = X / p.numModV
	numB = X % p.numModV

	// Main Feistel Round, 10 times
	for i := numRounds - 1; i >= 0; i-- {
		Q[t+p.numPad] = byte(i)
		binary.BigEndian.PutUint64(numUint64Bytes, numA)

		copy(Q[p.lenQ-p.b:], numUint64Bytes[8-p.b:])

		p.prf(PQ, buf_R)

		numY = binary.BigEndian.Uint64(Y[:p.d])

		numC = numY - numB

		if i%2 == 0 {
			numC %= p.numModU
			numC = (p.numModU - numC) % p.numModU
		} else {
			numC %= p.numModV
			numC = (p.numModV - numC) % p.numModV
		}

		numB = numA
		numA = numC
	}

	return numA*p.numModV + numB
}

// PRF as defined in the NIST spec is actually just AES-CBC-MAC, which is the last block of an AES-CBC encrypted ciphertext. Utilize the ciph function for the AES-CBC.
func (p Permutator) prf(input []byte, output []byte) {
	p.cbcEncryptor.CryptBlocks(output, input)
	// Reset IV to 0
	p.cbcEncryptor.(cbcMode).SetIV(ivZero)
}
