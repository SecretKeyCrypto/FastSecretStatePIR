package utils

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/capitalone/fpe/ff1"
)

const (
	feistelMin  = 100
	charsetBase = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

type Permutator struct {
	Cipher ff1.Cipher
	maxNum int
	minNum int
	radix  int
	minLen int
	maxLen int
}

func NewPermutator(numRange int, key, tweak []byte) (*Permutator, error) {
	radix, maxLen, _ := findBestRadixAndLength(numRange)
	if len(tweak) > maxLen {
		tweak = tweak[:maxLen]
	}
	minLen := minLenByRadix(radix)
	minNum := int(math.Pow(float64(radix), float64(minLen-1)))
	FF1, err := ff1.NewCipher(radix, maxLen, key, tweak)

	if err != nil {
		return nil, err
	}

	return &Permutator{
		Cipher: FF1,
		maxNum: numRange + minNum,
		minNum: minNum,
		radix:  radix,
		minLen: minLen,
		maxLen: maxLen,
	}, nil
}

func NewPermutatorWithParameters(maxNum, minNum, maxLen, minLen, radix int, key, tweak []byte) (*Permutator, error) {
	if len(tweak) > maxLen {
		tweak = tweak[:maxLen]
	}

	FF1, err := ff1.NewCipher(radix, maxLen, key, tweak)

	if err != nil {
		return nil, err
	}

	return &Permutator{
		Cipher: FF1,
		maxNum: maxNum,
		minNum: minNum,
		radix:  radix,
		minLen: minLen,
		maxLen: maxLen,
	}, nil
}

func (p *Permutator) WithinNumRange(num int) bool {
	return num >= p.minNum && num < p.maxNum
}

func (p *Permutator) EncryptMap(plaintextInt int) (int, error) {
	plaintext := p.IntToBaseRString(uint64(plaintextInt + p.minNum))
	for {
		ciphertext, err := p.Cipher.Encrypt(plaintext)
		if err != nil {
			return -1, err
		}
		ciphertextInt, err := p.BaseRStringToInt(ciphertext)
		if p.WithinNumRange(ciphertextInt) {
			return ciphertextInt - p.minNum, err
		}
		plaintext = ciphertext
	}
}

func (p *Permutator) DecryptMap(ciphertextInt int) (int, error) {
	ciphertext := p.IntToBaseRString(uint64(ciphertextInt + p.minNum))
	for {
		plaintext, err := p.Cipher.Decrypt(ciphertext)
		if err != nil {
			return -1, err
		}
		plaintextInt, err := p.BaseRStringToInt(plaintext)
		if p.WithinNumRange(plaintextInt) {
			return plaintextInt - p.minNum, err
		}
		ciphertext = plaintext
	}
}

func (p *Permutator) IntToBaseRString(u uint64) string {
	if p.radix == 10 || (isPowerOfTwo(p.radix) && p.radix <= 36) {
		baseRString := strconv.FormatUint(u, p.radix)
		numZerosNeeded := p.maxLen - len(baseRString)
		if numZerosNeeded > 0 {
			padding := strings.Repeat("0", numZerosNeeded)
			return padding + baseRString
		}
		return baseRString
	} else {
		a := make([]byte, p.maxLen)
		i := len(a)
		b := uint64(p.radix)
		for u >= b {
			i--
			q := u / b
			a[i] = charsetBase[uint(u-q*b)]
			u = q
		}
		// u < base
		i--
		a[i] = charsetBase[uint(u)]
		for i > 0 {
			i--
			a[i] = '0'
		}

		return string(a)
	}
}

func charsetForRadix(radix int) (charset string) {
	return charsetBase[:radix]
}

func (p *Permutator) BaseRStringToInt(s string) (int, error) {
	charset := charsetForRadix(p.radix)
	var num int
	for _, char := range s {
		val := strings.IndexRune(charset, char)
		if val == -1 {
			return 0, fmt.Errorf("invalid character: %v", string(char))
		}
		num = num*int(p.radix) + int(val)
	}

	return num, nil
}

func findBestRadixAndLength(maxNum int) (bestRadix int, bestLength int, minDelta int) {
	minDelta = math.MaxInt

	for radix := 2; radix <= 62; radix++ {
		minLen := minLenByRadix(radix)
		logValue := math.Log(float64(maxNum)+math.Pow(float64(radix), float64(minLen))) / math.Log(float64(radix))
		length := int(math.Ceil(logValue))

		if length < math.MaxUint32 {
			delta := int(math.Pow(float64(radix), float64(length))) - maxNum - int(math.Pow(float64(radix), float64(minLen)))
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

func isPowerOfTwo(x int) bool {
	return x&(x-1) == 0
}

func (p *Permutator) ReturnParameters() (int, int, int, int, int) {
	return p.maxNum, p.minNum, p.maxLen, p.minLen, p.radix
}
