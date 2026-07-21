package utils

import (
	"math/rand"
	"testing"
)

func TestPhiInvAtReconstructsExtensionElement(t *testing.T) {
	gf2 := NewGF2nExt2(NewGF2n(4))
	for count := 2; count <= 8; count++ {
		for trial := 0; trial < 20; trial++ {
			want := GFExt2Elem{A: uint32(rand.Intn(16)), B: uint32(rand.Intn(16))}
			points := gf2.RandDistinctBaseElements(count)
			values := make([]uint32, count)
			for i, point := range points {
				values[i] = gf2.Phi(want, point)
			}
			got, err := gf2.PhiInvAt(values, points)
			if err != nil {
				t.Fatalf("count %d trial %d: PhiInvAt returned error: %v", count, trial, err)
			}
			if got != want {
				t.Fatalf("count %d trial %d: PhiInvAt = %+v, want %+v", count, trial, got, want)
			}
		}
	}
}

func randomExtPoly(rng *rand.Rand, q, length int) []GFExt2Elem {
	poly := make([]GFExt2Elem, length)
	for i := range poly {
		poly[i] = GFExt2Elem{A: uint32(rng.Intn(q)), B: uint32(rng.Intn(q))}
	}
	if length > 0 && isZeroGFExt2(poly[length-1]) {
		poly[length-1] = GFExt2Elem{A: 1}
	}
	return poly
}

func equalExtPolys(a, b []GFExt2Elem) bool {
	a = trimExtPoly(a)
	b = trimExtPoly(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func modExtPolySchoolbook(g *GF2nExt2, dividend, divisor []GFExt2Elem) []GFExt2Elem {
	dividend = append([]GFExt2Elem(nil), trimExtPoly(dividend)...)
	divisor = trimExtPoly(divisor)
	if len(dividend) < len(divisor) {
		return dividend
	}
	divisorDegree := len(divisor) - 1
	leadingInverse := g.Inv(divisor[divisorDegree])
	for degree := len(dividend) - 1; degree >= divisorDegree; degree-- {
		coefficient := g.Mul(dividend[degree], leadingInverse)
		if isZeroGFExt2(coefficient) {
			continue
		}
		offset := degree - divisorDegree
		for j, divisorCoefficient := range divisor {
			dividend[offset+j] = g.Add(dividend[offset+j], g.Mul(coefficient, divisorCoefficient))
		}
	}
	return trimExtPoly(dividend[:divisorDegree])
}

func TestExtPolynomialKaratsubaMatchesSchoolbook(t *testing.T) {
	gf2 := NewGF2nExt2(NewGF2n(8))
	rng := rand.New(rand.NewSource(1))
	for _, lengths := range [][2]int{{1, 1}, {7, 13}, {33, 34}, {65, 47}, {128, 129}} {
		a := randomExtPoly(rng, int(gf2.Base.Q), lengths[0])
		b := randomExtPoly(rng, int(gf2.Base.Q), lengths[1])
		got := gf2.mulExtPolys(a, b)
		want := gf2.mulExtPolysSchoolbook(a, b)
		if !equalExtPolys(got, want) {
			t.Fatalf("polynomial product mismatch for lengths %d and %d", lengths[0], lengths[1])
		}
	}
}

func TestExtPolynomialFastRemainder(t *testing.T) {
	gf2 := NewGF2nExt2(NewGF2n(8))
	rng := rand.New(rand.NewSource(2))
	for _, lengths := range [][2]int{{8, 3}, {40, 17}, {97, 33}, {160, 65}} {
		dividend := randomExtPoly(rng, int(gf2.Base.Q), lengths[0])
		divisor := randomExtPoly(rng, int(gf2.Base.Q), lengths[1])
		remainder := gf2.modExtPoly(dividend, divisor)
		want := modExtPolySchoolbook(gf2, dividend, divisor)
		if !equalExtPolys(remainder, want) {
			t.Fatalf("remainder mismatch for lengths %d and %d", lengths[0], lengths[1])
		}
		if len(remainder) >= len(divisor) {
			t.Fatalf("remainder degree is not below divisor degree for lengths %d and %d", lengths[0], lengths[1])
		}

		// f(x) mod g(x) agrees with f(x) at every root of g. Constructing a
		// divisor with a known root gives an independent check of the division.
		root := GFExt2Elem{A: uint32(rng.Intn(int(gf2.Base.Q))), B: uint32(rng.Intn(int(gf2.Base.Q)))}
		knownRootDivisor := gf2.mulExtPolys(divisor, []GFExt2Elem{root, gf2.One()})
		knownRootRemainder := gf2.modExtPoly(dividend, knownRootDivisor)
		if gf2.EvalPoly(dividend, root) != gf2.EvalPoly(knownRootRemainder, root) {
			t.Fatalf("remainder does not preserve evaluation at a divisor root for lengths %d and %d", lengths[0], lengths[1])
		}
	}
}

func TestLagrangeWeightsAt0FastMatchesQuadratic(t *testing.T) {
	for _, n := range []int{4, 8} {
		gf2 := NewGF2nExt2(NewGF2n(n))
		for _, count := range []int{1, 2, 3, 7, 16, 31, 64, 127} {
			points := gf2.RandDistinctNonzero(count)
			got, err := gf2.LagrangeWeightsAt0Fast(points)
			if err != nil {
				t.Fatalf("n=%d count=%d: fast weights returned error: %v", n, count, err)
			}
			want := gf2.LagrangeWeightsAt0(points)
			if len(got) != len(want) {
				t.Fatalf("n=%d count=%d: got %d weights, want %d", n, count, len(got), len(want))
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("n=%d count=%d weight=%d: got %+v, want %+v", n, count, i, got[i], want[i])
				}
			}
		}
	}

	// Exercise deeper product/remainder trees independently of the small-field
	// cases above.
	gf2 := NewGF2nExt2(NewGF2n(8))
	points := gf2.RandDistinctNonzero(1024)
	got, err := gf2.LagrangeWeightsAt0Fast(points)
	if err != nil {
		t.Fatalf("count=1024: fast weights returned error: %v", err)
	}
	want := gf2.LagrangeWeightsAt0(points)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("count=1024 weight=%d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestLagrangeWeightsAt0FastInterpolatesAtZero(t *testing.T) {
	gf2 := NewGF2nExt2(NewGF2n(8))
	rng := rand.New(rand.NewSource(3))
	for _, count := range []int{2, 9, 32, 100} {
		points := gf2.RandDistinctNonzero(count)
		weights, err := gf2.LagrangeWeightsAt0Fast(points)
		if err != nil {
			t.Fatalf("count=%d: fast weights returned error: %v", count, err)
		}
		poly := randomExtPoly(rng, int(gf2.Base.Q), count)
		values := make([]GFExt2Elem, count)
		for i, point := range points {
			values[i] = gf2.EvalPoly(poly, point)
		}
		if got := gf2.EvalLagrangeAt0(values, weights); got != poly[0] {
			t.Fatalf("count=%d: interpolation at zero = %+v, want %+v", count, got, poly[0])
		}
	}
}

func TestLagrangeWeightsAt0FastRejectsInvalidPoints(t *testing.T) {
	gf2 := NewGF2nExt2(NewGF2n(4))
	weights, err := gf2.LagrangeWeightsAt0Fast(nil)
	if err != nil || len(weights) != 0 {
		t.Fatalf("fast weights for no points = %v, %v; want empty result and nil error", weights, err)
	}
	if _, err := gf2.LagrangeWeightsAt0Fast([]GFExt2Elem{{A: 1}, {}}); err == nil {
		t.Fatal("fast weights accepted a zero interpolation point")
	}
	if _, err := gf2.LagrangeWeightsAt0Fast([]GFExt2Elem{{A: 1}, {A: 1}}); err == nil {
		t.Fatal("fast weights accepted duplicate interpolation points")
	}
}

func BenchmarkLagrangeWeightsAt0Implementations(b *testing.B) {
	gf2 := NewGF2nExt2(NewGF2n(8))
	for _, count := range []int{32, 128, 512, 2048, 8192, 32768} {
		points := gf2.RandDistinctNonzero(count)
		b.Run("quadratic/count="+benchmarkInt(count), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = gf2.LagrangeWeightsAt0(points)
			}
		})
		b.Run("fast/count="+benchmarkInt(count), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := gf2.LagrangeWeightsAt0Fast(points); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchmarkInt(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
