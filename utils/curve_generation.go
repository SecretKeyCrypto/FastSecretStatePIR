package utils

import (
	"fmt"
	"math/rand"
)

// EvaluatePolynomial evaluates a polynomial using Horner's method.
// coeffs[k] is the coefficient of x^k (ascending order).
func EvaluatePolynomial(coeffs []int, x, q int) int {
	result := 0
	for k := len(coeffs) - 1; k >= 0; k-- {
		result = (result*x + coeffs[k]) % q
	}
	return result
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
			v = (v*x + coeffs[k]) % q
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

// GenerateConcCurvePoints3D generates the Conc. RM query for a 3D (m=3) database.
// Returns (checksum, ℓ, ConcAux):
//   - checksum: XOR of all L query indices (prevents dead-code elimination without
//     allocating L elements, which is too large for large q).
//   - ℓ: number of genuine query positions s×r, excluding 128 noise positions,
//     where s=d*t+1 and r=d+1 for extension degree e=2. Thus L=ℓ+128.
//   - ConcAux: Lagrange weights + z-values for decoding.
func GenerateConcCurvePoints3D(gf2 *GF2nExt2, t, d, i int) (uint64, int, ConcAux) {
	const noiseCount = 128

	gf := gf2.Base
	q := int(gf.Q)
	if t < 0 || d < 1 || d >= q {
		panic(fmt.Sprintf("concatenated RM parameters require t >= 0 and 1 <= d <= q-1, got t=%d d=%d q=%d", t, d, q))
	}
	s := d*t + 1
	r := d + 1
	if s > q*q-1 {
		panic(fmt.Sprintf("concatenated RM query needs %d distinct nonzero points, but GF(q^2) has only %d", s, q*q-1))
	}
	ell := s * r

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

	// The full-field case streams a deterministic enumeration. Besides avoiding
	// coupon-collector sampling, it enables the sum decoder below.
	fullOuterField := s == q*q-1
	var zvals []GFExt2Elem
	if !fullOuterField {
		zvals = gf2.RandDistinctNonzero(s)
	}

	// Stream all base and noisy query indices into a checksum (no allocation of the full slice).
	q2 := q * q
	q3 := q2 * q
	var wValues [][]uint32
	var fullInnerPoints []uint32
	if r < q {
		wValues = make([][]uint32, s)
	} else {
		fullInnerPoints = make([]uint32, q)
		for w := range fullInnerPoints {
			fullInnerPoints[w] = uint32(w)
		}
	}
	var checksum uint64
	for j := 0; j < s; j++ {
		zval := gf2.NonzeroElementAt(j)
		if !fullOuterField {
			zval = zvals[j]
		}
		// Evaluate each ψₕ at zⱼ.
		v0 := gf2.EvalPoly(psi[0], zval)
		v1 := gf2.EvalPoly(psi[1], zval)
		v2 := gf2.EvalPoly(psi[2], zval)
		var innerPoints []uint32
		if r == q {
			innerPoints = fullInnerPoints
		} else {
			innerPoints = gf2.RandDistinctBaseElements(r)
			wValues[j] = innerPoints
		}
		// Apply the inner evaluation map at r=d+1 distinct base-field points.
		for _, ww := range innerPoints {
			c0 := gf2.Phi(v0, ww)
			c1 := gf2.Phi(v1, ww)
			c2 := gf2.Phi(v2, ww)
			checksum ^= uint64(int(c2)*q2 + int(c1)*q + int(c0))
		}
	}

	for k := 0; k < noiseCount; k++ {
		checksum ^= uint64(rand.Intn(q3))
	}

	// Restricting a degree-d RM polynomial to a degree-t curve produces a
	// univariate polynomial of degree at most d*t, so all s=d*t+1 values are
	// required for general codewords.
	if fullOuterField {
		return checksum, ell, ConcAux{FullOuterField: true, WValues: wValues}
	}
	weights, err := gf2.LagrangeWeightsAt0Fast(zvals)
	if err != nil {
		panic(fmt.Errorf("precompute concatenated RM interpolation weights: %w", err))
	}

	return checksum, ell, ConcAux{LagrangeWeights: weights, ZVals: zvals, WValues: wValues}
}

// DecodeConcRM recovers the requested RM codeword value from s=d*t+1 blocks
// of r=d+1 server responses each (for extension degree e=2).
// Step 1: apply the inner inverse map φ⁻¹ to each block.
// Step 2: outer Lagrange at 0 — O(s) GF(q²) multiplies using precomputed λⱼ(0).
func DecodeConcRM(gf2 *GF2nExt2, yBlocks [][]uint32, aux ConcAux) (uint32, error) {
	n := len(aux.ZVals)
	if aux.FullOuterField {
		n = int(gf2.Base.Q)*int(gf2.Base.Q) - 1
		if len(aux.ZVals) != 0 || len(aux.LagrangeWeights) != 0 {
			return 0, fmt.Errorf("full-outer-field decoding does not accept explicit interpolation points or weights")
		}
	}
	if n == 0 {
		return 0, fmt.Errorf("concatenated RM auxiliary data has no interpolation points")
	}
	if !aux.FullOuterField && len(aux.LagrangeWeights) != n {
		return 0, fmt.Errorf("concatenated RM auxiliary data has %d points but %d weights", n, len(aux.LagrangeWeights))
	}
	if len(yBlocks) != n {
		return 0, fmt.Errorf("concatenated RM response has %d blocks but auxiliary data requires %d", len(yBlocks), n)
	}
	uvals := make([]GFExt2Elem, n)
	usesFullBaseField := len(aux.WValues) == 0
	if !usesFullBaseField && len(aux.WValues) != n {
		return 0, fmt.Errorf("concatenated RM auxiliary data has inner points for %d blocks; want %d", len(aux.WValues), n)
	}
	for j := 0; j < n; j++ {
		if usesFullBaseField {
			if len(yBlocks[j]) != int(gf2.Base.Q) {
				return 0, fmt.Errorf("concatenated RM response block %d has %d values; full-field inner decoding requires %d", j, len(yBlocks[j]), gf2.Base.Q)
			}
			uvals[j] = gf2.PhiInv(yBlocks[j])
			continue
		}
		value, err := gf2.PhiInvAt(yBlocks[j], aux.WValues[j])
		if err != nil {
			return 0, fmt.Errorf("decode inner block %d: %w", j, err)
		}
		uvals[j] = value
	}
	var result GFExt2Elem
	if aux.FullOuterField {
		// For |F|=q² and deg(f)≤|F|-2, Σ_{z∈F}f(z)=0. Hence
		// f(0)=-Σ_{z∈F*}f(z), which is the same sum in characteristic two.
		for _, value := range uvals {
			result = gf2.Add(result, value)
		}
	} else {
		result = gf2.EvalLagrangeAt0(uvals, aux.LagrangeWeights)
	}
	return result.A, nil
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

// GenerateCurvePointsWithNoise generates the lifted-RS query for the Lifted RS construction.
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
