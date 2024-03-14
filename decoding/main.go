package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"rme/utils"
)

func getAndVerifyParameter() (*utils.Permutator, *utils.Encryptor, utils.Config) {
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

	permutator, err := utils.NewPermutatorWithParameters(
		conf.PermutatorConfig.MaxNum,
		conf.PermutatorConfig.MinNum,
		conf.PermutatorConfig.MaxLen,
		conf.PermutatorConfig.MinLen,
		conf.PermutatorConfig.Radix,
		key,
		tweak)

	if err != nil {
		panic(err)
	}

	key, err = base64.StdEncoding.DecodeString(conf.EncyrptorKey)
	if err != nil {
		fmt.Println("Error decoding key:", err)
		panic(err)
	}

	encryptor, err := utils.NewEncryptor(key, conf.Q)

	if err != nil {
		fmt.Println("Error creating ff1 cipher encryptor:", err)
		panic(err)
	}
	return permutator, encryptor, conf
}

type Response struct {
	Sum int `json:"sum"`
}

func sendToServer(list []int) int {
	url := "http://10.1.61.31:8080"

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

func prepareQuerySequenceAndDecodingKey(q int, points []int, p *utils.Permutator, e *utils.Encryptor) ([]int, int) {
	var query []int
	dec_sum := 0
	for _, point := range points {
		po, _ := p.EncryptMap(point)
		row, col := utils.SingleIndexToRowColGo(q, po)
		query = append(query, utils.GoRowColToSingleIdxJulia(q, row, col))
		dec_sum += e.EncryptPosition(row, col)
		dec_sum %= q
	}

	return query, dec_sum
}

func main() {
	permutator, encryptor, config := getAndVerifyParameter()
	degree := config.K
	q := config.Q

	rand := rand.New(rand.NewSource(time.Now().UnixNano()))

	start := time.Now()
	target := rand.Intn(q * q)
	points := utils.GenerateCurvePoints(degree-1, q, target)
	query, dec_key := prepareQuerySequenceAndDecodingKey(q, points, permutator, encryptor)

	response := sendToServer(query)
	fmt.Println("Total Time", time.Since(start))

	filename := "../output/matrix_original.csv"

	ori, _ := utils.ReadMatrixFromFile(filename)
	t_row, t_col := utils.SingleIndexToRowColGo(q, target)
	target_val := ori[t_row][t_col]
	fmt.Println("sum of values: ", (response-dec_key+q)%q, " for ", " position ", target, " with value", target_val, ((response-dec_key+q)%q+target_val)%q == 0)
}
