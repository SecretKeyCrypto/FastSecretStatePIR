package pir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"rme/utils"
)

type PIR interface {
	Gen()
	Encode(string, string) [][]int
	Query(uint64, bool) (int, []int)
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
	params     Params
	permutator *utils.Permutator
	encryptor  *utils.Encryptor
}

// This works for any database size less than Q^2/2K^2
func NewPIR(params Params) *pir {
	var pir pir

	permutator, err := utils.NewPermutator(int(params.Q*params.Q), params.PermKey, params.PermTweak)
	if err != nil {
		fmt.Println("Error Creating Permutator:", err)
		panic(err)
	}

	encryptor, err := utils.NewEncryptor(params.EncKey, int(params.Q))
	if err != nil {
		fmt.Println("Error Creating Encryptor:", err)
		panic(err)
	}

	pir.params = params
	pir.permutator = &permutator
	pir.encryptor = &encryptor

	return &pir
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

	permutator, err := utils.NewPermutator(int(p.params.Q*p.params.Q), key, tweak)
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

	p.params.PermKey = key
	p.params.PermTweak = tweak
	p.params.EncKey = encKey
	p.permutator = &permutator
	p.encryptor = &encryptor

	if err := p.params.UpdateConfig(); err != nil {
		fmt.Println("Error writing config to file:", err)
	}
}

func (p *pir) Encode(input, output string) [][]int {
	inter := "../output/inter.csv"
	rmc := RMEncoding(input, inter, int(p.params.Q), int(p.params.K))
	p.permuteAndEncryptMatrix(rmc)
	utils.WriteMatrixToFile(rmc, output)
	return rmc
}

func (p *pir) Query(i int, serverOn bool) (int, []int) {
	points := utils.GenerateCurvePoints(int(p.params.K), int(p.params.Q), int(i))
	query, points := p.prepareQuerySequenceAndDecodingKey(points)
	var response int
	if serverOn {
		response = p.queryServer(query)
	} else {
		response = p.queryLocal(query)
	}
	return response, points
}

func (pir *pir) Decode(sum int, points []int) int {
	dec_sum := 0
	q := int(pir.params.Q)
	for _, point := range points {
		po, _ := pir.permutator.Permute(uint64(point))

		row, col := utils.SingleIndexToRowCol(q, int(po))
		dec_sum += pir.encryptor.EncryptPosition(row, col)
		dec_sum %= q
	}

	return (dec_sum - sum + q) % q
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

func RMEncoding(input, output string, q, k int) (matrix [][]int) {
	juliaPath, err1 := findJuliaPath()
	if err1 != nil {
		juliaPath = "/Applications/Julia-1.7.app/Contents/Resources/julia/bin/julia"
	}

	scriptPath := "../encoding/GoRMEInterface.jl"

	// Argument to pass to your Julia script
	arg_q := fmt.Sprint(q)
	arg_k := fmt.Sprint(k)

	// Execute the Julia script
	cmd := exec.Command(juliaPath, scriptPath, input, output, arg_q, arg_k)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error executing Julia:", err)
		panic(err)
	}

	matrix, err = utils.ReadMatrixFromFile(output)
	if err != nil {
		fmt.Println("Error Reading Matrix From Julia Output:", err)
		panic(err)
	}
	return
}

func (pir *pir) permuteAndEncryptMatrix(rmc [][]int) {
	var ciphertextInt64 uint64
	var val int
	var new_row int
	var new_col int
	var new_data int

	visited := make([][]bool, len(rmc))
	for i := range visited {
		visited[i] = make([]bool, len(rmc[0]))
	}

	q := len(rmc)
	cur_row := 0
	cur_col := 0
	data := rmc[cur_row][cur_col]

	for {
		val = utils.RowColToSingleIndex(q, cur_row, cur_col)
		ciphertextInt64, _ = pir.permutator.Permute(uint64(val))
		new_row, new_col = utils.SingleIndexToRowCol(q, int(ciphertextInt64))

		if visited[new_row][new_col] {
			cur_row = cur_row + (cur_col+1)/q
			cur_col = (cur_col + 1) % q
			if cur_row == q {
				break
			}

			data = rmc[cur_row][cur_col]
		} else {
			new_data = rmc[new_row][new_col]
			rmc[new_row][new_col] = pir.encryptor.Encrypt(new_row, new_col, data)

			data = new_data
			cur_row = new_row
			cur_col = new_col
			visited[new_row][new_col] = true
		}
	}

}

func (pir *pir) prepareQuerySequenceAndDecodingKey(points []int) ([]int, []int) {
	var query []int
	dec_sum := 0
	q := int(pir.params.Q)
	for _, point := range points {
		po, _ := pir.permutator.Permute(uint64(point))
		query = append(query, int(po))

		row, col := utils.SingleIndexToRowCol(q, int(po))
		dec_sum += pir.encryptor.EncryptPosition(row, col)
		dec_sum %= q
	}

	return query, points
}

func (pir *pir) queryServer(list []int) int {
	jsonData, _ := json.Marshal(list)

	req, err := http.NewRequest("POST", pir.params.ServerUrl, bytes.NewBuffer(jsonData))
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

func (p *pir) queryLocal(list []int) int {
	rmc, err := utils.ReadMatrixFromFile("../output/matrix.csv")
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
