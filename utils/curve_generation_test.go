package utils

import "testing"

func TestPhiInvMissing(t *testing.T) {
	gf := NewGF2n(4)
	gf2 := NewGF2nExt2(gf)
	coefficients := []uint32{3, 5, 7, 9, 11, 13, 2, 4, 6, 8, 10, 12, 14, 15, 1}
	extensionCoefficients := make([]GFExt2Elem, len(coefficients))
	for i, coefficient := range coefficients {
		extensionCoefficients[i] = GFExt2Elem{A: coefficient}
	}
	want := gf2.EvalPoly(extensionCoefficients, GFExt2Elem{B: 1})

	for missing := uint32(0); missing < gf.Q; missing++ {
		values := make([]uint32, 0, gf.Q-1)
		for w := uint32(0); w < gf.Q; w++ {
			if w == missing {
				continue
			}
			value := coefficients[len(coefficients)-1]
			for i := len(coefficients) - 2; i >= 0; i-- {
				value = gf.Mul(value, w) ^ coefficients[i]
			}
			values = append(values, value)
		}

		got, err := gf2.PhiInvMissing(values, missing)
		if err != nil {
			t.Fatalf("missing %d: PhiInvMissing returned error: %v", missing, err)
		}
		if got != want {
			t.Fatalf("missing %d: PhiInvMissing = %+v, want %+v", missing, got, want)
		}
	}
}

func TestGenerateConcCurvePoints3DAddsDefaultNoise(t *testing.T) {
	gf := NewGF2n(13)
	gf2 := NewGF2nExt2(gf)

	const d = 3
	_, gotEll, aux := GenerateConcCurvePoints3D(gf2, 2, d, 7)

	blockCount := d*2 + 1
	innerBlockSize := d + 1
	wantEll := blockCount * innerBlockSize
	if gotEll != wantEll {
		t.Fatalf("GenerateConcCurvePoints3D returned ell=%d, want %d", gotEll, wantEll)
	}
	if len(aux.ZVals) != blockCount || len(aux.LagrangeWeights) != blockCount {
		t.Fatalf("GenerateConcCurvePoints3D returned %d points and %d weights, want %d of each", len(aux.ZVals), len(aux.LagrangeWeights), blockCount)
	}
	if len(aux.WValues) != blockCount {
		t.Fatalf("GenerateConcCurvePoints3D returned %d inner interpolation-point blocks, want %d", len(aux.WValues), blockCount)
	}
	for block, values := range aux.WValues {
		if len(values) != innerBlockSize {
			t.Fatalf("inner interpolation-point block %d has %d values, want %d", block, len(values), innerBlockSize)
		}
	}
	wantWeights := gf2.LagrangeWeightsAt0(aux.ZVals)
	for i := range wantWeights {
		if aux.LagrangeWeights[i] != wantWeights[i] {
			t.Fatalf("GenerateConcCurvePoints3D weight %d = %+v, want %+v", i, aux.LagrangeWeights[i], wantWeights[i])
		}
	}
}

func TestGenerateConcCurvePoints3DCompactsQMinusOneInnerPoints(t *testing.T) {
	gf2 := NewGF2nExt2(NewGF2n(4))
	_, gotEll, aux := GenerateConcCurvePoints3D(gf2, 1, 14, 7)
	if gotEll != 15*15 {
		t.Fatalf("GenerateConcCurvePoints3D returned ell=%d, want %d", gotEll, 15*15)
	}
	if len(aux.WValues) != 0 {
		t.Fatalf("GenerateConcCurvePoints3D retained %d explicit inner-point blocks", len(aux.WValues))
	}
	if len(aux.MissingWValues) != 15 {
		t.Fatalf("GenerateConcCurvePoints3D returned %d omitted inner points, want 15", len(aux.MissingWValues))
	}
	for block, missing := range aux.MissingWValues {
		if missing >= gf2.Base.Q {
			t.Fatalf("omitted inner point %d for block %d is outside GF(%d)", missing, block, gf2.Base.Q)
		}
	}
}

func TestDecodeConcRMFullOuterField(t *testing.T) {
	gf := NewGF2n(4)
	gf2 := NewGF2nExt2(gf)
	blockCount := int(gf.Q)*int(gf.Q) - 1

	// Evaluate a nonconstant polynomial at every nonzero element of GF(q²).
	// The full-field decoder must recover its constant coefficient at zero.
	poly := []GFExt2Elem{{A: 7}, {A: 3, B: 5}, {A: 11, B: 9}}
	yBlocks := make([][]uint32, blockCount)
	seen := make(map[GFExt2Elem]bool, blockCount)
	for j := 0; j < blockCount; j++ {
		z := gf2.NonzeroElementAt(j)
		if z == (GFExt2Elem{}) || seen[z] {
			t.Fatalf("nonzero enumeration returned invalid or repeated element %+v", z)
		}
		seen[z] = true
		u := gf2.EvalPoly(poly, z)
		yBlocks[j] = make([]uint32, gf.Q)
		for w := uint32(0); w < gf.Q; w++ {
			yBlocks[j][w] = gf2.Phi(u, w)
		}
	}

	got, err := DecodeConcRM(gf2, yBlocks, ConcAux{FullOuterField: true})
	if err != nil {
		t.Fatalf("DecodeConcRM full-outer-field path returned error: %v", err)
	}
	if got != poly[0].A {
		t.Fatalf("DecodeConcRM full-outer-field path returned %d, want %d", got, poly[0].A)
	}
}

func TestDecodeConcRMFullOuterFieldRejectsExplicitWeights(t *testing.T) {
	gf2 := NewGF2nExt2(NewGF2n(4))
	_, err := DecodeConcRM(gf2, nil, ConcAux{
		FullOuterField:  true,
		ZVals:           []GFExt2Elem{{A: 1}},
		LagrangeWeights: []GFExt2Elem{{A: 1}},
	})
	if err == nil {
		t.Fatal("full-outer-field decoder accepted redundant explicit weights")
	}
}
