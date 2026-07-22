package pir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"

	"rme/utils"
)

// QueryResult bundles the server response values with the query and private positions.
type QueryResult struct {
	Values        []int
	Query         []int
	RealPositions []int
	Auxiliary     any
}

// ClientQuery captures the shuffled query positions and the private real positions.
type ClientQuery struct {
	Query         []int
	RealPositions []int
}

// PIR defines the common PIR interface used by the protocol implementations.
type PIR interface {
	Gen()
	Encode(string, string) (Matrix, error)
	Query(int, string) QueryResult
	Decode(QueryResult) ([]int, error)
	Close() error
}

const (
	keyLength   = 16
	tweakLength = 8
)

// Response is the JSON payload returned by the server.
type Response struct {
	Values []int `json:"values"`
}

type pirCore struct {
	params     Params
	permutator *utils.Permutator
	encryptor  *utils.Encryptor
}

// NewPIR creates a Lifted-RS instance. It is retained for backwards
// compatibility; new code should use NewLiftedRS or NewRMConc explicitly.
func NewPIR(params Params) *LiftedRS {
	return NewLiftedRS(params)
}

// Gen initializes the permutator and encryptor for the PIR instance.
func (p *pirCore) Gen() {
	key, err := utils.GenerateRandomKey(keyLength)
	if err != nil {
		fmt.Println("Error Generating Permuator Key:", err)
		panic(err)
	}

	tweak, err := utils.GenerateRandomKey(tweakLength)
	if err != nil {
		fmt.Println("Error Generating Permuator Tweak:", err)
		panic(err)
	}

	permutator, err := utils.NewPermutator(databaseSize(p.params), key, tweak)
	if err != nil {
		fmt.Println("Error Creating Permutator:", err)
		panic(err)
	}

	encKey, err := utils.GenerateRandomKey(keyLength)
	if err != nil {
		fmt.Println("Error Generating Encryptor Key:", err)
		panic(err)
	}

	encryptor, err := utils.NewEncryptor(encKey, int(p.params.Q))
	if err != nil {
		fmt.Println("Error Creating Encryptor:", err)
		panic(err)
	}

	p.permutator = &permutator
	p.encryptor = &encryptor
}

// GenFromConfig initializes the PIR keys from a configuration file.
func (p *pirCore) GenFromConfig(filename string) {
	keys := GetKeysFromConfig(filename)
	permutator, err := utils.NewPermutator(databaseSize(p.params), keys.PermKey, keys.PermTweak)
	if err != nil {
		fmt.Println("Error Creating Permutator:", err)
		panic(err)
	}

	encryptor, err := utils.NewEncryptor(keys.EncKey, int(p.params.Q))
	if err != nil {
		fmt.Println("Error Creating Encryptor:", err)
		panic(err)
	}
	p.permutator = &permutator
	p.encryptor = &encryptor
}

// permuteAndEncryptMatrix applies the PIR permutation and encryption to the encoded matrix.
func (pir *pirCore) permuteAndEncryptMatrix(slice int, rmc Matrix) error {
	// Size of the matrix (number of elements)
	size := rmc.Size()
	visited := make([]bool, size) // Track visited positions
	index := 0                    // Start index

	for index < size {
		// If already visited, move to the next unvisited index
		if visited[index] {
			index++
			continue
		}

		// Start a new cycle with the current data
		start := index
		data := rmc.GetByIndex(start)
		for !visited[start] {
			visited[start] = true

			// Permute the current index to get the new position
			ciphertextInt64, err := pir.permutator.Permute(uint64(start))
			if err != nil {
				return fmt.Errorf("permute encoded position %d: %w", start, err)
			}
			newIndex := int(ciphertextInt64)

			// Fetch the data at the new position to continue the cycle
			newData := rmc.GetByIndex(newIndex)

			// Encrypt and set data at the new position
			rmc.SetByIndex(newIndex, pir.encryptor.EncryptSlice(slice, newIndex, data))

			// Move to the next position in the cycle
			data = newData
			start = newIndex
		}

		// Advance index to find the next unvisited element if the cycle is complete
		index++
	}
	return nil
}

// fetchServerResponse sends a query list to the server and reads the response payload.
func (pir *pirCore) fetchServerResponse(list []int, url string) []int {
	jsonData, err := json.Marshal(list)
	if err != nil {
		panic(fmt.Errorf("encode PIR query: %w", err))
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		panic(fmt.Errorf("PIR server returned HTTP %s: %s", resp.Status, string(body)))
	}

	var respData Response
	if err := json.Unmarshal(body, &respData); err != nil {
		panic(fmt.Errorf("decode PIR server response: %w", err))
	}

	return respData.Values
}

// queryLocalDBValues retrieves the database values at the given positions from a local matrix file.
func (p *pirCore) queryLocalDBValues(list []int, filename string) []int {
	rmc, err := utils.ReadMatrixFromFile(filename)
	if err != nil {
		panic(err)
	}

	response := make([]int, len(list))
	for idx, i := range list {
		row, col := utils.SingleIndexToRowCol(int(p.params.Q), i)
		response[idx] = rmc[row][col]
	}

	return response
}

// queryLocalDB3DValues retrieves the database values at the given positions from a local 3D matrix file.
func (p *pirCore) queryLocalDB3DValues(list []int, filename string) []int {
	rmc, err := utils.Read3DMatrixFromFile(filename, int(p.params.Q))
	if err != nil {
		panic(err)
	}

	response := make([]int, len(list))
	for idx, i := range list {
		x, y, z := utils.SingleIndexToRowCol3D(int(p.params.Q), i)
		response[idx] = rmc[x][y][z]
	}

	return response
}

// PrepareQuerySequenceWithNoise permutes real positions, appends uniform noise,
// and shuffles the complete query while retaining the private real positions.
func (p *pirCore) PrepareQuerySequenceWithNoise(realPoints []int, noiseCount int) ([]int, []int) {
	if p.permutator == nil {
		panic("PIR keys are not initialized; call Gen or GenFromConfig first")
	}
	if noiseCount < 0 {
		panic(fmt.Sprintf("noise count must be non-negative, got %d", noiseCount))
	}

	realPositions := make([]int, len(realPoints))
	for i, point := range realPoints {
		position, err := p.permutator.Permute(uint64(point))
		if err != nil {
			panic(fmt.Sprintf("permute query position %d: %v", point, err))
		}
		realPositions[i] = int(position)
	}

	query := make([]int, len(realPositions)+noiseCount)
	copy(query, realPositions)
	size := databaseSize(p.params)
	for i := len(realPositions); i < len(query); i++ {
		query[i] = rand.Intn(size)
	}
	rand.Shuffle(len(query), func(i, j int) { query[i], query[j] = query[j], query[i] })
	return query, realPositions
}

// decodeIndividualResponse removes noise positions, unmasks the values at the
// private query positions, and combines them in the configured field.
func (p *pirCore) decodeIndividualResponse(serverValues, query, realPositions []int) (int, error) {
	if len(serverValues) != len(query) {
		return 0, fmt.Errorf("server returned %d values for %d query positions", len(serverValues), len(query))
	}
	if len(realPositions) == 0 {
		return 0, fmt.Errorf("result has no real positions")
	}

	positionToValue := make(map[int]int, len(query))
	for i, position := range query {
		positionToValue[position] = serverValues[i]
	}

	q := int(p.params.Q)
	if isPowerOfTwo(p.params.Q) {
		decoded := 0
		for _, position := range realPositions {
			value, ok := positionToValue[position]
			if !ok {
				return 0, fmt.Errorf("missing server response for real position %d", position)
			}
			decoded ^= p.encryptor.Decrypt(position, value)
		}
		return decoded, nil
	}

	var decoded int64
	for _, position := range realPositions {
		value, ok := positionToValue[position]
		if !ok {
			return 0, fmt.Errorf("missing server response for real position %d", position)
		}
		decoded += int64(p.encryptor.EncryptPosition(0, position)) - int64(value)
	}
	return int(((decoded % int64(q)) + int64(q)) % int64(q)), nil
}

// isPowerOfTwo reports whether a value is an exact power of two.
func isPowerOfTwo(v uint64) bool {
	return v != 0 && (v&(v-1)) == 0
}

// isPrime reports whether v can use the integer-mod-q field implementation.
func isPrime(v uint64) bool {
	if v < 2 {
		return false
	}
	if v%2 == 0 {
		return v == 2
	}
	for divisor := uint64(3); divisor <= v/divisor; divisor += 2 {
		if v%divisor == 0 {
			return false
		}
	}
	return true
}

func databaseSize(params Params) int {
	if params.Q == 0 {
		panic("field size q must be positive")
	}
	maxInt := uint64(^uint(0) >> 1)
	size := uint64(1)
	for dimension := uint8(0); dimension < params.M; dimension++ {
		if size > maxInt/params.Q {
			panic(fmt.Sprintf("database size %d^%d does not fit in an int", params.Q, params.M))
		}
		size *= params.Q
	}
	return int(size)
}
