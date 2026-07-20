package utils

import (
	"math/big"
	"math/rand"
)

// EvaluatePolynomial evaluates a polynomial using Horner's method.
// coeffs[k] is the coefficient of x^k (ascending order).
func EvaluatePolynomial(coeffs []int, x, q int) int {
	result := 0
	for k := len(coeffs) - 1; k >= 0; k-- {
		result = (result*x+coeffs[k])%q
	}
	return result
}

func EvaluatePolynomialForLargeField(coeffs []int, x, q int) int {
	bigQ := big.NewInt(int64(q))
	bigX := big.NewInt(int64(x))
	result := big.NewInt(int64(coeffs[len(coeffs)-1]))
	for k := len(coeffs) - 2; k >= 0; k-- {
		result.Mul(result, bigX)
		result.Mod(result, bigQ)
		result.Add(result, big.NewInt(int64(coeffs[k])))
		result.Mod(result, bigQ)
	}
	return int(result.Int64())
}

// GenerateRandomPolynomialCoeffs generates a random degree-d polynomial over F_q
// whose value at t equals a.
func GenerateRandomPolynomialCoeffs(degree, q, t, a int) []int {
	coeffs := make([]int, degree+1)
	for i := range coeffs {
		coeffs[i] = rand.Intn(q)
	}
	// Adjust constant term so p(t) = a.
	coeffs[0] = (coeffs[0] - (EvaluatePolynomial(coeffs, t, q) - a) + q) % q
	return coeffs
}

// buildDiffTable computes the Newton forward difference table for polynomial p at x=0.
// Returns state where state[k] = Δ^k p(0).
// Initialization cost is O(degree²); used once per polynomial before the hot loop.
func buildDiffTable(coeffs []int, degree, q int) []int {
	state := make([]int, degree+1)
	// Evaluate p at 0, 1, …, degree via Horner.
	for x := 0; x <= degree; x++ {
		v := coeffs[degree]
		for k := degree - 1; k >= 0; k-- {
			v = (v*x+coeffs[k]) % q
		}
		state[x] = v
	}
	// Convert to forward differences in-place: state[k] becomes Δ^k p(0).
	for k := 1; k <= degree; k++ {
		for j := degree; j >= k; j-- {
			state[j] = (state[j] - state[j-1] + q) % q
		}
	}
	return state
}

// advanceDiffState advances the difference table by one evaluation step.
// After the call, state[0] = p(x+1) if state was at position x.
// Hot path: only additions, no multiplications or divisions.
func advanceDiffState(state []int, q int) {
	for k := 0; k < len(state)-1; k++ {
		v := state[k] + state[k+1]
		if v >= q {
			v -= q
		}
		state[k] = v
	}
}

// GenerateCurvePoints generates q-1 query positions for a 2D database of size q×q.
// Uses Newton forward differences: O(degree²) setup + O(q·degree) additions only (no mults).
func GenerateCurvePoints(degree, q, i int) []int {
	x, y := SingleIndexToRowCol(q, i)
	t := rand.Intn(q)
	uniX := GenerateRandomPolynomialCoeffs(degree, q, t, x)
	uniY := GenerateRandomPolynomialCoeffs(degree, q, t, y)

	stateX := buildDiffTable(uniX, degree, q)
	stateY := buildDiffTable(uniY, degree, q)

	points := make([]int, 0, q-1)
	for z := 0; z < q; z++ {
		if z != t {
			// RowColToSingleIndex(q, stateX[0], stateY[0]) = stateY[0]*q + stateX[0]
			points = append(points, stateY[0]*q+stateX[0])
		}
		if z < q-1 {
			advanceDiffState(stateX, q)
			advanceDiffState(stateY, q)
		}
	}
	return points
}

// GenerateCurvePoints3D generates q-1 query positions for a 3D database of size q³.
func GenerateCurvePoints3D(degree, q, i int) []int {
	x, y, z := SingleIndexToRowCol3D(q, i)
	t := rand.Intn(q)
	uniX := GenerateRandomPolynomialCoeffs(degree, q, t, x)
	uniY := GenerateRandomPolynomialCoeffs(degree, q, t, y)
	uniZ := GenerateRandomPolynomialCoeffs(degree, q, t, z)

	stateX := buildDiffTable(uniX, degree, q)
	stateY := buildDiffTable(uniY, degree, q)
	stateZ := buildDiffTable(uniZ, degree, q)

	q2 := q * q
	points := make([]int, 0, q-1)
	for ev := 0; ev < q; ev++ {
		if ev != t {
			// RowColToSingleIndex3D(q, stateX[0], stateY[0], stateZ[0]) = stateZ*q²+stateY*q+stateX
			points = append(points, stateZ[0]*q2+stateY[0]*q+stateX[0])
		}
		if ev < q-1 {
			advanceDiffState(stateX, q)
			advanceDiffState(stateY, q)
			advanceDiffState(stateZ, q)
		}
	}
	return points
}

// ─── Concatenated-RM curve generation (Fig. 4 in the paper) ─────────────────
//
// Parameters: d = q-1 (RM degree for rate 1/m!), e = 2 (extension degree),
//   r = d(e-1)+1 = q, s = dt+1 = (q-1)t+1, ℓ = s·q total query points.
//
// Query generation (QConc):
//   1. Sample m degree-t polynomials ψₕ over GF(q²) with ψₕ(0) = iₕ.
//   2. Sample s distinct nonzero z₁,…,zₛ ∈ GF(q²)*.
//   3. For each j: evaluate ψₕ(zⱼ) ∈ GF(q²); project to q points in F_q^m
//      via φ(a+bβ, w) = a ⊕ b⊗w for w = 0,1,…,q-1.
//   4. Precompute Lagrange weights λⱼ(0) for all s z-values (O(s²)) since the
//      codeword restricted to the curve, g = f∘ψ, has degree d·t and needs
//      s = dt+1 evaluations to be uniquely determined at 0.
//
// Decode (DConc):
//   For each of the s interpolation blocks, the server has returned q values
//   yBlock[j][w] = db[φ(ψ(zⱼ), w)] for w ∈ Fq.
//   Step 1 — φ⁻¹ (O(q) per block): recover uⱼ = g(zⱼ) ∈ GF(q²) via
//     uⱼ = Σ_{w ∈ Fq} yBlock[j][w] · (β+w)⁻¹
//   This is valid because Π_{v ∈ Fq}(β+v) = β^q+β = 1, so the Lagrange
//   weights at point β for nodes Fq are simply (β+w)⁻¹ (precomputed once).
//   Step 2 — outer Lagrange (O(s)): f(i) = g(0) = Σⱼ uⱼ · λⱼ(0).

// GenerateConcCurvePoints3D generates the Conc. RM query for a 3D (m=3) database.
// Returns (checksum, ℓ, ConcAux):
//   - checksum: XOR of all ℓ query indices (prevents dead-code elimination without
//     allocating ℓ elements, which is too large for large q).
//   - ℓ: total number of query positions = s × q.
//   - ConcAux: Lagrange weights + z-values for decoding.
func GenerateConcCurvePoints3D(gf2 *GF2nExt2, t, i int) (uint64, int, ConcAux) {
	gf := gf2.Base
	q := int(gf.Q)
	s := (q-1)*t + 1 // s = dt+1 with d=q-1
	ell := s * q     // ℓ = s·r, r=q

	// Decompose target into (x, y, z) in F_q^3.
	x3D := i % q
	y3D := (i / q) % q
	z3D := i / (q * q)

	zero := GFExt2Elem{0, 0}
	// Sample m=3 random degree-t polynomials over GF(q²) with ψₕ(0) = iₕ ∈ F_q ⊂ GF(q²).
	psi := [3][]GFExt2Elem{
		gf2.RandPolyWithValue(t, zero, GFExt2Elem{uint32(x3D), 0}),
		gf2.RandPolyWithValue(t, zero, GFExt2Elem{uint32(y3D), 0}),
		gf2.RandPolyWithValue(t, zero, GFExt2Elem{uint32(z3D), 0}),
	}

	// Sample s distinct nonzero z-values in GF(q²).
	zvals := gf2.RandDistinctNonzero(s)

	// Stream all ℓ query indices into a checksum (no allocation of the full slice).
	q2 := q * q
	var checksum uint64
	for j := 0; j < s; j++ {
		// Evaluate each ψₕ at zⱼ.
		v0 := gf2.EvalPoly(psi[0], zvals[j])
		v1 := gf2.EvalPoly(psi[1], zvals[j])
		v2 := gf2.EvalPoly(psi[2], zvals[j])
		// Project to F_q^3 for each w ∈ {0,…,q-1} and accumulate index.
		for w := 0; w < q; w++ {
			ww := uint32(w)
			c0 := gf2.Phi(v0, ww)
			c1 := gf2.Phi(v1, ww)
			c2 := gf2.Phi(v2, ww)
			checksum ^= uint64(int(c2)*q2 + int(c1)*q + int(c0))
		}
	}

	// The codeword restricted to the curve, g = f∘ψ, has degree d·t (f has total
	// degree d, ψ has degree t), so all s = dt+1 evaluations are needed to
	// determine g(0) = f(i). Precompute Lagrange weights over all s z-values: O(s²).
	weights := gf2.LagrangeWeightsAt0(zvals)

	return checksum, ell, ConcAux{LagrangeWeights: weights, ZVals: zvals}
}

// DecodeConcRM recovers f(i) = g(0) ∈ GF(q) from s = dt+1 blocks of q server
// responses each, where g = f∘ψ has degree d·t.
// yBlocks[j][w] = db[φ(ψ(zⱼ), w)] for j ∈ [s], w ∈ Fq.
// Step 1: φ⁻¹ per block — O(q) scalar-GF(q²) multiplies using precomputed (β+w)⁻¹.
// Step 2: outer Lagrange at 0 — O(s) GF(q²) multiplies using precomputed λⱼ(0).
func DecodeConcRM(gf2 *GF2nExt2, yBlocks [][]uint32, aux ConcAux) uint32 {
	n := len(aux.ZVals)
	uvals := make([]GFExt2Elem, n)
	for j := 0; j < n; j++ {
		uvals[j] = gf2.PhiInv(yBlocks[j])
	}
	result := gf2.EvalLagrangeAt0(uvals, aux.LagrangeWeights)
	return result.A
}

// GenerateCurvePoints4D generates q-1 query positions for a 4D database of size q⁴.
func GenerateCurvePoints4D(degree, q, i int) []int {
	x, y, z, v := SingleIndexToRowCol4D(q, i)
	t := rand.Intn(q)
	uniX := GenerateRandomPolynomialCoeffs(degree, q, t, x)
	uniY := GenerateRandomPolynomialCoeffs(degree, q, t, y)
	uniZ := GenerateRandomPolynomialCoeffs(degree, q, t, z)
	uniV := GenerateRandomPolynomialCoeffs(degree, q, t, v)

	stateX := buildDiffTable(uniX, degree, q)
	stateY := buildDiffTable(uniY, degree, q)
	stateZ := buildDiffTable(uniZ, degree, q)
	stateV := buildDiffTable(uniV, degree, q)

	q2 := q * q
	q3 := q2 * q
	points := make([]int, 0, q-1)
	for ev := 0; ev < q; ev++ {
		if ev != t {
			points = append(points, stateV[0]*q3+stateZ[0]*q2+stateY[0]*q+stateX[0])
		}
		if ev < q-1 {
			advanceDiffState(stateX, q)
			advanceDiffState(stateY, q)
			advanceDiffState(stateZ, q)
			advanceDiffState(stateV, q)
		}
	}
	return points
}

// GenerateCurvePointsWithNoise generates the PLDN query for the Lifted RS construction.
// Returns (allPoints, realPoints):
//   - allPoints: q-1+noiseCount positions shuffled together, sent to the server.
//   - realPoints: the q-1 genuine curve positions, used by the client for decoding.
func GenerateCurvePointsWithNoise(degree, q, i, noiseCount int) ([]int, []int) {
	x, y := SingleIndexToRowCol(q, i)
	t := rand.Intn(q)
	uniX := GenerateRandomPolynomialCoeffs(degree, q, t, x)
	uniY := GenerateRandomPolynomialCoeffs(degree, q, t, y)

	stateX := buildDiffTable(uniX, degree, q)
	stateY := buildDiffTable(uniY, degree, q)

	real := make([]int, 0, q-1)
	for z := 0; z < q; z++ {
		if z != t {
			real = append(real, stateY[0]*q+stateX[0])
		}
		if z < q-1 {
			advanceDiffState(stateX, q)
			advanceDiffState(stateY, q)
		}
	}

	// Append noiseCount uniform random positions.
	qSq := q * q
	all := make([]int, len(real)+noiseCount)
	copy(all, real)
	for k := len(real); k < len(all); k++ {
		all[k] = rand.Intn(qSq)
	}
	rand.Shuffle(len(all), func(a, b int) { all[a], all[b] = all[b], all[a] })

	return all, real
}

// ─── GF(2^n) curve generation ────────────────────────────────────────────────
//
// Over GF(2^n), addition is XOR so Newton forward differences don't apply.
// We use Horner evaluation instead: O(degree · q) GF multiplications total.

// GenerateCurvePointsGF2n generates q−1 query positions for a 2D GF(2^n) database.
// The curve is a pair of random degree-k polynomials (p_x, p_y) over GF(2^n) with
// p_x(t) = x_target and p_y(t) = y_target for a random evaluation parameter t.
func GenerateCurvePointsGF2n(gf *GF2n, degree, i int) []int {
	q := int(gf.Q)
	x, y := SingleIndexToRowCol(q, i)
	t := rand.Intn(q)

	uniX := gf.RandPolyWithValue(degree, uint32(t), uint32(x))
	uniY := gf.RandPolyWithValue(degree, uint32(t), uint32(y))

	points := make([]int, 0, q-1)
	for z := 0; z < q; z++ {
		if z == t {
			continue
		}
		fx := gf.EvalPoly(uniX, uint32(z))
		fy := gf.EvalPoly(uniY, uint32(z))
		points = append(points, int(fy)*q+int(fx))
	}
	return points
}

// GenerateCurvePointsGF2n3D generates q−1 query positions for a 3D GF(2^n) database.
func GenerateCurvePointsGF2n3D(gf *GF2n, degree, i int) []int {
	q := int(gf.Q)
	x, y, z := SingleIndexToRowCol3D(q, i)
	t := rand.Intn(q)

	uniX := gf.RandPolyWithValue(degree, uint32(t), uint32(x))
	uniY := gf.RandPolyWithValue(degree, uint32(t), uint32(y))
	uniZ := gf.RandPolyWithValue(degree, uint32(t), uint32(z))

	q2 := q * q
	points := make([]int, 0, q-1)
	for ev := 0; ev < q; ev++ {
		if ev == t {
			continue
		}
		fx := gf.EvalPoly(uniX, uint32(ev))
		fy := gf.EvalPoly(uniY, uint32(ev))
		fz := gf.EvalPoly(uniZ, uint32(ev))
		points = append(points, int(fz)*q2+int(fy)*q+int(fx))
	}
	return points
}

// GenerateCurvePointsGF2nWithNoise is the PLDN variant for GF(2^n): generates q−1
// genuine curve positions plus noiseCount uniform-random positions, all shuffled.
// Returns (allPoints, realPoints).
func GenerateCurvePointsGF2nWithNoise(gf *GF2n, degree, i, noiseCount int) ([]int, []int) {
	q := int(gf.Q)
	x, y := SingleIndexToRowCol(q, i)
	t := rand.Intn(q)

	uniX := gf.RandPolyWithValue(degree, uint32(t), uint32(x))
	uniY := gf.RandPolyWithValue(degree, uint32(t), uint32(y))

	real := make([]int, 0, q-1)
	for z := 0; z < q; z++ {
		if z == t {
			continue
		}
		fx := gf.EvalPoly(uniX, uint32(z))
		fy := gf.EvalPoly(uniY, uint32(z))
		real = append(real, int(fy)*q+int(fx))
	}

	qSq := q * q
	all := make([]int, len(real)+noiseCount)
	copy(all, real)
	for k := len(real); k < len(all); k++ {
		all[k] = rand.Intn(qSq)
	}
	rand.Shuffle(len(all), func(a, b int) { all[a], all[b] = all[b], all[a] })
	return all, real
}

// GenerateCurvePoints3DWithNoise is the 3D variant with noise injection for PLDN.
func GenerateCurvePoints3DWithNoise(degree, q, i, noiseCount int) ([]int, []int) {
	x, y, z := SingleIndexToRowCol3D(q, i)
	t := rand.Intn(q)
	uniX := GenerateRandomPolynomialCoeffs(degree, q, t, x)
	uniY := GenerateRandomPolynomialCoeffs(degree, q, t, y)
	uniZ := GenerateRandomPolynomialCoeffs(degree, q, t, z)

	stateX := buildDiffTable(uniX, degree, q)
	stateY := buildDiffTable(uniY, degree, q)
	stateZ := buildDiffTable(uniZ, degree, q)

	q2 := q * q
	real := make([]int, 0, q-1)
	for ev := 0; ev < q; ev++ {
		if ev != t {
			real = append(real, stateZ[0]*q2+stateY[0]*q+stateX[0])
		}
		if ev < q-1 {
			advanceDiffState(stateX, q)
			advanceDiffState(stateY, q)
			advanceDiffState(stateZ, q)
		}
	}

	qCube := q2 * q
	all := make([]int, len(real)+noiseCount)
	copy(all, real)
	for k := len(real); k < len(all); k++ {
		all[k] = rand.Intn(qCube)
	}
	rand.Shuffle(len(all), func(a, b int) { all[a], all[b] = all[b], all[a] })

	return all, real
}
