package main

import (
	"encoding/base64"
	"fmt"
	"math/rand"
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

func main() {
	filename := "../output/matrix.csv"

	// Read the matrix back from the file
	rmc, err := utils.ReadMatrixFromFile(filename)
	if err != nil {
		panic(err)
	}

	permutator, encryptor, config := getAndVerifyParameter()
	degree := config.K
	q := config.Q

	rand := rand.New(rand.NewSource(time.Now().UnixNano()))

	start := time.Now()
	startTime := time.Now()
	target := rand.Intn(q * q)
	points := utils.GenerateCurvePoints(degree-1, q, target)

	duration := time.Since(startTime)
	fmt.Println("Time for generating permuted points:", duration)
	startTime = time.Now()
	var xx, yy, vv []int
	for _, point := range points {
		po, _ := permutator.EncryptMap(point)
		xx = append(xx, po/q)
		yy = append(yy, po%q)
	}
	duration = time.Since(startTime)
	fmt.Println("Time for Permutation", duration)

	startTime = time.Now()
	for i := 0; i < q-1; i++ {
		vv = append(vv, rmc[xx[i]][yy[i]])
	}

	duration = time.Since(startTime)
	fmt.Println("Server Time for Lookup", duration)

	var sum int

	startTime = time.Now()
	for i := 0; i < q-1; i++ {
		vv[i] = encryptor.Decrypt(xx[i], yy[i], vv[i])
		sum += vv[i]
	}

	duration = time.Since(startTime)
	fmt.Println("Time for DecryptValues and decoding", duration)

	target_p, _ := permutator.EncryptMap(target)
	target_val := encryptor.Decrypt(target_p/q, target_p%q, rmc[target_p/q][target_p%q])

	fmt.Println("sum of values: ", sum%q, "for position ", target, "with value", target_val, (sum%q+target_val)%q == 0)

	fmt.Println("Total Time", time.Since(start))
}
