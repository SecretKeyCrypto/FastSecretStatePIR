package pir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
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

// This works for any database size less than Q^2/2K^2
func NewPIR(params Params) *pir {
	var pir pir

	pir.params = params

	juliaPath, err1 := findJuliaPath()
	if err1 != nil {
		juliaPath = "/Applications/Julia-1.7.app/Contents/Resources/julia/bin/julia"
	}

	scriptPath := "../encoding/GoRMEInterface.jl"

	juliaCmd := exec.Command(juliaPath, scriptPath)
	pir.juliaCmd = juliaCmd

	juliaCmdIn, err := pir.juliaCmd.StdinPipe()
	pir.juliaCmdIn = juliaCmdIn
	if err != nil {
		fmt.Println("Error getting stdin of Julia:", err)
		panic(err)
	}

	juliaCmdOut, err := pir.juliaCmd.StdoutPipe()
	pir.juliaCmdOut = juliaCmdOut
	if err != nil {
		fmt.Println("Error getting stdout of Julia:", err)
		panic(err)
	}

	err = pir.juliaCmd.Start()
	if err != nil {
		fmt.Println("Error starting Julia:", err)
		panic(err)
	}

	return &pir
}

func (p *pir) Close() {
	err := p.juliaCmd.Wait()
	if err != nil {
		fmt.Println("Error waiting for Julia:", err)
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
	results := make([]int, len(sums)) // Create a list to store the results

	for i, sum := range sums {
		dec_sum := 0

		// Perform the same process for each sum in the list
		for _, po := range query {
			dec_sum += pir.encryptor.EncryptPosition(i, int(po))
			dec_sum %= q
		}

		// Perform the final decoding step for each sum
		results[i] = (dec_sum - sum + q) % q
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
