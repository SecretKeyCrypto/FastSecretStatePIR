package utils

import "math/rand"

// GFExt2Elem is an element a + b·β of GF(q²) = GF(q)[β]/(β²+β+C).
type GFExt2Elem struct{ A, B uint32 }

// GF2nExt2 represents GF(q²) = GF(q)[β]/(β²+β+C) where Tr_{GF(q)/GF(2)}(C) = 1.
// Elements are (a, b) representing a + b·β, with β² = β + C.
type GF2nExt2 struct {
	Base           *GF2n
	C              uint32         // irreducible constant; β² = β + C
	BetaInvWeights []GFExt2Elem   // BetaInvWeights[w] = (β+w)⁻¹ for w ∈ Fq; used by PhiInv
}

// NewGF2nExt2 constructs GF(q²) as a degree-2 extension of GF(q).
// Finds the first C ∈ GF(q)* with absolute trace Tr(C)=1 so β²+β+C is irreducible.
// Also precomputes BetaInvWeights[w] = (β+w)⁻¹ for w = 0..q-1.
func NewGF2nExt2(base *GF2n) *GF2nExt2 {
	var C uint32
	for x := uint32(1); x < base.Q; x++ {
		if absTrace(base, x) == 1 {
			C = x
			break
		}
	}
	if C == 0 {
		panic("GF2nExt2: no element with absolute trace 1 found")
	}
	g := &GF2nExt2{Base: base, C: C}

	// Precompute (β+w)⁻¹ for all w ∈ Fq.
	// β+w = GFExt2Elem{w, 1}; β ∉ Fq so β+w ≠ 0 for all w.
	// These weights support O(q) φ⁻¹ recovery: PhiInv(y) = Σ_w y[w]·(β+w)⁻¹.
	g.BetaInvWeights = make([]GFExt2Elem, base.Q)
	for w := uint32(0); w < base.Q; w++ {
		g.BetaInvWeights[w] = g.Inv(GFExt2Elem{w, 1})
	}
	return g
}

// absTrace computes Tr_{GF(2^n)/GF(2)}(x) = x ⊕ x² ⊕ x⁴ ⊕ … ⊕ x^(2^(n-1)) mod 2.
func absTrace(gf *GF2n, x uint32) uint32 {
	t, cur := x, x
	for i := 1; i < gf.N; i++ {
		cur = gf.Mul(cur, cur)
		t ^= cur
	}
	return t & 1
}

func (g *GF2nExt2) Zero() GFExt2Elem { return GFExt2Elem{} }
func (g *GF2nExt2) One() GFExt2Elem  { return GFExt2Elem{1, 0} }

func (g *GF2nExt2) Add(a, b GFExt2Elem) GFExt2Elem {
	return GFExt2Elem{a.A ^ b.A, a.B ^ b.B}
}

// Mul computes (a0+a1β)(b0+b1β) using β² = β+C.
// Result: A = a0b0 ⊕ a1b1·C,  B = a0b1 ⊕ a1b0 ⊕ a1b1.
func (g *GF2nExt2) Mul(a, b GFExt2Elem) GFExt2Elem {
	gf := g.Base
	t0 := gf.Mul(a.A, b.A)
	t1 := gf.Mul(a.B, b.B)
	t2 := gf.Mul(a.A^a.B, b.A^b.B) // Karatsuba: (a0+a1)(b0+b1)
	return GFExt2Elem{
		A: t0 ^ gf.Mul(t1, g.C),
		B: t0 ^ t2, // = a0b1 ⊕ a1b0 ⊕ a1b1
	}
}

// Inv returns a⁻¹.  N(a0+a1β) = a0²⊕a0a1⊕a1²C;  a⁻¹ = ((a0⊕a1)+a1β)/N(a).
func (g *GF2nExt2) Inv(a GFExt2Elem) GFExt2Elem {
	gf := g.Base
	norm := gf.Add(gf.Add(gf.Mul(a.A, a.A), gf.Mul(a.A, a.B)), gf.Mul(gf.Mul(a.B, a.B), g.C))
	ni := gf.Inv(norm)
	return GFExt2Elem{gf.Mul(a.A^a.B, ni), gf.Mul(a.B, ni)}
}

// EvalPoly evaluates f(x) = c[0]+c[1]x+… via Horner in GF(q²).
func (g *GF2nExt2) EvalPoly(coeffs []GFExt2Elem, x GFExt2Elem) GFExt2Elem {
	if len(coeffs) == 0 {
		return g.Zero()
	}
	r := coeffs[len(coeffs)-1]
	for k := len(coeffs) - 2; k >= 0; k-- {
		r = g.Add(g.Mul(r, x), coeffs[k])
	}
	return r
}

// RandPolyWithValue returns a random degree-deg poly over GF(q²) with f(t)=a.
func (g *GF2nExt2) RandPolyWithValue(deg int, t, a GFExt2Elem) []GFExt2Elem {
	q := int(g.Base.Q)
	c := make([]GFExt2Elem, deg+1)
	for i := range c {
		c[i] = GFExt2Elem{uint32(rand.Intn(q)), uint32(rand.Intn(q))}
	}
	cur := g.EvalPoly(c, t)
	c[0].A ^= cur.A ^ a.A
	c[0].B ^= cur.B ^ a.B
	return c
}

// RandDistinctNonzero returns count distinct nonzero random elements of GF(q²).
func (g *GF2nExt2) RandDistinctNonzero(count int) []GFExt2Elem {
	q := int(g.Base.Q)
	seen := make(map[uint64]struct{}, count+1)
	seen[0] = struct{}{} // exclude 0
	out := make([]GFExt2Elem, 0, count)
	for len(out) < count {
		a, b := uint32(rand.Intn(q)), uint32(rand.Intn(q))
		k := uint64(a)<<32 | uint64(b)
		if _, dup := seen[k]; !dup {
			seen[k] = struct{}{}
			out = append(out, GFExt2Elem{a, b})
		}
	}
	return out
}

// Phi evaluates (a+b·β) as a GF(q)-linear polynomial at w ∈ GF(q):
// φ(a+bβ, w) = a ⊕ (b⊗w).
func (g *GF2nExt2) Phi(e GFExt2Elem, w uint32) uint32 {
	return e.A ^ g.Base.Mul(e.B, w)
}

// ConcAux holds precomputed auxiliary data produced during Conc. RM query generation.
// LagrangeWeights[j] = λⱼ(0) = Πₖ≠ⱼ zₖ / Πₖ≠ⱼ(zₖ⊕zⱼ) in GF(q²).
// These are precomputed in O(s²) and allow O(s) decode.
type ConcAux struct {
	LagrangeWeights []GFExt2Elem
	ZVals           []GFExt2Elem
}

// ScalarMul returns e·a where a ∈ Fq, using two base-field multiplications.
func (g *GF2nExt2) ScalarMul(e GFExt2Elem, a uint32) GFExt2Elem {
	if a == 0 {
		return GFExt2Elem{}
	}
	return GFExt2Elem{g.Base.Mul(e.A, a), g.Base.Mul(e.B, a)}
}

// PhiInv recovers u ∈ GF(q²) from its q φ-projections yBlock[w] = φ(u,w) = u.A⊕u.B⊗w.
// Uses the identity Σ_{w ∈ Fq} φ(u,w)·(β+w)⁻¹ = u, which follows from the Lagrange
// formula with nodes Fq at point β and the fact that Π_{v ∈ Fq}(β+v) = β^q+β = 1.
// len(yBlock) must equal int(g.Base.Q).
func (g *GF2nExt2) PhiInv(yBlock []uint32) GFExt2Elem {
	var acc GFExt2Elem
	for w, y := range yBlock {
		if y != 0 {
			acc = g.Add(acc, g.ScalarMul(g.BetaInvWeights[w], y))
		}
	}
	return acc
}

// LagrangeWeightsAt0 precomputes λⱼ(0) for all j in O(s²).
// These weights are query-specific (depend on the random zⱼ values).
func (g *GF2nExt2) LagrangeWeightsAt0(zvals []GFExt2Elem) []GFExt2Elem {
	s := len(zvals)
	weights := make([]GFExt2Elem, s)

	// Compute P = Πⱼ zⱼ incrementally.
	P := g.One()
	for _, z := range zvals {
		P = g.Mul(P, z)
	}

	for j := 0; j < s; j++ {
		// numer_j = P / zⱼ  (product excluding zⱼ)
		numer := g.Mul(P, g.Inv(zvals[j]))

		// denom_j = Πₖ≠ⱼ (zₖ ⊕ zⱼ)
		denom := g.One()
		for k := 0; k < s; k++ {
			if k != j {
				denom = g.Mul(denom, g.Add(zvals[k], zvals[j]))
			}
		}
		weights[j] = g.Mul(numer, g.Inv(denom))
	}
	return weights
}

// EvalLagrangeAt0 computes f(0) = Σⱼ uvals[j]·weights[j] in O(s).
func (g *GF2nExt2) EvalLagrangeAt0(uvals, weights []GFExt2Elem) GFExt2Elem {
	var acc GFExt2Elem
	for j := range uvals {
		acc = g.Add(acc, g.Mul(uvals[j], weights[j]))
	}
	return acc
}
