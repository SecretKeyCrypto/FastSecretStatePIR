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
	Sum int `json:"sum"`
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

	rmc := p.RMEncoding(input, inter, int(p.params.Q), int(p.params.K), int(p.params.M))
	p.permuteAndEncryptMatrix(rmc)

	switch m := rmc.(type) {
	case *Matrix2D:
		utils.Write2DMatrixToFile(m.data, output)
	case *Matrix3D:
		utils.Write3DMatrixToFile(m.data, output)
	}

	return rmc
}

func (p *pir) Query(i int, url string) (int, []int) {
	points := utils.GenerateCurvePoints(int(p.params.K), int(p.params.Q), int(i))
	query := p.PrepareQuerySequence(points)
	response := p.queryServer(query, url)
	return response, query
}

func (p *pir) QueryLocal(i int, filename string) (int, []int) {
	var points []int
	var query []int
	var response int

	switch p.params.M {
	case 2:
		points = utils.GenerateCurvePoints(int(p.params.K), int(p.params.Q), int(i))
		query = p.PrepareQuerySequence(points)
		response = p.queryLocalDB(query, filename)
	case 3:
		points = utils.GenerateCurvePoints3D(int(p.params.K), int(p.params.Q), int(i))
		query = p.PrepareQuerySequence(points)
		response = p.queryLocalDB3D(query, filename)
	}

	return response, query
}

func (pir *pir) Decode(sum int, query []int) int {
	dec_sum := 0
	q := int(pir.params.Q)
	for _, po := range query {
		dec_sum += pir.encryptor.EncryptPosition(0, int(po))
		dec_sum %= q
	}

	return (dec_sum - sum + q) % q
}

func (pir *pir) DecodeLargeRecord(sum []int, query []int) []int {
	dec_sum := 0
	q := int(pir.params.Q)
	for i, po := range query {
		dec_sum += pir.encryptor.EncryptPosition(i, int(po))
		dec_sum %= q
	}

	for i := range sum {
		sum[i] = (dec_sum - sum[i] + q) % q
	}

	return sum
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

func (pir *pir) permuteAndEncryptMatrix(rmc Matrix) {
	var ciphertextInt64 uint64
	var new_data int

	visited := make([]bool, rmc.Size())

	data := rmc.GetByIndex(0)

	index := 0
	new_index := 0

	for {
		ciphertextInt64, _ = pir.permutator.Permute(uint64(index))
		new_index = int(ciphertextInt64)

		if visited[new_index] {
			index += 1
			if index == len(visited) {
				break
			}

			data = rmc.GetByIndex(index)
		} else {
			new_data = rmc.GetByIndex(new_index)

			rmc.SetByIndex(new_index, pir.encryptor.Encrypt(new_index, data))

			data = new_data
			index = new_index
			visited[new_index] = true
		}
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

func (pir *pir) queryServer(list []int, url string) int {
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

	return respData.Sum
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
