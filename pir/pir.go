package pir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"rme/utils"
)

type PIR interface {
	Gen()
	Encode(string, string) [][]int
	Query(uint64, string) (int, []int)
	Decode(int, []int) int
}

const (
	keyLength   = 16
	tweakLength = 8
)

type Response struct {
	Sums []int `json:"sums"`
}

type pir struct {
	params      Params
	permutator  *utils.Permutator
	encryptor   *utils.Encryptor
	juliaCmd    *exec.Cmd
	juliaCmdIn  io.WriteCloser
	juliaCmdOut io.ReadCloser
}

// NewPIR creates a PIR instance. The encoder subprocess is started lazily on
// the first Encode call, so Gen/Query/Decode work without any encoder installed.
func NewPIR(params Params) *pir {
	var p pir
	p.params = params
	return &p
}

// startEncoder starts the RM encoder subprocess. The C++ binary
// (encoding/rme) is preferred; Julia (encoding/GoRMEInterface.jl) is used as
// fallback if the binary is not present.
func (p *pir) startEncoder() {
	var cmd *exec.Cmd

	cppBin := "../encoding/rme"
	if _, err := os.Stat(cppBin); err == nil {
		cmd = exec.Command(cppBin)
	} else {
		// Fall back to Julia.
		juliaPath, err := findJuliaPath()
		if err != nil {
			juliaPath = "/Applications/Julia-1.7.app/Contents/Resources/julia/bin/julia"
		}
		cmd = exec.Command(juliaPath, "../encoding/GoRMEInterface.jl")
	}

	p.juliaCmd = cmd

	cmdIn, err := cmd.StdinPipe()
	if err != nil {
		fmt.Println("Error getting encoder stdin:", err)
		panic(err)
	}
	p.juliaCmdIn = cmdIn

	cmdOut, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error getting encoder stdout:", err)
		panic(err)
	}
	p.juliaCmdOut = cmdOut

	if err = cmd.Start(); err != nil {
		fmt.Println("Error starting encoder:", err)
		panic(err)
	}
}

func (p *pir) Close() {
	if p.juliaCmd == nil {
		return
	}
	if err := p.juliaCmd.Wait(); err != nil {
		fmt.Println("Error waiting for encoder:", err)
		panic(err)
	}
}

func (p *pir) Gen() {
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

	permutator, err := utils.NewPermutator(int(math.Pow(float64(p.params.Q), float64(p.params.M))), key, tweak)
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

func (p *pir) GenFromConfig(filename string) {
	keys := GetKeysFromConfig(filename)
	permutator, err := utils.NewPermutator(int(math.Pow(float64(p.params.Q), float64(p.params.M))), keys.PermKey, keys.PermTweak)
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

func (p *pir) Encode(input, output string) Matrix {
	if p.juliaCmd == nil {
		p.startEncoder()
	}
	inter := "../output/inter.csv"

	// TODO: Update to support sliced encoding and pass the slice id to Encrypt function.
	rmc := p.RMEncoding(input, inter, int(p.params.Q), int(p.params.K), int(p.params.M))
	p.permuteAndEncryptMatrix(0, rmc)

	switch m := rmc.(type) {
	case *Matrix2D:
		utils.Write2DMatrixToFile(m.data, output)
	case *Matrix3D:
		utils.Write3DMatrixToFile(m.data, output)
	}

	return rmc
}

func (p *pir) Query(i int, url string) ([]int, []int) {
	points := utils.GenerateCurvePoints(int(p.params.K), int(p.params.Q), int(i))
	query := p.PrepareQuerySequence(points)
	response := p.queryServer(query, url)
	return response, query
}

func (p *pir) QueryLocal(i int, filenames []string) ([]int, []int) {
	var points []int
	var query []int
	var response []int

	switch p.params.M {
	case 2:
		for _, filename := range filenames {
			points = utils.GenerateCurvePoints(int(p.params.K), int(p.params.Q), int(i))
			query = p.PrepareQuerySequence(points)
			response = append(response, p.queryLocalDB(query, filename))
		}
	case 3:
		for _, filename := range filenames {
			points = utils.GenerateCurvePoints3D(int(p.params.K), int(p.params.Q), int(i))
			query = p.PrepareQuerySequence(points)
			response = append(response, p.queryLocalDB3D(query, filename))
		}
	}

	return response, query
}

func (pir *pir) Decode(sums []int, query []int) []int {
	q := int(pir.params.Q)
	results := make([]int, len(sums))

	for i, sum := range sums {
		// Accumulate in int64 to avoid per-element modular reduction.
		// Safe as long as len(query)*(q-1) < 2^63, which holds for q < 2^31.
		var acc int64
		for _, po := range query {
			acc += int64(pir.encryptor.EncryptPosition(i, po))
		}
		decSum := int(acc % int64(q))
		results[i] = (decSum - sum%q + q) % q
	}

	return results
}

// PrepareQuerySequenceWithNoise is the PLDN variant for the Lifted RS construction.
// It permutes the real curve points, appends noiseCount uniform-random positions,
// and shuffles everything. Returns (permutedAll, permutedReal):
//   - permutedAll: the full L-length query sent to the server.
//   - permutedReal: the ℓ=q-1 permuted real positions used during decoding.
func (pir *pir) PrepareQuerySequenceWithNoise(realPoints []int, noiseCount int) ([]int, []int) {
	permReal := make([]int, len(realPoints))
	for idx, pt := range realPoints {
		po, _ := pir.permutator.Permute(uint64(pt))
		permReal[idx] = int(po)
	}

	qm := int(math.Pow(float64(pir.params.Q), float64(pir.params.M)))
	all := make([]int, len(permReal)+noiseCount)
	copy(all, permReal)
	for k := len(permReal); k < len(all); k++ {
		// Noise positions are uniform in the permuted space [0, q^m).
		all[k] = rand.Intn(qm)
	}
	rand.Shuffle(len(all), func(a, b int) { all[a], all[b] = all[b], all[a] })

	return all, permReal
}

// DecodeLifted decodes a single-slice response under the PLDN protocol where
// the server returns individual values at every queried position.
// serverValuesAtReal[j] = x̂[permutedReal[j]]: the server's (masked) codeword
// value at the j-th real curve position.
// The client subtracts PRF masks and sums over the q-1 positions; the sumcheck
// identity Σ_{z≠t} g(z) = −g(t) then recovers f(target).
func (pir *pir) DecodeLifted(serverValuesAtReal []int, permutedReal []int) int {
	q := int(pir.params.Q)
	var acc int64
	for j, po := range permutedReal {
		acc += int64(pir.encryptor.EncryptPosition(0, po)) - int64(serverValuesAtReal[j])
	}
	return int(((acc % int64(q)) + int64(q)) % int64(q))
}

// DecodeLiftedGF2n is the GF(2^n) variant of DecodeLifted.
// Server returns individual values at each real curve position; client XORs
// PRF masks with server values (GF addition = XOR) to recover f(target).
func (pir *pir) DecodeLiftedGF2n(serverValuesAtReal []int, permutedReal []int) int {
	mask := int(pir.params.Q) - 1
	acc := 0
	for j, po := range permutedReal {
		acc ^= (pir.encryptor.EncryptPosition(0, po) & mask) ^ (serverValuesAtReal[j] & mask)
	}
	return acc
}

// ─── GF(2^n) query and decode ─────────────────────────────────────────────────
//
// Over GF(2^n), field addition is XOR, so the server accumulates responses with
// XOR and the client mask-subtracts with XOR rather than integer mod-subtraction.

// QueryLocalGF2n queries a permuted+encrypted local database file using GF(2^n)
// arithmetic (XOR accumulation instead of integer addition mod q).
func (p *pir) QueryLocalGF2n(gf *utils.GF2n, i int, filenames []string) ([]int, []int) {
	q := int(p.params.Q)
	points := utils.GenerateCurvePointsGF2n(gf, int(p.params.K), i)
	query := p.PrepareQuerySequence(points)

	var response []int
	for _, filename := range filenames {
		rmc, err := utils.ReadMatrixFromFile(filename)
		if err != nil {
			panic(err)
		}
		res := 0
		for _, idx := range query {
			row, col := utils.SingleIndexToRowCol(q, idx)
			res ^= rmc[row][col]
		}
		response = append(response, res)
	}
	return response, query
}

// DecodeGF2n recovers message values from GF(2^n) server responses.
// Uses XOR masking: result[i] = (XOR of PRF masks) XOR sums[i].
func (pir *pir) DecodeGF2n(sums []int, query []int) []int {
	mask := int(pir.params.Q) - 1 // q−1: masks to n bits since q = 2^n
	results := make([]int, len(sums))
	for i, sum := range sums {
		var acc int
		for _, po := range query {
			acc ^= pir.encryptor.EncryptPosition(i, po) & mask
		}
		results[i] = acc ^ (sum & mask)
	}
	return results
}

func findJuliaPath() (string, error) {
	cmd := exec.Command("which", "julia")

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	path := out.String()
	path = path[:len(path)-1]

	return path, nil
}

func (pir *pir) RMEncoding(input, output string, q, k, m int) Matrix {
	juliaArgs := fmt.Sprintf("%s %s %d %d %d\n", input, output, q, k, m)
	_, err := pir.juliaCmdIn.Write([]byte(juliaArgs))
	if err != nil {
		fmt.Println("Error writing to Julia:", err)
		panic(err)
	}
	var b []byte = make([]byte, 1)
	pir.juliaCmdOut.Read(b)

	switch m {
	case 3:
		mat, err := utils.Read3DMatrixFromFile(output, q)
		if err != nil {
			fmt.Println("Error Reading Matrix From Julia Output:", err)
			panic(err)
		}
		return &Matrix3D{data: mat, q: q}
	default:
		mat, err := utils.ReadMatrixFromFile(output)
		if err != nil {
			fmt.Println("Error Reading Matrix From Julia Output:", err)
			panic(err)
		}
		return &Matrix2D{data: mat, q: q}
	}
}

func (pir *pir) permuteAndEncryptMatrix(slice int, rmc Matrix) {
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
			ciphertextInt64, _ := pir.permutator.Permute(uint64(start))
			new_index := int(ciphertextInt64)

			// Fetch the data at the new position to continue the cycle
			new_data := rmc.GetByIndex(new_index)

			// Encrypt and set data at the new position
			rmc.SetByIndex(new_index, pir.encryptor.EncryptSlice(slice, new_index, data))

			// Move to the next position in the cycle
			data = new_data
			start = new_index
		}

		// Advance index to find the next unvisited element if the cycle is complete
		index++
	}
}

func (pir *pir) PrepareQuerySequence(points []int) []int {
	var query []int
	for _, point := range points {

		po, _ := pir.permutator.Permute(uint64(point))
		query = append(query, int(po))
	}

	return query
}

func (pir *pir) queryServer(list []int, url string) []int {
	jsonData, _ := json.Marshal(list)

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

	var respData Response

	json.Unmarshal([]byte(body), &respData)

	return respData.Sums
}

func (p *pir) queryLocalDB(list []int, filename string) int {
	rmc, err := utils.ReadMatrixFromFile(filename)
	if err != nil {
		panic(err)
	}

	res := 0

	for _, i := range list {
		row, col := utils.SingleIndexToRowCol(int(p.params.Q), i)
		res += rmc[row][col]
		res %= int(p.params.Q)
	}

	return res
}

func (p *pir) queryLocalDB3D(list []int, filename string) int {
	rmc, err := utils.Read3DMatrixFromFile(filename, int(p.params.Q))
	if err != nil {
		panic(err)
	}

	res := 0

	for _, i := range list {
		x, y, z := utils.SingleIndexToRowCol3D(int(p.params.Q), i)
		res += rmc[x][y][z]
		res %= int(p.params.Q)
	}

	return res
}
