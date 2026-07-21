package utils

import "fmt"

// extPolyMulSchoolbookThreshold controls when polynomial multiplication over
// GF(q²) switches from schoolbook multiplication to Karatsuba multiplication.
// It is deliberately package-local so benchmarks can guide later tuning.
const extPolyMulSchoolbookThreshold = 32

// extProductNode is a node in a subproduct tree. Its polynomial is
// ∏_(start <= j < end) (X + z_j); addition and subtraction coincide because
// GF(q²) has characteristic two.
type extProductNode struct {
	poly        []GFExt2Elem
	left, right *extProductNode
	start, end  int
}

func isZeroGFExt2(a GFExt2Elem) bool {
	return a.A == 0 && a.B == 0
}

func trimExtPoly(a []GFExt2Elem) []GFExt2Elem {
	for len(a) > 0 && isZeroGFExt2(a[len(a)-1]) {
		a = a[:len(a)-1]
	}
	return a
}

func addExtPolys(a, b []GFExt2Elem) []GFExt2Elem {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	out := make([]GFExt2Elem, n)
	for i := 0; i < n; i++ {
		if i < len(a) {
			out[i] = a[i]
		}
		if i < len(b) {
			out[i].A ^= b[i].A
			out[i].B ^= b[i].B
		}
	}
	return trimExtPoly(out)
}

func addExtPolyAt(dst, src []GFExt2Elem, offset int) {
	for i, coefficient := range src {
		dst[offset+i].A ^= coefficient.A
		dst[offset+i].B ^= coefficient.B
	}
}

func (g *GF2nExt2) mulExtPolysSchoolbook(a, b []GFExt2Elem) []GFExt2Elem {
	a = trimExtPoly(a)
	b = trimExtPoly(b)
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	out := make([]GFExt2Elem, len(a)+len(b)-1)
	for i, ai := range a {
		if isZeroGFExt2(ai) {
			continue
		}
		for j, bj := range b {
			if !isZeroGFExt2(bj) {
				out[i+j] = g.Add(out[i+j], g.Mul(ai, bj))
			}
		}
	}
	return trimExtPoly(out)
}

// mulExtPolys multiplies polynomials using Karatsuba recursion, with
// schoolbook multiplication at small sizes. Coefficients are in ascending
// degree order.
func (g *GF2nExt2) mulExtPolys(a, b []GFExt2Elem) []GFExt2Elem {
	a = trimExtPoly(a)
	b = trimExtPoly(b)
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	if len(a) <= extPolyMulSchoolbookThreshold || len(b) <= extPolyMulSchoolbookThreshold {
		return g.mulExtPolysSchoolbook(a, b)
	}

	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	mid := n / 2
	aSplit := mid
	if aSplit > len(a) {
		aSplit = len(a)
	}
	bSplit := mid
	if bSplit > len(b) {
		bSplit = len(b)
	}
	a0, a1 := a[:aSplit], a[aSplit:]
	b0, b1 := b[:bSplit], b[bSplit:]

	z0 := g.mulExtPolys(a0, b0)
	z2 := g.mulExtPolys(a1, b1)
	z1 := g.mulExtPolys(addExtPolys(a0, a1), addExtPolys(b0, b1))
	z1 = addExtPolys(addExtPolys(z1, z0), z2)

	out := make([]GFExt2Elem, len(a)+len(b)-1)
	addExtPolyAt(out, z0, 0)
	addExtPolyAt(out, z1, mid)
	addExtPolyAt(out, z2, 2*mid)
	return trimExtPoly(out)
}

func truncateExtPoly(a []GFExt2Elem, n int) []GFExt2Elem {
	// Power-series arithmetic must preserve its declared precision even when
	// the highest retained coefficients happen to be zero.
	out := make([]GFExt2Elem, n)
	if len(a) > n {
		a = a[:n]
	}
	copy(out, a)
	return out
}

func reverseExtPoly(a []GFExt2Elem, n int) []GFExt2Elem {
	out := make([]GFExt2Elem, n)
	for i := 0; i < n && i < len(a); i++ {
		out[i] = a[len(a)-1-i]
	}
	return out
}

// inverseExtPolySeries returns a(X)^-1 mod X^n. In characteristic two the
// Newton update h <- h*(2-a*h) simplifies to h <- a*h^2; its error is squared
// on every iteration, so the number of correct coefficients doubles.
func (g *GF2nExt2) inverseExtPolySeries(a []GFExt2Elem, n int) []GFExt2Elem {
	if n == 0 {
		return nil
	}
	if len(a) == 0 || isZeroGFExt2(a[0]) {
		panic("inverseExtPolySeries: zero constant coefficient")
	}
	h := []GFExt2Elem{g.Inv(a[0])}
	for len(h) < n {
		next := 2 * len(h)
		if next > n {
			next = n
		}
		aPrefix := a
		if len(aPrefix) > next {
			aPrefix = aPrefix[:next]
		}
		ah := truncateExtPoly(g.mulExtPolys(aPrefix, h), next)
		h = truncateExtPoly(g.mulExtPolys(h, ah), next)
	}
	return h
}

// modExtPoly computes dividend mod divisor using reversed-polynomial division
// and Newton-series inversion. The divisor need not be monic, but must be
// nonzero.
func (g *GF2nExt2) modExtPoly(dividend, divisor []GFExt2Elem) []GFExt2Elem {
	dividend = trimExtPoly(dividend)
	divisor = trimExtPoly(divisor)
	if len(divisor) == 0 {
		panic("modExtPoly: division by zero polynomial")
	}
	if len(dividend) < len(divisor) {
		out := make([]GFExt2Elem, len(dividend))
		copy(out, dividend)
		return out
	}

	quotientLength := len(dividend) - len(divisor) + 1
	reversedDividend := reverseExtPoly(dividend, quotientLength)
	reversedDivisor := reverseExtPoly(divisor, len(divisor))
	inverse := g.inverseExtPolySeries(reversedDivisor, quotientLength)
	reversedQuotient := truncateExtPoly(g.mulExtPolys(reversedDividend, inverse), quotientLength)
	quotient := reverseExtPoly(reversedQuotient, quotientLength)
	product := g.mulExtPolys(quotient, divisor)

	remainderLength := len(divisor) - 1
	remainder := make([]GFExt2Elem, remainderLength)
	for i := 0; i < remainderLength; i++ {
		if i < len(dividend) {
			remainder[i] = dividend[i]
		}
		if i < len(product) {
			remainder[i] = g.Add(remainder[i], product[i])
		}
	}
	return trimExtPoly(remainder)
}

func (g *GF2nExt2) buildExtProductTree(points []GFExt2Elem, start int) *extProductNode {
	if len(points) == 1 {
		return &extProductNode{
			poly:  []GFExt2Elem{points[0], g.One()},
			start: start,
			end:   start + 1,
		}
	}
	mid := len(points) / 2
	left := g.buildExtProductTree(points[:mid], start)
	right := g.buildExtProductTree(points[mid:], start+mid)
	return &extProductNode{
		poly:  g.mulExtPolys(left.poly, right.poly),
		left:  left,
		right: right,
		start: start,
		end:   start + len(points),
	}
}

func derivativeExtPoly(a []GFExt2Elem) []GFExt2Elem {
	if len(a) <= 1 {
		return nil
	}
	// In characteristic two, multiplying a coefficient by its exponent keeps
	// it exactly when the exponent is odd and maps it to zero when even.
	out := make([]GFExt2Elem, len(a)-1)
	for exponent := 1; exponent < len(a); exponent += 2 {
		out[exponent-1] = a[exponent]
	}
	return trimExtPoly(out)
}

func (g *GF2nExt2) evalExtPolyWithProductTree(
	poly []GFExt2Elem,
	node *extProductNode,
	values []GFExt2Elem,
) {
	if node.end-node.start == 1 {
		if len(poly) > 0 {
			values[node.start] = poly[0]
		}
		return
	}
	leftRemainder := g.modExtPoly(poly, node.left.poly)
	rightRemainder := g.modExtPoly(poly, node.right.poly)
	g.evalExtPolyWithProductTree(leftRemainder, node.left, values)
	g.evalExtPolyWithProductTree(rightRemainder, node.right, values)
}

// batchInvertGFExt2 computes all inverses using one field inversion and O(n)
// multiplications. Inputs must all be nonzero.
func (g *GF2nExt2) batchInvertGFExt2(values []GFExt2Elem) []GFExt2Elem {
	n := len(values)
	if n == 0 {
		return nil
	}
	prefix := make([]GFExt2Elem, n)
	product := g.One()
	for i, value := range values {
		prefix[i] = product
		product = g.Mul(product, value)
	}
	productInverse := g.Inv(product)
	inverses := make([]GFExt2Elem, n)
	for i := n - 1; i >= 0; i-- {
		inverses[i] = g.Mul(productInverse, prefix[i])
		productInverse = g.Mul(productInverse, values[i])
	}
	return inverses
}

// LagrangeWeightsAt0Fast computes the same query-specific weights as
// LagrangeWeightsAt0 without replacing that established implementation.
//
// It builds M(X)=∏_j(X+z_j), evaluates M'(z_j) with a subproduct tree, and
// uses λ_j(0)=M(0)/(z_j*M'(z_j)). Polynomial multiplication uses Karatsuba;
// polynomial remainders use Newton-series inversion. The resulting complexity
// is subquadratic in the number of interpolation points.
func (g *GF2nExt2) LagrangeWeightsAt0Fast(zvals []GFExt2Elem) ([]GFExt2Elem, error) {
	if len(zvals) == 0 {
		return []GFExt2Elem{}, nil
	}
	seen := make(map[uint64]struct{}, len(zvals))
	for i, z := range zvals {
		if isZeroGFExt2(z) {
			return nil, fmt.Errorf("LagrangeWeightsAt0Fast: interpolation point %d is zero", i)
		}
		key := uint64(z.A)<<32 | uint64(z.B)
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("LagrangeWeightsAt0Fast: interpolation point %d is duplicated", i)
		}
		seen[key] = struct{}{}
	}

	tree := g.buildExtProductTree(zvals, 0)
	derivative := derivativeExtPoly(tree.poly)
	derivativeValues := make([]GFExt2Elem, len(zvals))
	g.evalExtPolyWithProductTree(derivative, tree, derivativeValues)

	denominators := make([]GFExt2Elem, len(zvals))
	for i := range zvals {
		denominators[i] = g.Mul(zvals[i], derivativeValues[i])
		if isZeroGFExt2(denominators[i]) {
			return nil, fmt.Errorf("LagrangeWeightsAt0Fast: zero denominator at interpolation point %d", i)
		}
	}
	inverseDenominators := g.batchInvertGFExt2(denominators)
	constant := tree.poly[0]
	weights := make([]GFExt2Elem, len(zvals))
	for i := range weights {
		weights[i] = g.Mul(constant, inverseDenominators[i])
	}
	return weights, nil
}
