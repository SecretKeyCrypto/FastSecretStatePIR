package utils

import (
	"fmt"
	"math/rand"
)

// GF2n represents the field GF(2^n) with log/exp table arithmetic.
// Addition = XOR. Multiplication via discrete-log tables (O(1) per op).
// Supported degrees n: 4, 8, 12, 13, 14, 16, 17, 18, 20.
type GF2n struct {
	N   int      // extension degree
	Q   uint32   // field size = 1 << N
	exp []uint32 // exp[i] = α^i; duplicated (2*(Q-1) entries) to avoid mod in Mul
	log []uint32 // log[x] = i such that α^i = x; log[0] is unused (leave as 0)
}

// Primitive polynomials over GF(2) (full bit representation including leading x^n term).
// All are verified primitive: α = x generates all 2^n − 1 non-zero elements.
var gf2nPrimPolys = map[int]uint32{
	4:  0x13,     // x^4 + x + 1
	8:  0x11D,    // x^8 + x^4 + x^3 + x^2 + 1
	12: 0x1053,   // x^12 + x^6 + x^4 + x + 1
	13: 0x201B,   // x^13 + x^4 + x^3 + x + 1
	14: 0x402B,   // x^14 + x^5 + x^3 + x + 1
	16: 0x1002D,  // x^16 + x^5 + x^3 + x^2 + 1
	17: 0x20009,  // x^17 + x^3 + 1
	18: 0x40081,  // x^18 + x^7 + 1
	20: 0x100009, // x^20 + x^3 + 1
}

// NewGF2n builds GF(2^n) log/exp tables in O(2^n) time and space.
// Panics if n is not in the supported set or the polynomial is non-primitive.
func NewGF2n(n int) *GF2n {
	irred, ok := gf2nPrimPolys[n]
	if !ok {
		panic(fmt.Sprintf("GF2n: unsupported degree %d", n))
	}
	gf := &GF2n{N: n, Q: 1 << uint(n)}
	q := int(gf.Q)

	gf.exp = make([]uint32, 2*(q-1))
	gf.log = make([]uint32, q)

	x := uint32(1) // current power: α^0 = 1
	for i := 0; i < q-1; i++ {
		gf.exp[i] = x
		gf.exp[i+q-1] = x // duplicate: Mul uses log[a]+log[b] ∈ [0, 2(q-2)], no mod needed
		gf.log[x] = uint32(i)
		// Multiply x by α (the polynomial "x" = bit-1):
		// shift left, then XOR with the irreducible poly if the degree-n bit overflowed.
		x <<= 1
		if x >= gf.Q {
			x ^= irred
		}
	}

	// Sanity: α^(q-1) must wrap back to 1.
	if x != 1 {
		panic(fmt.Sprintf("GF2n(%d): polynomial 0x%X gives α^(q-1)≠1", n, irred))
	}
	// Stronger primitivity check: for a non-primitive poly of order d | (q-1), d < q-1,
	// the loop overwrites log[1] each time α^i=1 (at i=d, 2d, ...). So log[1]≠0 after
	// the loop. A primitive poly writes log[1]=0 exactly once (at i=0) and never again.
	if gf.log[1] != 0 {
		panic(fmt.Sprintf("GF2n(%d): polynomial 0x%X is not primitive (order < q-1)", n, irred))
	}
	return gf
}

// Add returns a + b in GF(2^n).
func (g *GF2n) Add(a, b uint32) uint32 { return a ^ b }

// Mul returns a · b in GF(2^n) via log/exp tables.
func (g *GF2n) Mul(a, b uint32) uint32 {
	if a == 0 || b == 0 {
		return 0
	}
	// g.log[a] + g.log[b] ≤ 2(Q−2) < 2(Q−1) = len(g.exp), so no bounds check needed.
	return g.exp[g.log[a]+g.log[b]]
}

// Pow returns a^e in GF(2^n), taking e mod (Q−1).
func (g *GF2n) Pow(a uint32, e uint64) uint32 {
	if a == 0 {
		return 0
	}
	logR := uint32(uint64(g.log[a]) * e % uint64(g.Q-1))
	return g.exp[logR]
}

// Inv returns 1/a in GF(2^n). Panics if a == 0.
func (g *GF2n) Inv(a uint32) uint32 {
	if a == 0 {
		panic("GF2n.Inv: zero has no inverse")
	}
	return g.exp[g.Q-1-g.log[a]]
}

// EvalPoly evaluates f(x) = coeffs[0] + coeffs[1]·x + ··· using Horner's method.
func (g *GF2n) EvalPoly(coeffs []uint32, x uint32) uint32 {
	if len(coeffs) == 0 {
		return 0
	}
	r := coeffs[len(coeffs)-1]
	for k := len(coeffs) - 2; k >= 0; k-- {
		r = g.Add(g.Mul(r, x), coeffs[k])
	}
	return r
}

// RandPolyWithValue returns a random degree-k polynomial over GF(2^n) such that f(t) = a.
// Uses the global math/rand source; not safe for concurrent use without external locking.
func (g *GF2n) RandPolyWithValue(degree int, t, a uint32) []uint32 {
	q := int(g.Q)
	coeffs := make([]uint32, degree+1)
	for i := range coeffs {
		coeffs[i] = uint32(rand.Intn(q))
	}
	// XOR-adjust c[0] so f(t) = a.
	// In GF(2^n) addition = subtraction = XOR, so: c[0] ^= f(t) ^ a.
	coeffs[0] ^= g.EvalPoly(coeffs, t) ^ a
	return coeffs
}
