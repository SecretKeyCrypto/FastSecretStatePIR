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

func (p *Permutator) WithinNumRange(myNum string) (bool, error) {
	num, err := p.BaseRStringToInt(myNum)
	if err != nil {
		return false, err
	}

	return num >= p.minNum && num < p.maxNum, nil
}

func (p *Permutator) EncryptMap(plaintextInt int) (int, error) {
	plaintext, _ := p.IntToBaseRString(int64(plaintextInt + p.minNum))
	for {
		ciphertext, err := p.Cipher.Encrypt(plaintext)
		if err != nil {
			return -1, err
		}
		passedMaxNumTest, _ := p.WithinNumRange(ciphertext)
		if passedMaxNumTest {
			ciphertextInt, err := p.BaseRStringToInt(ciphertext)
			return ciphertextInt - p.minNum, err
		}
		plaintext = ciphertext
	}
}

func (p *Permutator) DecryptMap(ciphertextInt int) (int, error) {
	ciphertext, _ := p.IntToBaseRString(int64(ciphertextInt + p.minNum))
	for {
		plaintext, err := p.Cipher.Decrypt(ciphertext)
		if err != nil {
			return -1, err
		}
		passedMaxNumTest, _ := p.WithinNumRange(plaintext)
		if passedMaxNumTest {
			plaintextInt, err := p.BaseRStringToInt(plaintext)
			return plaintextInt - p.minNum, err
		}
		ciphertext = plaintext
	}
}

func (p *Permutator) IntToBaseRString(number int64) (string, error) {
	var baseRString string
	if p.radix <= 36 {
		baseRString = strconv.FormatInt(number, p.radix)
	} else {
		var a [64 + 1]byte // +1 for sign of 64bit value in base 2
		i := len(a)

		b := uint64(p.radix)
		u := uint64(number)
		for u >= b {
			i--
			q := u / b
			a[i] = charsetBase[uint(u-q*b)]
			u = q
		}
		// u < base
		i--
		a[i] = charsetBase[uint(u)]
		baseRString = string(a[i:])
	}
	paddedString := fmt.Sprintf("%0*s", p.maxLen, baseRString)
	return paddedString, nil
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

func (p *Permutator) ReturnParameters() (int, int, int, int, int) {
	return p.maxNum, p.minNum, p.maxLen, p.minLen, p.radix
}
