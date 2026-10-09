package pir

import (
	"fmt"
	"math/bits"
	"math/rand"

	"rme/utils"
)

// RMConc implements PIR using the RM concatenated-curve construction.
type RMConc struct {
	*pirCore
	encoder *RMEncoder
}

// ConcClientQuery contains the query positions and interpolation data needed
// to decode an RM concatenated-curve response.
type ConcClientQuery struct {
	ClientQuery
	Auxiliary utils.ConcAux
}

// NewRMConc creates an RM concatenated-curve PIR instance.
func NewRMConc(params Params) *RMConc {
	if params.D == 0 && params.Q > 0 {
		params.D = params.Q - 2
	}
	return &RMConc{
		pirCore: &pirCore{params: params},
		encoder: NewRMEncoder(),
	}
}

var _ PIR = (*RMConc)(nil)

// Encode performs the native RMEncoding used by RMConc.
func (p *RMConc) Encode(input, output string) (Matrix, error) {
	return p.encoder.Encode(input, output, p.params, p.pirCore)
}

// Close stops the RMEncoding process if it was started.
func (p *RMConc) Close() error {
	return p.encoder.Close()
}

// Query generates an RM-concatenated client query and fetches its response.
func (p *RMConc) Query(i int, url string) QueryResult {
	clientQuery := p.GenerateConcCurveClientQuery(i, 128)
	response := p.fetchServerResponse(clientQuery.Query, url)
	return QueryResult{
		Values:        response,
		Query:         clientQuery.Query,
		RealPositions: clientQuery.RealPositions,
		Auxiliary:     clientQuery.Auxiliary,
	}
}

// Decode reconstructs the value returned for an RM-concatenated query.
func (p *RMConc) Decode(result QueryResult) ([]int, error) {
	if len(result.RealPositions) == 0 {
		return nil, fmt.Errorf("RM-concatenated result has no real positions")
	}
	aux, ok := result.Auxiliary.(utils.ConcAux)
	if !ok {
		return nil, fmt.Errorf("RM-concatenated result is missing concatenated-curve auxiliary data")
	}
	if len(result.Values) != len(result.Query) {
		return nil, fmt.Errorf("server returned %d values for %d query positions", len(result.Values), len(result.Query))
	}

	if !isPowerOfTwo(p.params.Q) {
		return nil, fmt.Errorf("RM-concatenated decoding requires q=2^n, got %d", p.params.Q)
	}
	blockCount, err := p.concBlockCount()
	if err != nil {
		return nil, err
	}
	fullOuterField := blockCount == int(p.params.Q*p.params.Q-1)
	validOuterAux := aux.FullOuterField == fullOuterField
	if fullOuterField {
		validOuterAux = validOuterAux && len(aux.ZVals) == 0 && len(aux.LagrangeWeights) == 0
	} else {
		validOuterAux = validOuterAux && len(aux.ZVals) == blockCount && len(aux.LagrangeWeights) == blockCount
	}
	if !validOuterAux {
		return nil, fmt.Errorf(
			"RM-concatenated auxiliary data has full-field=%t, %d points, and %d weights; degree d=%d and curve degree t=%d require %d blocks (full-field=%t)",
			aux.FullOuterField, len(aux.ZVals), len(aux.LagrangeWeights), p.params.D, p.params.K, blockCount, fullOuterField,
		)
	}
	innerBlockSize := int(p.params.D) + 1 // r=d(e-1)+1 with extension degree e=2
	if len(result.RealPositions) != blockCount*innerBlockSize {
		return nil, fmt.Errorf("RM-concatenated result has %d real positions; degree d=%d requires exactly %d", len(result.RealPositions), p.params.D, blockCount*innerBlockSize)
	}

	positionToValue := make(map[int]int, len(result.Query))
	for i, position := range result.Query {
		positionToValue[position] = result.Values[i]
	}
	yBlocks := make([][]uint32, blockCount)
	for block := range yBlocks {
		yBlocks[block] = make([]uint32, innerBlockSize)
		for h := 0; h < innerBlockSize; h++ {
			position := result.RealPositions[block*innerBlockSize+h]
			value, found := positionToValue[position]
			if !found {
				return nil, fmt.Errorf("missing server response for real position %d", position)
			}
			yBlocks[block][h] = uint32(p.encryptor.Decrypt(position, value))
		}
	}

	n := bits.Len64(p.params.Q) - 1
	gf2 := utils.NewGF2nExt2(utils.NewGF2n(n))
	decoded, err := utils.DecodeConcRM(gf2, yBlocks, aux)
	if err != nil {
		return nil, fmt.Errorf("decode RM-concatenated response: %w", err)
	}
	return []int{int(decoded)}, nil
}

// QueryLocal evaluates an RM-concatenated query against a local database.
// It is intended for tests and benchmarks that do not run an HTTP server.
func (p *RMConc) QueryLocal(i int, filename string) QueryResult {
	clientQuery := p.GenerateConcCurveClientQuery(i, 128)
	response := p.queryLocalDB3DValues(clientQuery.Query, filename)
	return QueryResult{
		Values:        response,
		Query:         clientQuery.Query,
		RealPositions: clientQuery.RealPositions,
		Auxiliary:     clientQuery.Auxiliary,
	}
}

// QueryLocalConcRM is retained as a compatibility alias for QueryLocal.
func (p *RMConc) QueryLocalConcRM(i int, filename string) QueryResult {
	return p.QueryLocal(i, filename)
}

// GenerateConcCurveClientQuery creates the RM-concatenated client query for a 3D database.
func (p *RMConc) GenerateConcCurveClientQuery(i int, noiseCount int) ConcClientQuery {
	if p.params.M != 3 {
		panic(fmt.Sprintf("RM-concatenated query generation requires dimension 3, got %d", p.params.M))
	}
	if !isPowerOfTwo(p.params.Q) {
		panic(fmt.Sprintf("RM-concatenated query generation requires q=2^n, got %d", p.params.Q))
	}
	if i < 0 || i >= databaseSize(p.params) {
		panic(fmt.Sprintf("RM-concatenated query index %d is outside database of size %d", i, databaseSize(p.params)))
	}

	n := bits.Len64(p.params.Q) - 1
	gf2 := utils.NewGF2nExt2(utils.NewGF2n(n))
	q := int(p.params.Q)
	t := int(p.params.K)
	r := int(p.params.D) + 1 // d(e-1)+1 for e=2
	s, err := p.concBlockCount()
	if err != nil {
		panic(err)
	}

	x := i % q
	y := (i / q) % q
	z := i / (q * q)
	zero := utils.GFExt2Elem{}
	psi := [3][]utils.GFExt2Elem{
		gf2.RandPolyWithValue(t, zero, utils.GFExt2Elem{A: uint32(x)}),
		gf2.RandPolyWithValue(t, zero, utils.GFExt2Elem{A: uint32(y)}),
		gf2.RandPolyWithValue(t, zero, utils.GFExt2Elem{A: uint32(z)}),
	}
	fullOuterField := s == q*q-1
	var zValues []utils.GFExt2Elem
	if !fullOuterField {
		zValues = gf2.RandDistinctNonzero(s)
	}

	points := make([]int, 0, s*r)
	var wValues [][]uint32
	var missingWValues []uint32
	var fullInnerPoints []uint32
	if r == q-1 {
		missingWValues = make([]uint32, s)
	} else if r < q {
		wValues = make([][]uint32, s)
	} else {
		fullInnerPoints = make([]uint32, q)
		for w := range fullInnerPoints {
			fullInnerPoints[w] = uint32(w)
		}
	}
	q2 := q * q
	for j := 0; j < s; j++ {
		zValue := gf2.NonzeroElementAt(j)
		if !fullOuterField {
			zValue = zValues[j]
		}
		v0 := gf2.EvalPoly(psi[0], zValue)
		v1 := gf2.EvalPoly(psi[1], zValue)
		v2 := gf2.EvalPoly(psi[2], zValue)
		if r == q-1 {
			missing := uint32(rand.Intn(q))
			missingWValues[j] = missing
			for ww := uint32(0); ww < gf2.Base.Q; ww++ {
				if ww == missing {
					continue
				}
				c0 := gf2.Phi(v0, ww)
				c1 := gf2.Phi(v1, ww)
				c2 := gf2.Phi(v2, ww)
				points = append(points, int(c2)*q2+int(c1)*q+int(c0))
			}
			continue
		}
		var innerPoints []uint32
		if r == q {
			innerPoints = fullInnerPoints
		} else {
			innerPoints = gf2.RandDistinctBaseElements(r)
			wValues[j] = innerPoints
		}
		for _, ww := range innerPoints {
			c0 := gf2.Phi(v0, ww)
			c1 := gf2.Phi(v1, ww)
			c2 := gf2.Phi(v2, ww)
			points = append(points, int(c2)*q2+int(c1)*q+int(c0))
		}
	}

	query, realPositions := p.PrepareQuerySequenceWithNoise(points, noiseCount)
	if fullOuterField {
		return ConcClientQuery{
			ClientQuery: ClientQuery{Query: query, RealPositions: realPositions},
			Auxiliary:   utils.ConcAux{FullOuterField: true, WValues: wValues, MissingWValues: missingWValues},
		}
	}
	weights, err := gf2.LagrangeWeightsAt0Fast(zValues)
	if err != nil {
		panic(fmt.Errorf("precompute RM-concatenated interpolation weights: %w", err))
	}
	aux := utils.ConcAux{
		LagrangeWeights: weights,
		ZVals:           zValues,
		WValues:         wValues,
		MissingWValues:  missingWValues,
	}
	return ConcClientQuery{
		ClientQuery: ClientQuery{Query: query, RealPositions: realPositions},
		Auxiliary:   aux,
	}
}

// concBlockCount returns the number of outer interpolation points needed for
// an RM polynomial of degree d restricted to a query curve of degree t.
func (p *RMConc) concBlockCount() (int, error) {
	if p.params.Q < 2 {
		return 0, fmt.Errorf("RM-concatenated construction requires q >= 2, got %d", p.params.Q)
	}
	if p.params.D == 0 || p.params.D >= p.params.Q {
		return 0, fmt.Errorf("RM-concatenated degree d must satisfy 1 <= d <= q-1, got d=%d q=%d", p.params.D, p.params.Q)
	}
	s := p.params.D*uint64(p.params.K) + 1
	if s > p.params.Q*p.params.Q-1 {
		return 0, fmt.Errorf("RM-concatenated construction needs %d distinct nonzero points, but GF(q^2) has only %d", s, p.params.Q*p.params.Q-1)
	}
	return int(s), nil
}
