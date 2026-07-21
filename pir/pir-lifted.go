package pir

import (
	"fmt"
	"math/bits"

	"rme/utils"
)

// LiftedRS implements PIR using the lifted Reed-Solomon construction.
type LiftedRS struct {
	*pirCore
	encoder *RMEncoder
}

// NewLiftedRS creates a lifted Reed-Solomon PIR instance.
func NewLiftedRS(params Params) *LiftedRS {
	return &LiftedRS{
		pirCore: &pirCore{params: params},
		encoder: NewRMEncoder(),
	}
}

var _ PIR = (*LiftedRS)(nil)

// Encode uses RMEncoding as a testing placeholder for the unavailable
// Lifted-RS encoder. This validates query and decode correctness, but does not
// represent Lifted-RS storage rate or encoding performance.
func (p *LiftedRS) Encode(input, output string) (Matrix, error) {
	return p.encoder.Encode(input, output, p.params, p.pirCore)
}

// Close stops the RMEncoding process if it was started.
func (p *LiftedRS) Close() error {
	return p.encoder.Close()
}

// GenerateClientQuery creates the client-side lifted-RS query and its private real positions.
func (p *LiftedRS) GenerateClientQuery(i int, noiseCount int) ClientQuery {
	if i < 0 || i >= databaseSize(p.params) {
		panic(fmt.Sprintf("lifted-RS query index %d is outside database of size %d", i, databaseSize(p.params)))
	}
	if !isPowerOfTwo(p.params.Q) && !isPrime(p.params.Q) {
		panic(fmt.Sprintf("lifted-RS query generation supports prime q or the implemented binary extension fields, got q=%d", p.params.Q))
	}
	var points []int
	switch p.params.M {
	case 2:
		if isPowerOfTwo(p.params.Q) {
			gf := utils.NewGF2n(bits.Len64(p.params.Q) - 1)
			points = utils.GenerateCurvePointsGF2n(gf, int(p.params.K), i)
		} else {
			points = utils.GenerateCurvePoints(int(p.params.K), int(p.params.Q), i)
		}
	case 3:
		if isPowerOfTwo(p.params.Q) {
			gf := utils.NewGF2n(bits.Len64(p.params.Q) - 1)
			points = utils.GenerateCurvePointsGF2n3D(gf, int(p.params.K), i)
		} else {
			points = utils.GenerateCurvePoints3D(int(p.params.K), int(p.params.Q), i)
		}
	default:
		panic(fmt.Sprintf("lifted-RS query generation does not support dimension %d", p.params.M))
	}
	query, realPositions := p.PrepareQuerySequenceWithNoise(points, noiseCount)
	return ClientQuery{Query: query, RealPositions: realPositions}
}

// Query generates a client query, fetches the server response, and returns the full result bundle.
func (p *LiftedRS) Query(i int, url string) QueryResult {
	clientQuery := p.GenerateClientQuery(i, 128)
	response := p.fetchServerResponse(clientQuery.Query, url)
	return QueryResult{Values: response, Query: clientQuery.Query, RealPositions: clientQuery.RealPositions}
}

// QueryLocal evaluates a Lift-RS query against a local database file.
// It is intended for tests and benchmarks that do not run an HTTP server.
func (p *LiftedRS) QueryLocal(i int, filename string) QueryResult {
	clientQuery := p.GenerateClientQuery(i, 128)
	var response []int
	if p.params.M == 3 {
		response = p.queryLocalDB3DValues(clientQuery.Query, filename)
	} else {
		response = p.queryLocalDBValues(clientQuery.Query, filename)
	}
	return QueryResult{
		Values:        response,
		Query:         clientQuery.Query,
		RealPositions: clientQuery.RealPositions,
	}
}

// QueryLocalLiftedRS is the legacy tuple-returning local-query helper.
// New code should use QueryLocal.
func (p *LiftedRS) QueryLocalLiftedRS(i int, filename string) ([]int, []int, []int) {
	result := p.QueryLocal(i, filename)
	return result.Values, result.Query, result.RealPositions
}

// Decode reconstructs the requested value from a query result.
func (p *LiftedRS) Decode(result QueryResult) ([]int, error) {
	if len(result.RealPositions) == 0 {
		return nil, fmt.Errorf("lifted-RS result has no real positions")
	}
	decoded, err := p.DecodeLiftedRS(result.Values, result.Query, result.RealPositions)
	if err != nil {
		return nil, err
	}
	return []int{decoded}, nil
}

// DecodeLiftedRS decodes the lifted-RS response using the server values and the private real positions.
func (p *LiftedRS) DecodeLiftedRS(serverValues []int, query []int, realPositions []int) (int, error) {
	return p.decodeIndividualResponse(serverValues, query, realPositions)
}

// DecodeLifted decodes a single-slice lifted-RS response using the private real positions.
func (pir *LiftedRS) DecodeLifted(serverValuesAtReal []int, permutedReal []int) int {
	q := int(pir.params.Q)
	var acc int64
	for j, po := range permutedReal {
		acc += int64(pir.encryptor.EncryptPosition(0, po)) - int64(serverValuesAtReal[j])
	}
	return int(((acc % int64(q)) + int64(q)) % int64(q))
}

// DecodeLiftedGF2n decodes a GF(2^n) lifted-RS response using XOR-based masking.
func (pir *LiftedRS) DecodeLiftedGF2n(serverValuesAtReal []int, permutedReal []int) int {
	acc := 0
	for j, po := range permutedReal {
		acc ^= pir.encryptor.Decrypt(po, serverValuesAtReal[j])
	}
	return acc
}
