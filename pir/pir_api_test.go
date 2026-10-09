package pir

import (
	"os"
	"path/filepath"
	"testing"

	"rme/utils"
)

func TestLiftedRSDecodeThroughPIRInterface(t *testing.T) {
	const q = 31
	var scheme PIR = NewLiftedRS(Params{Q: q, K: 2, M: 2})
	scheme.Gen()

	lifted := scheme.(*LiftedRS)
	query := []int{3, 7}
	want := 11

	values := []int{
		lifted.encryptor.Encrypt(query[0], 5),
		lifted.encryptor.Encrypt(query[1], q-want-5),
	}
	got, err := scheme.Decode(QueryResult{Values: values, Query: query, RealPositions: query})
	if err != nil {
		t.Fatalf("Decode() returned error: %v", err)
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("Decode() = %v, want [%d]", got, want)
	}
}

func TestPIRDecodeRejectsMissingRealPositions(t *testing.T) {
	schemes := map[string]PIR{
		"LiftedRS": NewLiftedRS(Params{Q: 31, K: 2, M: 2}),
		"RMConc":   NewRMConc(Params{Q: 16, K: 2, M: 3}),
	}
	for name, scheme := range schemes {
		t.Run(name, func(t *testing.T) {
			if _, err := scheme.Decode(QueryResult{Values: []int{1}, Query: []int{1}}); err == nil {
				t.Fatal("Decode() returned nil error without real positions")
			}
		})
	}
}

func TestPIRRMEncodingRequiresKeys(t *testing.T) {
	schemes := map[string]PIR{
		"LiftedRS": NewLiftedRS(Params{Q: 31, K: 2, M: 2}),
		"RMConc":   NewRMConc(Params{Q: 16, K: 2, M: 3}),
	}
	for name, scheme := range schemes {
		t.Run(name, func(t *testing.T) {
			if _, err := scheme.Encode("unused-input", "unused-output"); err == nil {
				t.Fatal("Encode() returned nil error before key generation")
			}
		})
	}
}

func TestRMConcDecodeThroughPIRInterface(t *testing.T) {
	const (
		n           = 4
		q           = 1 << n
		curveDegree = 2
		rmDegree    = 3
		target      = 7
	)
	gf := utils.NewGF2n(n)
	gf2 := utils.NewGF2nExt2(gf)
	// Use an exact degree-t curve so f∘ψ has exact degree d*t rather than
	// relying on random leading coefficients in this regression test.
	psi := []utils.GFExt2Elem{
		{A: target},
		{A: 1},
		{A: 1, B: 1},
	}
	blockCount := rmDegree*curveDegree + 1
	innerBlockSize := rmDegree + 1
	zValues := gf2.RandDistinctNonzero(blockCount)
	wValues := make([][]uint32, blockCount)
	aux := utils.ConcAux{
		LagrangeWeights: gf2.LagrangeWeightsAt0(zValues),
		ZVals:           zValues,
		WValues:         wValues,
	}

	scheme := NewRMConc(Params{Q: q, K: curveDegree, M: 3, D: rmDegree})
	scheme.Gen()
	positions := make([]int, blockCount*innerBlockSize)
	values := make([]int, len(positions))
	// This polynomial has exact degree d=3, so f∘ψ may have degree d*t=6.
	// A decoder that retained only t+1 blocks would fail this test in general.
	coefficients := []uint32{3, 5, 0, 1}
	evalRM := func(x uint32) uint32 {
		result := coefficients[len(coefficients)-1]
		for j := len(coefficients) - 2; j >= 0; j-- {
			result = gf.Mul(result, x) ^ coefficients[j]
		}
		return result
	}
	for block := 0; block < blockCount; block++ {
		v := gf2.EvalPoly(psi, zValues[block])
		wValues[block] = gf2.RandDistinctBaseElements(innerBlockSize)
		for h, w := range wValues[block] {
			i := block*innerBlockSize + h
			positions[i] = i
			plain := int(evalRM(gf2.Phi(v, w)))
			values[i] = scheme.encryptor.Encrypt(i, plain)
		}
	}

	var generic PIR = scheme
	got, err := generic.Decode(QueryResult{
		Values:        values,
		Query:         positions,
		RealPositions: positions,
		Auxiliary:     aux,
	})
	if err != nil {
		t.Fatalf("Decode() returned error: %v", err)
	}
	want := int(evalRM(target))
	if len(got) != 1 || got[0] != want {
		t.Fatalf("Decode() = %v, want [%d]", got, want)
	}
}

func TestRMConcEndToEndWithGF2nCodeword(t *testing.T) {
	const (
		n           = 4
		q           = 1 << n
		curveDegree = 2
		rmDegree    = 2
	)
	gf := utils.NewGF2n(n)
	// A genuine total-degree-2 RM polynomial over GF(16). Keeping this
	// polynomial explicit makes the expected value independent of the query
	// and decoding implementations under test.
	evalCodeword := func(x, y, z uint32) uint32 {
		return 7 ^
			gf.Mul(3, x) ^ gf.Mul(5, y) ^ gf.Mul(9, z) ^
			gf.Mul(11, gf.Mul(x, x)) ^
			gf.Mul(13, gf.Mul(x, y)) ^
			gf.Mul(6, gf.Mul(y, z))
	}

	codeword := make([][][]int, q)
	for x := 0; x < q; x++ {
		codeword[x] = make([][]int, q)
		for y := 0; y < q; y++ {
			codeword[x][y] = make([]int, q)
			for z := 0; z < q; z++ {
				codeword[x][y][z] = int(evalCodeword(uint32(x), uint32(y), uint32(z)))
			}
		}
	}

	scheme := NewRMConc(Params{Q: q, K: curveDegree, M: 3, D: rmDegree})
	scheme.Gen()
	encoded := &Matrix3D{data: codeword, q: q}
	if err := scheme.permuteAndEncryptMatrix(0, encoded); err != nil {
		t.Fatalf("permute and mask RM codeword: %v", err)
	}
	serverDatabase := filepath.Join(t.TempDir(), "rmconc-encoded.csv")
	if err := utils.Write3DMatrixToFile(codeword, serverDatabase); err != nil {
		t.Fatalf("write encoded server database: %v", err)
	}

	indices := []int{0, 1, 7, q*q + 3*q + 5, q*q*q - 1}
	for _, index := range indices {
		result := scheme.QueryLocalConcRM(index, serverDatabase)
		got, err := scheme.Decode(result)
		if err != nil {
			t.Fatalf("index %d: Decode() returned error: %v", index, err)
		}
		x, y, z := utils.SingleIndexToRowCol3D(q, index)
		want := int(evalCodeword(uint32(x), uint32(y), uint32(z)))
		if len(got) != 1 || got[0] != want {
			t.Fatalf("index %d: Decode() = %v, want [%d]", index, got, want)
		}
	}
}

func TestLiftedRSEndToEndWithGF2nCodeword(t *testing.T) {
	const (
		n           = 4
		q           = 1 << n
		curveDegree = 2
	)
	gf := utils.NewGF2n(n)
	// A total-degree-2 polynomial has degree at most four on a degree-2
	// query curve, below q-1. The Lift-RS line-sum identity therefore applies.
	evalCodeword := func(x, y uint32) uint32 {
		return 7 ^ gf.Mul(3, x) ^ gf.Mul(5, y) ^
			gf.Mul(11, gf.Mul(x, x)) ^ gf.Mul(13, gf.Mul(x, y))
	}

	codeword := make([][]int, q)
	for x := 0; x < q; x++ {
		codeword[x] = make([]int, q)
		for y := 0; y < q; y++ {
			codeword[x][y] = int(evalCodeword(uint32(x), uint32(y)))
		}
	}

	scheme := NewLiftedRS(Params{Q: q, K: curveDegree, M: 2})
	scheme.Gen()
	encoded := &Matrix2D{data: codeword, q: q}
	if err := scheme.permuteAndEncryptMatrix(0, encoded); err != nil {
		t.Fatalf("permute and mask Lift-RS codeword: %v", err)
	}
	serverDatabase := filepath.Join(t.TempDir(), "lift-rs-encoded.csv")
	if err := utils.Write2DMatrixToFile(codeword, serverDatabase); err != nil {
		t.Fatalf("write encoded server database: %v", err)
	}

	indices := []int{0, 1, 7, 3*q + 5, q*q - 1}
	for _, index := range indices {
		values, query, realPositions := scheme.QueryLocalLiftedRS(index, serverDatabase)
		got, err := scheme.Decode(QueryResult{Values: values, Query: query, RealPositions: realPositions})
		if err != nil {
			t.Fatalf("index %d: Decode() returned error: %v", index, err)
		}
		x, y := utils.SingleIndexToRowCol(q, index)
		want := int(evalCodeword(uint32(x), uint32(y)))
		if len(got) != 1 || got[0] != want {
			t.Fatalf("index %d: Decode() = %v, want [%d]", index, got, want)
		}
	}
}

func TestRMConcDefaultsDegreeToQMinusTwo(t *testing.T) {
	scheme := NewRMConc(Params{Q: 16, K: 2, M: 3})
	if scheme.params.D != 14 {
		t.Fatalf("default RM degree = %d, want 14", scheme.params.D)
	}
}

func TestLiftedRSRejectsUnsupportedCompositeFieldSize(t *testing.T) {
	scheme := NewLiftedRS(Params{Q: 15, K: 2, M: 2})
	scheme.Gen()
	defer func() {
		if recover() == nil {
			t.Fatal("GenerateClientQuery accepted composite q=15")
		}
	}()
	scheme.GenerateClientQuery(0, 0)
}

func TestRMConcQueryUsesConfiguredDegree(t *testing.T) {
	const (
		q           = 16
		curveDegree = 2
		rmDegree    = 3
	)
	scheme := NewRMConc(Params{Q: q, K: curveDegree, M: 3, D: rmDegree})
	scheme.Gen()
	query := scheme.GenerateConcCurveClientQuery(7, 0)
	wantBlocks := rmDegree*curveDegree + 1
	wantInnerBlockSize := rmDegree + 1
	wantPositions := wantBlocks * wantInnerBlockSize
	if len(query.RealPositions) != wantPositions || len(query.Query) != wantPositions {
		t.Fatalf("query has %d real and %d total positions, want %d of each", len(query.RealPositions), len(query.Query), wantPositions)
	}
	if len(query.Auxiliary.ZVals) != wantBlocks || len(query.Auxiliary.LagrangeWeights) != wantBlocks {
		t.Fatalf("query has %d points and %d weights, want %d of each", len(query.Auxiliary.ZVals), len(query.Auxiliary.LagrangeWeights), wantBlocks)
	}
	if len(query.Auxiliary.WValues) != wantBlocks {
		t.Fatalf("query has %d inner interpolation-point blocks, want %d", len(query.Auxiliary.WValues), wantBlocks)
	}
	for block, values := range query.Auxiliary.WValues {
		if len(values) != wantInnerBlockSize {
			t.Fatalf("inner interpolation-point block %d has %d values, want %d", block, len(values), wantInnerBlockSize)
		}
		seen := make(map[uint32]bool, len(values))
		for _, value := range values {
			if value >= q || seen[value] {
				t.Fatalf("inner interpolation-point block %d contains invalid or repeated value %d", block, value)
			}
			seen[value] = true
		}
	}
	gf2 := utils.NewGF2nExt2(utils.NewGF2n(4))
	wantWeights := gf2.LagrangeWeightsAt0(query.Auxiliary.ZVals)
	for i := range wantWeights {
		if query.Auxiliary.LagrangeWeights[i] != wantWeights[i] {
			t.Fatalf("query interpolation weight %d = %+v, want %+v", i, query.Auxiliary.LagrangeWeights[i], wantWeights[i])
		}
	}
}

func TestRMConcQueryUsesFullOuterFieldShortcut(t *testing.T) {
	const (
		q           = 16
		curveDegree = 127
		rmDegree    = 2
	)
	// d*t+1 = 255 = q²-1, so every nonzero outer point is used.
	scheme := NewRMConc(Params{Q: q, K: curveDegree, M: 3, D: rmDegree})
	scheme.Gen()
	query := scheme.GenerateConcCurveClientQuery(7, 0)
	wantBlocks := q*q - 1
	wantPositions := wantBlocks * (rmDegree + 1)
	if !query.Auxiliary.FullOuterField {
		t.Fatal("query did not select the full-outer-field interpolation shortcut")
	}
	if len(query.Auxiliary.ZVals) != 0 || len(query.Auxiliary.LagrangeWeights) != 0 {
		t.Fatalf("full-outer-field query retained %d points and %d weights", len(query.Auxiliary.ZVals), len(query.Auxiliary.LagrangeWeights))
	}
	if len(query.RealPositions) != wantPositions || len(query.Query) != wantPositions {
		t.Fatalf("query has %d real and %d total positions, want %d of each", len(query.RealPositions), len(query.Query), wantPositions)
	}
	if len(query.Auxiliary.WValues) != wantBlocks {
		t.Fatalf("query has inner points for %d blocks, want %d", len(query.Auxiliary.WValues), wantBlocks)
	}
}

func TestRMEncodingRejectsMessageAboveConfiguredDegree(t *testing.T) {
	input := filepath.Join(t.TempDir(), "message.csv")
	// RM(3, 3) has 20 monomials; 21 symbols require degree 4.
	if err := os.WriteFile(input, []byte("0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,0,1,2,3,4\n"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	scheme := NewRMConc(Params{Q: 16, K: 2, M: 3, D: 3})
	scheme.Gen()
	if _, err := scheme.Encode(input, filepath.Join(t.TempDir(), "encoded.csv")); err == nil {
		t.Fatal("Encode() accepted a message whose inferred RM degree exceeds D")
	}
}
