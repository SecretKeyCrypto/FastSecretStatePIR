package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os/exec"
	"time"

	"rme/utils"
)

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

func RMEncoding(q, k int) (matrix [][]int) {
	juliaPath, err1 := findJuliaPath()
	if err1 != nil {
		juliaPath = "/Applications/Julia-1.7.app/Contents/Resources/julia/bin/julia"
	}

	scriptPath := "./GoRMEInterface.jl"

	// Argument to pass to your Julia script
	argument := fmt.Sprint(q)
	arg_k := fmt.Sprint(k)

	// Execute the Julia script
	cmd := exec.Command(juliaPath, scriptPath, argument, arg_k)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error executing Julia:", err)
	}

	filename_ori := "../output/matrix_original.csv"
	matrix, err = utils.ReadMatrixFromFile(filename_ori)
	if err != nil {
		fmt.Println("Error Reading Matrix From Julia Output:", err)
		panic(err)
	}
	return
}

func permuteMatrix(permutator utils.Permutator, encryptor *utils.Encryptor, rmc [][]int) {
	var ciphertextInt int
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
		val = utils.RowColToSingleIndexGo(q, cur_row, cur_col)
		ciphertextInt, _ = permutator.EncryptMap(val)
		new_row, new_col = utils.SingleIndexToRowColGo(q, ciphertextInt)

		if visited[new_row][new_col] {
			cur_row = cur_row + (cur_col+1)/q
			cur_col = (cur_col + 1) % q
			if cur_row == q {
				break
			}

			data = rmc[cur_row][cur_col]
		} else {
			new_data = rmc[new_row][new_col]
			rmc[new_row][new_col] = encryptor.Encrypt(new_row, new_col, data)

			data = new_data
			cur_row = new_row
			cur_col = new_col
			visited[new_row][new_col] = true
		}
	}

}

func verify(permutator *utils.Permutator, encryptor *utils.Encryptor, enc [][]int, q int) {
	filename_ori := "../output/matrix_original.csv"
	ori, err := utils.ReadMatrixFromFile(filename_ori)
	if err != nil {
		panic(err)
	}

	decoded := make([][]int, q)

	for i := range decoded {
		decoded[i] = make([]int, q)
	}

	for i := 0; i < len(enc); i++ {
		for j := 0; j < len(enc[0]); j++ {
			decoded[i][j] = encryptor.Decrypt(i, j, enc[i][j])
		}
	}

	for i := 0; i < q*q; i++ {
		new_position, _ := permutator.EncryptMap(i)
		row, col := utils.SingleIndexToRowColGo(q, new_position)
		ori_row, ori_col := utils.SingleIndexToRowColGo(q, i)

		if ori[ori_row][ori_col] != decoded[row][col] {
			panicMessage := fmt.Sprintf("MISS MATCH: Index %d, From value %d at (%d, %d) To %d at (%d, %d)", i, ori[row][col], i/11, i%11, decoded[row][col], row, col)
			fmt.Println(panicMessage)
		}
	}
}

func getPermutatorAndEncrpytor() (*utils.Permutator, *utils.Encryptor, utils.Config) {
	conf := utils.GetParameterConfig()

	key, err := base64.StdEncoding.DecodeString(conf.PermutatorConfig.Key)
	if err != nil {
		fmt.Println("Error decoding key:", err)
		panic(err)
	}

	tweak, err := base64.StdEncoding.DecodeString(conf.PermutatorConfig.Tweak)
	if err != nil {
		fmt.Println("Error decoding tweak:", err)
		panic(err)
	}

	numRangeMax := conf.Q * conf.Q
	permutator, err := utils.NewPermutator(numRangeMax, key, tweak)

	conf.PermutatorConfig.MaxNum,
		conf.PermutatorConfig.MinNum,
		conf.PermutatorConfig.MaxLen,
		conf.PermutatorConfig.MinLen,
		conf.PermutatorConfig.Radix = permutator.ReturnParameters()

	if err != nil {
		panic(err)
	}

	key, err = base64.StdEncoding.DecodeString(conf.EncyrptorKey)
	if err != nil {
		fmt.Println("Error decoding key:", err)
		panic(err)
	}

	encryptor, _ := utils.NewEncryptor(key, conf.Q)
	utils.WriteConfigToFile(conf)

	return permutator, encryptor, conf
}

func main() {
	permutator, encryptor, config := getPermutatorAndEncrpytor()

	starttime := time.Now()
	rmc := RMEncoding(config.Q, config.K)
	fmt.Println(time.Since(starttime), "For Encoding")
	starttime = time.Now()

	permuteMatrix(*permutator, encryptor, rmc)
	fmt.Println(time.Since(starttime), "For Permutation")
	starttime = time.Now()

	filename := "../output/matrix.csv"
	if err := utils.WriteMatrixToFile(rmc, filename); err != nil {
		panic(err)
	}

	verify(permutator, encryptor, rmc, config.Q)
}
