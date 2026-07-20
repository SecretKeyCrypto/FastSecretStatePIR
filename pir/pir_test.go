package pir

import (
	"fmt"
	"math"
	"math/bits"
	"math/rand"
	"net/http"
	"rme/utils"
	"testing"
	"time"
)

// *************************************************************************************
//
//	Encoding Benchmark
//
// *************************************************************************************
func BenchmarkEncode(b *testing.B) {
	q := 31
	k := 4
	pir := NewPIR(Params{Q: uint64(q), K: uint8(k), M: uint8(2)})

	pir.Gen()
	input := "../input/db.csv"
	output := "../output/matrix.csv"
	FakeDB(q, 6, "../input/db.csv")

	var totalDuration time.Duration

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		pir.Encode(input, output)
		duration := time.Since(start)
		totalDuration += duration
	}

	b.StopTimer()
	fmt.Printf("Average time per operation: %v\n", totalDuration/time.Duration(b.N))
}

// *************************************************************************************
//
//	Query Generation Benchmarks
//
// *************************************************************************************

// onlineParams2D covers the 2D (m=2) parameter space.
// For Lifted RS, rate → 1 so DB size ≈ q² elements × ceil(log₂q / 8) bytes.
// Client sends q-1 positions per query (bandwidth).
var onlineParams2D = []struct {
	q, t  int
	label string
}{
	{521, 4, "2D q≈2^9  db≈543KB  bw=520"},
	{1021, 4, "2D q≈2^10 db≈2.1MB  bw=1020"},
	{4093, 4, "2D q≈2^12 db≈33MB   bw=4092"},
	{16381, 4, "2D q≈2^14 db≈537MB  bw=16380"},
	// paper Table 2 (~128-bit security)
	{65521, 4, "2D q≈2^16 db≈8.6GB  bw=65520 t=4"},
	{65521, 8, "2D q≈2^16 db≈8.6GB  bw=65520 t=8"},
}

// onlineParams3D covers the 3D (m=3) parameter space.
// For Lifted RS, DB size ≈ q³ elements × ceil(log₂q / 8) bytes.
var onlineParams3D = []struct {
	q, t  int
	label string
}{
	{131, 4, "3D q=131   db≈2.2MB  bw=130"},
	{251, 4, "3D q≈2^8   db≈15MB   bw=250"},
	{509, 4, "3D q≈2^9   db≈132MB  bw=508"},
	{1021, 4, "3D q≈2^10  db≈2.1GB  bw=1020"},
	// paper Table 2 (~128-bit security)
	{2039, 4, "3D q≈2^11  db≈17GB   bw=2038 t=4"},
	{2039, 8, "3D q≈2^11  db≈17GB   bw=2038 t=8"},
}

// Keep old table names as aliases so existing benchmarks compile unchanged.
var liftedRSParams2D = []struct {
	q, t  int
	label string
}{
	{65521, 8, "q≈2^16,t=8"},
	{65521, 4, "q≈2^16,t=4"},
	{11587, 4, "q=11587,t=4"},
}

var liftedRSParams3D = []struct {
	q, t  int
	label string
}{
	{2039, 8, "q≈2^11,t=8"},
	{2039, 4, "q≈2^11,t=4"},
	{3691, 4, "q=3691,t=4"},
}

func BenchmarkGenerateQuery(b *testing.B) {
	for _, p := range liftedRSParams2D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(p.q * p.q)
				start := time.Now()
				points := utils.GenerateCurvePoints(p.t, p.q, target)
				pir.PrepareQuerySequence(points)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  [%s] avg query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

func BenchmarkGenerate3DQuery(b *testing.B) {
	for _, p := range liftedRSParams3D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(int(math.Pow(float64(p.q), 3)))
				start := time.Now()
				points := utils.GenerateCurvePoints3D(p.t, p.q, target)
				pir.PrepareQuerySequence(points)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  [%s] avg 3D query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

func BenchmarkGenerate4DQuery(b *testing.B) {
	q := 797
	k := 4
	m := 4
	pir := NewPIR(Params{Q: uint64(q), K: uint8(k), M: uint8(2)})
	pir.Gen()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var total time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := rng.Intn(int(math.Pow(float64(q), float64(m))))
		start := time.Now()
		points := utils.GenerateCurvePoints4D(k, q, target)
		pir.PrepareQuerySequence(points)
		total += time.Since(start)
	}
	b.StopTimer()
	fmt.Printf("Average time per generate 4D query: %v, b.N=%d\n", total/time.Duration(b.N), b.N)
}

// BenchmarkGenerateQueryLifted benchmarks the full PLDN query generation for the
// Lifted RS construction: curve points + noise injection + permutation + shuffle.
// noiseCount = security parameter λ (paper uses L - ℓ > λ, here λ = 128).
func BenchmarkGenerateQueryLifted(b *testing.B) {
	const noiseCount = 128
	for _, p := range liftedRSParams2D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(p.q * p.q)
				start := time.Now()
				allPts, realPts := utils.GenerateCurvePointsWithNoise(p.t, p.q, target, noiseCount)
				pir.PrepareQuerySequenceWithNoise(realPts, 0) // real already generated; noise already in allPts
				_ = allPts
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  [%s] avg PLDN query gen (noise=%d): %v\n", p.label, noiseCount, total/time.Duration(b.N))
		})
	}
}

// *************************************************************************************
//
//	Decoding Benchmarks
//
// *************************************************************************************

func BenchmarkDecodingLargeRecorddQuery(b *testing.B) {
	q := 4093
	k := 3
	m := 2
	pir := NewPIR(Params{Q: uint64(q), K: uint8(k), M: uint8(m)})
	pir.Gen()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var total time.Duration
	// Record size 100 KB; each slice stores log2(q) ≈ 12 bits.
	mockResponse := make([]int, int(100*math.Pow(2, 10)/1.5))
	for i := range mockResponse {
		mockResponse[i] = rng.Intn(q)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := rng.Intn(int(math.Pow(float64(q), float64(m))))
		start := time.Now()
		points := utils.GenerateCurvePoints(k, q, target)
		query := pir.PrepareQuerySequence(points)
		pir.Decode(mockResponse, query)
		total += time.Since(start)
	}
	b.StopTimer()
	fmt.Printf("Average time per Decoding 100KB query: %v, b.N=%d\n", total/time.Duration(b.N), b.N)
}

func BenchmarkDecoding3DQuery(b *testing.B) {
	q := 7919
	k := 5
	m := 3
	pir := NewPIR(Params{Q: uint64(q), K: uint8(k), M: uint8(m)})
	pir.Gen()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var total time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := rng.Intn(int(math.Pow(float64(q), float64(m))))
		points := utils.GenerateCurvePoints3D(k, q, target)
		query := pir.PrepareQuerySequence(points)
		start := time.Now()
		pir.Decode([]int{q - 1}, query)
		total += time.Since(start)
	}
	b.StopTimer()
	fmt.Printf("Average time per Decoding 3D query: %v, b.N=%d\n", total/time.Duration(b.N), b.N)
}

// BenchmarkDecodingLifted benchmarks the PLDN decode path where the server returns
// individual values at the q-1 real curve positions (not a single sum).
func BenchmarkDecodingLifted(b *testing.B) {
	for _, p := range liftedRSParams2D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			// Mock: q-1 server values at real positions.
			mockVals := make([]int, p.q-1)
			for i := range mockVals {
				mockVals[i] = rng.Intn(p.q)
			}

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(p.q * p.q)
				points := utils.GenerateCurvePoints(p.t, p.q, target)
				_, permReal := pir.PrepareQuerySequenceWithNoise(points, 0)
				start := time.Now()
				pir.DecodeLifted(mockVals, permReal)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  [%s] avg PLDN decode: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// *************************************************************************************
//
//	Online Phase Benchmarks (query generation + single-symbol decode)
//
// *************************************************************************************

// BenchmarkQueryGen2D measures just the query-generation step for 2D databases:
// sample q-1 curve points through the target, then apply the secret permutation.
// This produces the list of positions sent to the server.
func BenchmarkQueryGen2D(b *testing.B) {
	for _, p := range onlineParams2D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(p.q * p.q)
				start := time.Now()
				points := utils.GenerateCurvePoints(p.t, p.q, target)
				pir.PrepareQuerySequence(points)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-45s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// BenchmarkDecode2D measures just the decode step for 2D databases:
// given a mock server sum, subtract the PRF masks and recover the record value.
// Uses a single-slice response (one integer mod q from the server).
func BenchmarkDecode2D(b *testing.B) {
	for _, p := range onlineParams2D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			mockSum := []int{rng.Intn(p.q)}

			// Pre-generate a query so decode has a realistic query slice to work with.
			points := utils.GenerateCurvePoints(p.t, p.q, rng.Intn(p.q*p.q))
			query := pir.PrepareQuerySequence(points)

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				pir.Decode(mockSum, query)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-45s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── Prime-field benchmarks at 2^n-scale, t=7 ────────────────────────────────
//
// Largest prime q below 2^n for n = 16,17,18,20 with t=7.
// Lets us compare prime-field performance directly against GF(2^n) at the same bandwidth.

var primeParams2D_t7 = []struct {
	q, t  int
	label string
}{
	{65521, 7, "prime q=65521  (2^16-15) bw=65520 t=7"},
	{131071, 7, "prime q=131071 (2^17-1)  bw=131070 t=7"},
	{262139, 7, "prime q=262139 (2^18-5)  bw=262138 t=7"},
	{1048573, 7, "prime q=1048573(2^20-3)  bw=1048572 t=7"},
}

// BenchmarkQueryGenPrime2D_t7 measures prime-field query generation at 2^n scale with t=7.
func BenchmarkQueryGenPrime2D_t7(b *testing.B) {
	for _, p := range primeParams2D_t7 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(p.q * p.q)
				start := time.Now()
				points := utils.GenerateCurvePoints(p.t, p.q, target)
				pir.PrepareQuerySequence(points)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// BenchmarkDecodePrime2D_t7 measures prime-field decode at 2^n scale with t=7.
func BenchmarkDecodePrime2D_t7(b *testing.B) {
	for _, p := range primeParams2D_t7 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			mockSum := []int{rng.Intn(p.q)}

			points := utils.GenerateCurvePoints(p.t, p.q, rng.Intn(p.q*p.q))
			query := pir.PrepareQuerySequence(points)

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				pir.Decode(mockSum, query)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── GF(2^n) benchmarks (m=2, t=7) ───────────────────────────────────────────
//
// Extension fields GF(2^n) for n = 16, 17, 18, 20 with t=7 servers.
// Codeword has q^2 elements × n bits each; client sends q−1 positions per query.
// Forward differences don't apply (char 2), so curve generation uses Horner: O(t·q) GF muls.

var gf2nParams2D = []struct {
	n, t  int
	label string
}{
	{16, 7, "GF(2^16) q=65536   codeword≈8.6GB  bw=65535  t=7"},
	{17, 7, "GF(2^17) q=131072  codeword≈34GB   bw=131071 t=7"},
	{18, 7, "GF(2^18) q=262144  codeword≈137GB  bw=262143 t=7"},
	{20, 7, "GF(2^20) q=1048576 codeword≈2.2TB  bw=1048575 t=7"},
}

// BenchmarkQueryGenGF2n2D measures query generation for 2D RM over GF(2^n):
// sample q−1 curve points over GF(2^n) through the target, then apply the PRP.
func BenchmarkQueryGenGF2n2D(b *testing.B) {
	for _, p := range gf2nParams2D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			gf := utils.NewGF2n(p.n)
			q := int(gf.Q)
			pir := NewPIR(Params{Q: uint64(q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(q * q)
				start := time.Now()
				points := utils.GenerateCurvePointsGF2n(gf, p.t, target)
				pir.PrepareQuerySequence(points)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// BenchmarkDecodeGF2n2D measures decode for 2D RM over GF(2^n):
// XOR the PRF masks over q−1 positions, then XOR with the server sum.
func BenchmarkDecodeGF2n2D(b *testing.B) {
	for _, p := range gf2nParams2D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			gf := utils.NewGF2n(p.n)
			q := int(gf.Q)
			pir := NewPIR(Params{Q: uint64(q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			mockSum := []int{rng.Intn(q)}

			// Pre-generate a realistic query (q−1 permuted positions).
			points := utils.GenerateCurvePointsGF2n(gf, p.t, rng.Intn(q*q))
			query := pir.PrepareQuerySequence(points)

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				pir.DecodeGF2n(mockSum, query)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── Prime-field m=3 benchmarks at 2^n-scale, t=7 ───────────────────────────

var primeParams3D_t7 = []struct {
	q, t  int
	label string
}{
	{8191, 7, "prime q=8191   (2^13-1)  m=3 bw=8190   t=7"},
	{65521, 7, "prime q=65521  (2^16-15) m=3 bw=65520  t=7"},
	{131071, 7, "prime q=131071 (2^17-1)  m=3 bw=131070 t=7"},
	{262139, 7, "prime q=262139 (2^18-5)  m=3 bw=262138 t=7"},
	{1048573, 7, "prime q=1048573(2^20-3)  m=3 bw=1048572 t=7"},
}

func BenchmarkQueryGenPrime3D_t7(b *testing.B) {
	for _, p := range primeParams3D_t7 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			dbSize := p.q * p.q * p.q

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(dbSize)
				start := time.Now()
				points := utils.GenerateCurvePoints3D(p.t, p.q, target)
				pir.PrepareQuerySequence(points)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-65s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

func BenchmarkDecodePrime3D_t7(b *testing.B) {
	for _, p := range primeParams3D_t7 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			mockSum := []int{rng.Intn(p.q)}
			dbSize := p.q * p.q * p.q

			points := utils.GenerateCurvePoints3D(p.t, p.q, rng.Intn(dbSize))
			query := pir.PrepareQuerySequence(points)

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				pir.Decode(mockSum, query)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-65s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── GF(2^n) m=3 benchmarks, t=7 ────────────────────────────────────────────

var gf2nParams3D = []struct {
	n, t  int
	label string
}{
	{13, 7, "GF(2^13) q=8192    m=3 bw=8191    t=7"},
	{16, 7, "GF(2^16) q=65536   m=3 bw=65535   t=7"},
	{17, 7, "GF(2^17) q=131072  m=3 bw=131071  t=7"},
	{18, 7, "GF(2^18) q=262144  m=3 bw=262143  t=7"},
	{20, 7, "GF(2^20) q=1048576 m=3 bw=1048575 t=7"},
}

func BenchmarkQueryGenGF2n3D(b *testing.B) {
	for _, p := range gf2nParams3D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			gf := utils.NewGF2n(p.n)
			q := int(gf.Q)
			pir := NewPIR(Params{Q: uint64(q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			dbSize := q * q * q

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(dbSize)
				start := time.Now()
				points := utils.GenerateCurvePointsGF2n3D(gf, p.t, target)
				pir.PrepareQuerySequence(points)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-65s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

func BenchmarkDecodeGF2n3D(b *testing.B) {
	for _, p := range gf2nParams3D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			gf := utils.NewGF2n(p.n)
			q := int(gf.Q)
			pir := NewPIR(Params{Q: uint64(q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			mockSum := []int{rng.Intn(q)}
			dbSize := q * q * q

			points := utils.GenerateCurvePointsGF2n3D(gf, p.t, rng.Intn(dbSize))
			query := pir.PrepareQuerySequence(points)

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				pir.DecodeGF2n(mockSum, query)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-65s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── m=2, t=8 benchmarks for prime and GF(2^16) ─────────────────────────────

var params2D_t8 = []struct {
	q, t  int
	gf    bool
	label string
}{
	{8191, 8, false, "prime q=8191   (2^13-1)  m=2 bw=8190   t=8"},
	{8192, 8, true, "GF(2^13) q=8192    m=2 bw=8191   t=8"},
	{65521, 8, false, "prime q=65521  (2^16-15) m=2 bw=65520  t=8"},
	{65536, 8, true, "GF(2^16) q=65536   m=2 bw=65535  t=8"},
}

func BenchmarkQueryGen2D_t8(b *testing.B) {
	for _, p := range params2D_t8 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1 // log2(q)
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(p.q * p.q)
				start := time.Now()
				var points []int
				if p.gf {
					points = utils.GenerateCurvePointsGF2n(gf, p.t, target)
				} else {
					points = utils.GenerateCurvePoints(p.t, p.q, target)
				}
				pir.PrepareQuerySequence(points)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

func BenchmarkDecode2D_t8(b *testing.B) {
	for _, p := range params2D_t8 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			mockSum := []int{rng.Intn(p.q)}

			var points []int
			if p.gf {
				points = utils.GenerateCurvePointsGF2n(gf, p.t, rng.Intn(p.q*p.q))
			} else {
				points = utils.GenerateCurvePoints(p.t, p.q, rng.Intn(p.q*p.q))
			}
			query := pir.PrepareQuerySequence(points)

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				if p.gf {
					pir.DecodeGF2n(mockSum, query)
				} else {
					pir.Decode(mockSum, query)
				}
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── m=3, t=8 benchmarks ─────────────────────────────────────────────────────

var params3D_t8 = []struct {
	q, t  int
	gf    bool
	label string
}{
	{8191, 8, false, "prime q=8191   (2^13-1)  m=3 bw=8190   t=8"},
	{8192, 8, true, "GF(2^13) q=8192    m=3 bw=8191   t=8"},
	{65521, 8, false, "prime q=65521  (2^16-15) m=3 bw=65520  t=8"},
	{65536, 8, true, "GF(2^16) q=65536   m=3 bw=65535  t=8"},
}

func BenchmarkQueryGen3D_t8(b *testing.B) {
	for _, p := range params3D_t8 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			dbSize := p.q * p.q * p.q

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(dbSize)
				start := time.Now()
				var points []int
				if p.gf {
					points = utils.GenerateCurvePointsGF2n3D(gf, p.t, target)
				} else {
					points = utils.GenerateCurvePoints3D(p.t, p.q, target)
				}
				pir.PrepareQuerySequence(points)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

func BenchmarkDecode3D_t8(b *testing.B) {
	for _, p := range params3D_t8 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			mockSum := []int{rng.Intn(p.q)}
			dbSize := p.q * p.q * p.q

			var points []int
			if p.gf {
				points = utils.GenerateCurvePointsGF2n3D(gf, p.t, rng.Intn(dbSize))
			} else {
				points = utils.GenerateCurvePoints3D(p.t, p.q, rng.Intn(dbSize))
			}
			query := pir.PrepareQuerySequence(points)

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				if p.gf {
					pir.DecodeGF2n(mockSum, query)
				} else {
					pir.Decode(mockSum, query)
				}
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── m=3, t=5 benchmarks ─────────────────────────────────────────────────────

var params3D_t5 = []struct {
	q, t  int
	gf    bool
	label string
}{
	{65521, 5, false, "prime q=65521  (2^16-15) m=3 bw=65520  t=5"},
	{65536, 5, true, "GF(2^16) q=65536   m=3 bw=65535  t=5"},
}

func BenchmarkQueryGen3D_t5(b *testing.B) {
	const noiseCount = 128
	for _, p := range params3D_t5 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			dbSize := p.q * p.q * p.q

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(dbSize)
				start := time.Now()
				var points []int
				if p.gf {
					points = utils.GenerateCurvePointsGF2n3D(gf, p.t, target)
				} else {
					points = utils.GenerateCurvePoints3D(p.t, p.q, target)
				}
				pir.PrepareQuerySequenceWithNoise(points, noiseCount)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

func BenchmarkDecode3D_t5(b *testing.B) {
	const noiseCount = 128
	for _, p := range params3D_t5 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			dbSize := p.q * p.q * p.q

			// Pre-generate allPermuted (sent to server) and permReal (real positions).
			var realPts []int
			if p.gf {
				realPts = utils.GenerateCurvePointsGF2n3D(gf, p.t, rng.Intn(dbSize))
			} else {
				realPts = utils.GenerateCurvePoints3D(p.t, p.q, rng.Intn(dbSize))
			}
			allPermuted, permReal := pir.PrepareQuerySequenceWithNoise(realPts, noiseCount)

			// Mock server response: one field element per queried position.
			mockResp := make([]int, len(allPermuted))
			for i := range mockResp {
				mockResp[i] = rng.Intn(p.q)
			}

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				// Extract server values at the q-1 real positions (discard noise).
				realSet := make(map[int]int, len(permReal))
				for j, rp := range permReal {
					realSet[rp] = j
				}
				serverValsAtReal := make([]int, len(permReal))
				for k, pos := range allPermuted {
					if j, ok := realSet[pos]; ok {
						serverValsAtReal[j] = mockResp[k]
					}
				}
				if p.gf {
					pir.DecodeLiftedGF2n(serverValsAtReal, permReal)
				} else {
					pir.DecodeLifted(serverValsAtReal, permReal)
				}
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── m=3, t=6, q=2^14 benchmarks ─────────────────────────────────────────────

var params3D_t6_q14 = []struct {
	q, t  int
	gf    bool
	label string
}{
	{16381, 6, false, "prime q=16381  (2^14-3)  m=3 bw=16380  t=6"},
	{16384, 6, true, "GF(2^14) q=16384   m=3 bw=16383  t=6"},
}

func BenchmarkQueryGen3D_t6_q14(b *testing.B) {
	const noiseCount = 128
	for _, p := range params3D_t6_q14 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			dbSize := p.q * p.q * p.q

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(dbSize)
				start := time.Now()
				var points []int
				if p.gf {
					points = utils.GenerateCurvePointsGF2n3D(gf, p.t, target)
				} else {
					points = utils.GenerateCurvePoints3D(p.t, p.q, target)
				}
				pir.PrepareQuerySequenceWithNoise(points, noiseCount)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

func BenchmarkDecode3D_t6_q14(b *testing.B) {
	const noiseCount = 128
	for _, p := range params3D_t6_q14 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			dbSize := p.q * p.q * p.q

			var realPts []int
			if p.gf {
				realPts = utils.GenerateCurvePointsGF2n3D(gf, p.t, rng.Intn(dbSize))
			} else {
				realPts = utils.GenerateCurvePoints3D(p.t, p.q, rng.Intn(dbSize))
			}
			allPermuted, permReal := pir.PrepareQuerySequenceWithNoise(realPts, noiseCount)

			mockResp := make([]int, len(allPermuted))
			for i := range mockResp {
				mockResp[i] = rng.Intn(p.q)
			}

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				realSet := make(map[int]int, len(permReal))
				for j, rp := range permReal {
					realSet[rp] = j
				}
				serverValsAtReal := make([]int, len(permReal))
				for k, pos := range allPermuted {
					if j, ok := realSet[pos]; ok {
						serverValsAtReal[j] = mockResp[k]
					}
				}
				if p.gf {
					pir.DecodeLiftedGF2n(serverValsAtReal, permReal)
				} else {
					pir.DecodeLifted(serverValsAtReal, permReal)
				}
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── m=3, t=6 benchmarks ─────────────────────────────────────────────────────

var params3D_t6 = []struct {
	q, t  int
	gf    bool
	label string
}{
	{65521, 6, false, "prime q=65521  (2^16-15) m=3 bw=65520  t=6"},
	{65536, 6, true, "GF(2^16) q=65536   m=3 bw=65535  t=6"},
}

func BenchmarkQueryGen3D_t6(b *testing.B) {
	const noiseCount = 128
	for _, p := range params3D_t6 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			dbSize := p.q * p.q * p.q

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(dbSize)
				start := time.Now()
				var points []int
				if p.gf {
					points = utils.GenerateCurvePointsGF2n3D(gf, p.t, target)
				} else {
					points = utils.GenerateCurvePoints3D(p.t, p.q, target)
				}
				pir.PrepareQuerySequenceWithNoise(points, noiseCount)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

func BenchmarkDecode3D_t6(b *testing.B) {
	const noiseCount = 128
	for _, p := range params3D_t6 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			dbSize := p.q * p.q * p.q

			var realPts []int
			if p.gf {
				realPts = utils.GenerateCurvePointsGF2n3D(gf, p.t, rng.Intn(dbSize))
			} else {
				realPts = utils.GenerateCurvePoints3D(p.t, p.q, rng.Intn(dbSize))
			}
			allPermuted, permReal := pir.PrepareQuerySequenceWithNoise(realPts, noiseCount)

			mockResp := make([]int, len(allPermuted))
			for i := range mockResp {
				mockResp[i] = rng.Intn(p.q)
			}

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				realSet := make(map[int]int, len(permReal))
				for j, rp := range permReal {
					realSet[rp] = j
				}
				serverValsAtReal := make([]int, len(permReal))
				for k, pos := range allPermuted {
					if j, ok := realSet[pos]; ok {
						serverValsAtReal[j] = mockResp[k]
					}
				}
				if p.gf {
					pir.DecodeLiftedGF2n(serverValsAtReal, permReal)
				} else {
					pir.DecodeLifted(serverValsAtReal, permReal)
				}
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-60s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── PLDN m=2 benchmarks ─────────────────────────────────────────────────────

var paramsPLDN2D = []struct {
	q, t  int
	gf    bool
	label string
}{
	{65521, 8, false, "prime q=65521  (2^16-15) m=2 bw=65520  t=8"},
	{65536, 8, true, "GF(2^16) q=65536   m=2 bw=65535  t=8"},
	{131071, 7, false, "prime q=131071 (2^17-1)  m=2 bw=131070 t=7"},
	{131072, 7, true, "GF(2^17) q=131072  m=2 bw=131071 t=7"},
	{262139, 7, false, "prime q=262139 (2^18-5)  m=2 bw=262138 t=7"},
	{262144, 7, true, "GF(2^18) q=262144  m=2 bw=262143 t=7"},
	{1048573, 7, false, "prime q=1048573(2^20-3)  m=2 bw=1048572 t=7"},
	{1048576, 7, true, "GF(2^20) q=1048576 m=2 bw=1048575 t=7"},
	// Table 1 new parameter selections (Bounded 2^50 column).
	{4093, 6, false, "prime q=4093   (2^12-3)  m=2 bw=4092   t=6"},
	{32749, 5, false, "prime q=32749  (2^15-19) m=2 bw=32748  t=5"},
	{262139, 5, false, "prime q=262139 (2^18-5)  m=2 bw=262138 t=5"},
	{1048573, 5, false, "prime q=1048573(2^20-3)  m=2 bw=1048572 t=5"},
}

func BenchmarkQueryGenPLDN2D(b *testing.B) {
	const noiseCount = 128
	for _, p := range paramsPLDN2D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(p.q * p.q)
				start := time.Now()
				var points []int
				if p.gf {
					points = utils.GenerateCurvePointsGF2n(gf, p.t, target)
				} else {
					points = utils.GenerateCurvePoints(p.t, p.q, target)
				}
				pir.PrepareQuerySequenceWithNoise(points, noiseCount)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-65s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

func BenchmarkDecodePLDN2D(b *testing.B) {
	const noiseCount = 128
	for _, p := range paramsPLDN2D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(2)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			var realPts []int
			if p.gf {
				realPts = utils.GenerateCurvePointsGF2n(gf, p.t, rng.Intn(p.q*p.q))
			} else {
				realPts = utils.GenerateCurvePoints(p.t, p.q, rng.Intn(p.q*p.q))
			}
			allPermuted, permReal := pir.PrepareQuerySequenceWithNoise(realPts, noiseCount)

			mockResp := make([]int, len(allPermuted))
			for i := range mockResp {
				mockResp[i] = rng.Intn(p.q)
			}

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				realSet := make(map[int]int, len(permReal))
				for j, rp := range permReal {
					realSet[rp] = j
				}
				serverValsAtReal := make([]int, len(permReal))
				for k, pos := range allPermuted {
					if j, ok := realSet[pos]; ok {
						serverValsAtReal[j] = mockResp[k]
					}
				}
				if p.gf {
					pir.DecodeLiftedGF2n(serverValsAtReal, permReal)
				} else {
					pir.DecodeLifted(serverValsAtReal, permReal)
				}
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-65s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── PLDN m=3, q=2^13, t=7 ───────────────────────────────────────────────────

var paramsPLDN3D_q13_t7 = []struct {
	q, t  int
	gf    bool
	label string
}{
	{8191, 7, false, "prime q=8191   (2^13-1)  m=3 bw=8190   t=7"},
	{8192, 7, true, "GF(2^13) q=8192    m=3 bw=8191    t=7"},
}

func BenchmarkQueryGenPLDN3D_q13_t7(b *testing.B) {
	const noiseCount = 128
	for _, p := range paramsPLDN3D_q13_t7 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			dbSize := p.q * p.q * p.q

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(dbSize)
				start := time.Now()
				var points []int
				if p.gf {
					points = utils.GenerateCurvePointsGF2n3D(gf, p.t, target)
				} else {
					points = utils.GenerateCurvePoints3D(p.t, p.q, target)
				}
				pir.PrepareQuerySequenceWithNoise(points, noiseCount)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-65s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

func BenchmarkDecodePLDN3D_q13_t7(b *testing.B) {
	const noiseCount = 128
	for _, p := range paramsPLDN3D_q13_t7 {
		p := p
		b.Run(p.label, func(b *testing.B) {
			var gf *utils.GF2n
			if p.gf {
				n := bits.Len(uint(p.q)) - 1
				gf = utils.NewGF2n(n)
			}
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			dbSize := p.q * p.q * p.q

			var realPts []int
			if p.gf {
				realPts = utils.GenerateCurvePointsGF2n3D(gf, p.t, rng.Intn(dbSize))
			} else {
				realPts = utils.GenerateCurvePoints3D(p.t, p.q, rng.Intn(dbSize))
			}
			allPermuted, permReal := pir.PrepareQuerySequenceWithNoise(realPts, noiseCount)

			mockResp := make([]int, len(allPermuted))
			for i := range mockResp {
				mockResp[i] = rng.Intn(p.q)
			}

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				realSet := make(map[int]int, len(permReal))
				for j, rp := range permReal {
					realSet[rp] = j
				}
				serverValsAtReal := make([]int, len(permReal))
				for k, pos := range allPermuted {
					if j, ok := realSet[pos]; ok {
						serverValsAtReal[j] = mockResp[k]
					}
				}
				if p.gf {
					pir.DecodeLiftedGF2n(serverValsAtReal, permReal)
				} else {
					pir.DecodeLifted(serverValsAtReal, permReal)
				}
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-65s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// ─── Concatenated RM correctness test ────────────────────────────────────────

// TestConcRMDecode verifies the full φ⁻¹ + outer-Lagrange decode pipeline.
// Two sub-tests per field size:
//   "identity" — db[p]=p; PhiInv must recover ψ(zⱼ) exactly.
//   "linear"   — db[p]=a·p⊕b; exercises a nontrivial (but degree-1) database so
//                that f∘ψ still has degree t and the t+1 outer Lagrange is exact.
func TestConcRMDecode(t *testing.T) {
	cases := []struct{ n, degree int }{
		{8, 3},
		{13, 7},
		{14, 6},
	}
	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("n=%d_t=%d", c.n, c.degree), func(t *testing.T) {
			gf := utils.NewGF2n(c.n)
			gf2 := utils.NewGF2nExt2(gf)
			q := int(gf.Q)
			rng := rand.New(rand.NewSource(42))

			for trial := 0; trial < 10; trial++ {
				target := uint32(rng.Intn(q))
				zero := utils.GFExt2Elem{}
				psi := gf2.RandPolyWithValue(c.degree, zero, utils.GFExt2Elem{A: target, B: 0})

				zvals := gf2.RandDistinctNonzero(c.degree + 1)
				weights := gf2.LagrangeWeightsAt0(zvals)
				aux := utils.ConcAux{LagrangeWeights: weights, ZVals: zvals}

				buildBlocks := func(db func(uint32) uint32) [][]uint32 {
					blocks := make([][]uint32, c.degree+1)
					for j := 0; j <= c.degree; j++ {
						v := gf2.EvalPoly(psi, zvals[j])
						blocks[j] = make([]uint32, q)
						for w := 0; w < q; w++ {
							blocks[j][w] = db(gf2.Phi(v, uint32(w)))
						}
					}
					return blocks
				}

				// identity database: db[p] = p
				if got := utils.DecodeConcRM(gf2, buildBlocks(func(p uint32) uint32 { return p }), aux); got != target {
					t.Errorf("identity trial %d: want %d got %d (n=%d t=%d)", trial, target, got, c.n, c.degree)
				}

				// linear database: db[p] = a·p ⊕ b  (f∘ψ has degree t → t+1 points exact)
				a := uint32(rng.Intn(q-1)) + 1 // nonzero
				b := uint32(rng.Intn(q))
				want := gf.Mul(a, target) ^ b
				if got := utils.DecodeConcRM(gf2, buildBlocks(func(p uint32) uint32 { return gf.Mul(a, p) ^ b }), aux); got != want {
					t.Errorf("linear trial %d: want %d got %d (n=%d t=%d)", trial, want, got, c.n, c.degree)
				}
			}
		})
	}
}

// TestConcRMDecodeHighDegree verifies that decoding a genuine degree-d RM
// codeword needs s = dt+1 outer points, not t+1. The codeword restricted to the
// curve, g = f∘ψ, has degree d·t; interpolating it at 0 from only t+1 points
// gives the wrong answer, while s = dt+1 points recover f(i) exactly.
func TestConcRMDecodeHighDegree(t *testing.T) {
	gf := utils.NewGF2n(8)
	gf2 := utils.NewGF2nExt2(gf)
	q := int(gf.Q)
	d := q - 1
	curveDeg := 3        // t
	gDeg := d * curveDeg // deg(g) = d·t
	s := gDeg + 1
	rng := rand.New(rand.NewSource(7))

	// Random degree-gDeg g over GF(q²) modelling f∘ψ, with g(0) = f(i) ∈ Fq.
	g := make([]utils.GFExt2Elem, gDeg+1)
	for i := range g {
		g[i] = utils.GFExt2Elem{A: uint32(rng.Intn(q)), B: uint32(rng.Intn(q))}
	}
	g[0].B = 0
	want := g[0].A

	zvals := gf2.RandDistinctNonzero(s)

	// blocks[j][w] = φ(g(zⱼ), w); PhiInv then recovers g(zⱼ).
	buildBlocks := func(n int) [][]uint32 {
		blocks := make([][]uint32, n)
		for j := 0; j < n; j++ {
			u := gf2.EvalPoly(g, zvals[j])
			blocks[j] = make([]uint32, q)
			for w := 0; w < q; w++ {
				blocks[j][w] = gf2.Phi(u, uint32(w))
			}
		}
		return blocks
	}

	// s = dt+1 points recover f(i) exactly.
	auxFull := utils.ConcAux{LagrangeWeights: gf2.LagrangeWeightsAt0(zvals), ZVals: zvals}
	if got := utils.DecodeConcRM(gf2, buildBlocks(s), auxFull); got != want {
		t.Fatalf("s=%d points: want %d got %d", s, want, got)
	}

	// t+1 points are insufficient for a degree-d·t polynomial.
	short := zvals[:curveDeg+1]
	auxShort := utils.ConcAux{LagrangeWeights: gf2.LagrangeWeightsAt0(short), ZVals: short}
	if got := utils.DecodeConcRM(gf2, buildBlocks(curveDeg+1), auxShort); got == want {
		t.Errorf("t+1=%d points unexpectedly decoded correctly for degree-%d g", curveDeg+1, gDeg)
	}
}

// ─── Concatenated RM m=3 benchmarks (Table 2 last column) ───────────────────
//
// Parameters follow Fig. 4 with d = q-1, e = 2:
//   r = q,  s = (q-1)·t + 1,  ℓ = s·q  (total query positions)
//
// Query-gen timing includes:
//   1. Sampling m degree-t polynomials over GF(q²)          O(m·t)
//   2. Evaluating them at s points in GF(q²)                O(m·t·s)
//   3. Streaming all ℓ = s·q query indices (checksum only)  O(m·s·q)  ← dominates
//   4. Lagrange weight precomp over all s points            O(s²)
//
// The codeword restricted to the curve, g = f∘ψ, has degree d·t, so all
// s = dt+1 evaluations are required to interpolate g(0) = f(i).
//
// Decode timing:
//   1. φ⁻¹ step (O(q) scalar-GF(q²) muls per block, s blocks)   O(q·s)  ← dominates
//   2. Outer Lagrange weighted sum over s points                  O(s)

var paramsConcRM3D = []struct {
	n, t  int
	label string
}{
	// Smallest Table 2 entry — benchmarkable in reasonable time (~seconds).
	{13, 7, "GF(2^13) q=8192  m=3 t=7  s=57344  ℓ=470M"},
	// Larger entries — query gen dominated by O(s²) precomp and O(s·q) output.
	{14, 6, "GF(2^14) q=16384 m=3 t=6  s=98299  ℓ=1.6B"},
	{16, 5, "GF(2^16) q=65536 m=3 t=5  s=327676 ℓ=21.5B"},
	{16, 6, "GF(2^16) q=65536 m=3 t=6  s=393211 ℓ=25.8B"},
	// Table 2 new parameter selections.
	{12, 6, "GF(2^12) q=4096  m=3 t=6  new"},
	{13, 6, "GF(2^13) q=8192  m=3 t=6  new"},
	{12, 4, "GF(2^12) q=4096  m=3 t=4  new"},
	{13, 4, "GF(2^13) q=8192  m=3 t=4  new"},
	{14, 4, "GF(2^14) q=16384 m=3 t=4  new"},
	{16, 4, "GF(2^16) q=65536 m=3 t=4  new"},
}

// BenchmarkQueryGenConcRM3D measures Conc. RM query generation.
// Includes: GF(q²) evaluations, φ-projection checksum, and O(t²) Lagrange precomp.
func BenchmarkQueryGenConcRM3D(b *testing.B) {
	for _, p := range paramsConcRM3D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			gf := utils.NewGF2n(p.n)
			gf2 := utils.NewGF2nExt2(gf)
			q := int(gf.Q)
			dbSize := q * q * q
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(dbSize)
				start := time.Now()
				_, _, _ = utils.GenerateConcCurvePoints3D(gf2, p.t, target)
				total += time.Since(start)
			}
			b.StopTimer()
			s := (q-1)*p.t + 1
			ell := s * q
			fmt.Printf("  %-65s query gen: %v  (s=%d ℓ=%d)\n", p.label, total/time.Duration(b.N), s, ell)
		})
	}
}

// BenchmarkDecodeConcRM3D measures the full Conc. RM decode:
// φ⁻¹ (O(q·s) scalar-GF(q²) ops) + outer Lagrange at 0 (O(s) GF(q²) ops),
// where s = dt+1 since g = f∘ψ has degree d·t.
//
// The O(s²) Lagrange-weight precompute is one-time client setup (amortizable
// across queries, since the z-values can be fixed), not part of per-query decode.
// Field multiplies are constant-time in their operands, so decode timing does not
// depend on the weight/z values — we fill length-s buffers directly to isolate the
// per-query decode cost and keep GF(2^16) feasible.
func BenchmarkDecodeConcRM3D(b *testing.B) {
	for _, p := range paramsConcRM3D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			gf := utils.NewGF2n(p.n)
			gf2 := utils.NewGF2nExt2(gf)
			q := int(gf.Q)
			s := (q-1)*p.t + 1 // s = dt+1 with d = q-1
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			// Length-s weight and z-value buffers (values irrelevant to timing).
			weights := make([]utils.GFExt2Elem, s)
			for j := range weights {
				weights[j] = utils.GFExt2Elem{A: uint32(rng.Intn(q)), B: uint32(rng.Intn(q))}
			}
			zvals := make([]utils.GFExt2Elem, s)
			aux := utils.ConcAux{LagrangeWeights: weights, ZVals: zvals}

			// Mock server responses: q values per block, s blocks. All s blocks
			// alias one q-sized buffer so memory stays O(q) instead of O(ℓ)=O(s·q),
			// which would be tens–hundreds of GB for GF(2^16). The decode arithmetic
			// (s φ⁻¹ passes + s Lagrange muls) is unaffected by the aliasing.
			block := make([]uint32, q)
			for w := range block {
				block[w] = uint32(rng.Intn(q))
			}
			yBlocks := make([][]uint32, s)
			for j := range yBlocks {
				yBlocks[j] = block
			}

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				utils.DecodeConcRM(gf2, yBlocks, aux)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-65s decode:    %v  (φ⁻¹+Lagrange, s=%d blocks × q=%d)\n",
				p.label, total/time.Duration(b.N), s, q)
		})
	}
}

// BenchmarkQueryGen3D measures just the query-generation step for 3D databases.
func BenchmarkQueryGen3D(b *testing.B) {
	for _, p := range onlineParams3D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				target := rng.Intn(int(math.Pow(float64(p.q), 3)))
				start := time.Now()
				points := utils.GenerateCurvePoints3D(p.t, p.q, target)
				pir.PrepareQuerySequence(points)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-45s query gen: %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// BenchmarkDecode3D measures just the decode step for 3D databases.
func BenchmarkDecode3D(b *testing.B) {
	for _, p := range onlineParams3D {
		p := p
		b.Run(p.label, func(b *testing.B) {
			pir := NewPIR(Params{Q: uint64(p.q), K: uint8(p.t), M: uint8(3)})
			pir.Gen()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			mockSum := []int{rng.Intn(p.q)}

			points := utils.GenerateCurvePoints3D(p.t, p.q, rng.Intn(int(math.Pow(float64(p.q), 3))))
			query := pir.PrepareQuerySequence(points)

			var total time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				pir.Decode(mockSum, query)
				total += time.Since(start)
			}
			b.StopTimer()
			fmt.Printf("  %-45s decode:    %v\n", p.label, total/time.Duration(b.N))
		})
	}
}

// BenchmarkClientComputation is the legacy single-point benchmark kept for comparison.
func BenchmarkClientComputation(b *testing.B) {
	q := 7919
	k := 5
	m := 3
	pir := NewPIR(Params{Q: uint64(q), K: uint8(k), M: uint8(m)})
	pir.Gen()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var total time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := rng.Intn(q * q)
		start := time.Now()
		points := utils.GenerateCurvePoints3D(k, q, target)
		query := pir.PrepareQuerySequence(points)
		pir.Decode([]int{q - 1}, query)
		total += time.Since(start)
	}
	b.StopTimer()
	fmt.Printf("Average time per client computation: %v, b.N=%d\n", total/time.Duration(b.N), b.N)
}

func BenchmarkCodewordPermutation(b *testing.B) {
	q := 2039
	k := 2
	m := 2
	p := NewPIR(Params{Q: uint64(q), K: uint8(k), M: uint8(m)})
	p.Gen()
	rmc := make([][]int, q)
	for i := 0; i < q; i++ {
		rmc[i] = make([]int, q)
		for j := 0; j < q; j++ {
			rmc[i][j] = rand.Intn(q)
		}
	}
	var total time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		p.permuteAndEncryptMatrix(0, &Matrix2D{data: rmc, q: q})
		total += time.Since(start)
	}
	b.StopTimer()
	fmt.Printf("Average time for permute RMC of size %d: %v, b.N=%d\n", q*q, total/time.Duration(b.N), b.N)
}

// *************************************************************************************
//
//	Functional Tests
//
// *************************************************************************************
func TestEndToEnd(b *testing.T) {
	q := 31
	k := 2
	pir := NewPIR(Params{Q: uint64(q), K: uint8(k), M: uint8(2)})
	pir.Gen()
	input := "../input/db.csv"
	output := "../output/matrix.csv"
	FakeDB(q, 6, input)

	pir.Encode(input, output)
	ori, _ := utils.ReadMatrixFromFile("../output/inter.csv")

	for i := 0; i < q*q; i++ {
		sum, query := pir.QueryLocal(i, []string{output})
		dec := pir.Decode(sum, query)
		row, col := utils.SingleIndexToRowCol(q, i)
		if ori[row][col] != dec[0] {
			panic("Decoding ERROR")
		}
	}
}

func TestEndToEndFromConfigKey(b *testing.T) {
	q := 31
	k := 2
	pir := NewPIR(Params{Q: uint64(q), K: uint8(k), M: uint8(2)})
	pir.GenFromConfig("../config.json")
	input := "../input/example_db.csv"
	output := "../output/example_matrix.csv"
	FakeDB(q, 10, input)

	pir.Encode(input, output)
	ori, _ := utils.ReadMatrixFromFile("../output/inter.csv")

	for i := 0; i < q*q; i++ {
		sum, query := pir.QueryLocal(i, []string{output})
		dec := pir.Decode(sum, query)
		row, col := utils.SingleIndexToRowCol(q, i)
		if ori[row][col] != dec[0] {
			panic("Decoding ERROR")
		}
	}
}

func TestQueryFromServer(b *testing.T) {
	q := 31
	k := 2
	m := 2
	pir := NewPIR(Params{Q: uint64(q), K: uint8(k), M: uint8(m)})
	configFilename := "../config.json"
	pir.GenFromConfig(configFilename)

	if !checkServer(utils.GetParameterConfig(configFilename).ServerUrl) {
		b.Skip("Skipping test: server is not reachable")
	}

	for i := 0; i < q*q; i++ {
		sum, dec_sum := pir.Query(i, utils.GetParameterConfig(configFilename).ServerUrl)
		pir.Decode(sum, dec_sum)
	}
}

func checkServer(url string) bool {
	client := http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
