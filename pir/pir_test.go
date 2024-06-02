package pir

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"rme/utils"
	"testing"
	"time"
)

func BenchmarkEncode(b *testing.B) {
	q := 31
	d := 4
	pir := NewPIR(Params{Q: uint64(q), K: uint8(d), M: uint8(2)})

	pir.Gen()
	input := "../input/db.csv"
	output := "../output/matrix.csv"
	FakeDB(q, 6, "../input/db.csv")

	var totalDuration time.Duration

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		pir.Encode(input, output)
		duration := time.Since(start)
		totalDuration += duration
	}

	b.StopTimer()
	fmt.Printf("Average time per operation: %v\n", totalDuration/time.Duration(b.N))
}

func BenchmarkGenerateQuery(b *testing.B) {
	q := 65521
	d := 2
	pir := NewPIR(Params{Q: uint64(q), K: uint8(d), M: uint8(2)})
	pir.Gen()

	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	var generateCurve, totalDuration time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := rand.Intn(q * q)
		start := time.Now()
		points := utils.GenerateCurvePoints(d, q, target)
		generateCurve += time.Since(start)
		pir.prepareQuerySequence(points)
		totalDuration += time.Since(start)
	}

	b.StopTimer()
	fmt.Printf("Average time per generating points: %v\n", generateCurve/time.Duration(b.N))
	fmt.Printf("Average time per generate query: %v\n", totalDuration/time.Duration(b.N))
}

func BenchmarkGenerate3DQuery(b *testing.B) {
	q := 65521
	d := 2
	m := 3
	pir := NewPIR(Params{Q: uint64(q), K: uint8(d), M: uint8(2)})
	pir.Gen()

	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	var generateCurve, totalDuration time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := rand.Intn(int(math.Pow(float64(q), float64(m))))
		start := time.Now()
		points := utils.GenerateCurvePoints3D(d, q, target)
		generateCurve += time.Since(start)
		pir.prepareQuerySequence(points)
		totalDuration += time.Since(start)
	}

	b.StopTimer()
	fmt.Printf("Average time per generating 3D points: %v\n", generateCurve/time.Duration(b.N))
	fmt.Printf("Average time per generate 3D query: %v\n", totalDuration/time.Duration(b.N))
}

func TestEndToEnd(b *testing.T) {
	q := 31
	d := 2
	pir := NewPIR(Params{Q: uint64(q), K: uint8(d), M: uint8(2)})
	pir.Gen()
	input := "../input/db.csv"
	output := "../output/matrix.csv"
	FakeDB(q, 6, input)

	pir.Encode(input, output)
	ori, _ := utils.ReadMatrixFromFile("../output/inter.csv")

	for i := 0; i < q*q; i++ {
		sum, points := pir.QueryLocal(i, output)
		dec := pir.Decode(sum, points)
		row, col := utils.SingleIndexToRowCol(q, i)
		if ori[row][col] != dec {
			panic("Decoding ERROR")
		}
	}
}

func TestEndToEndFromConfigKey(b *testing.T) {
	q := 31
	d := 2
	pir := NewPIR(Params{Q: uint64(q), K: uint8(d), M: uint8(2)})
	pir.GenFromConfig("../config.json")
	input := "../input/example_db.csv"
	output := "../output/example_matrix.csv"
	FakeDB(q, 10, input)

	pir.Encode(input, output)
	ori, _ := utils.ReadMatrixFromFile("../output/inter.csv")

	for i := 0; i < q*q; i++ {
		sum, points := pir.QueryLocal(i, output)
		dec := pir.Decode(sum, points)
		row, col := utils.SingleIndexToRowCol(q, i)
		if ori[row][col] != dec {
			panic("Decoding ERROR")
		}
	}
}

func TestQueryFromServer(b *testing.T) {
	q := 31
	d := 2
	m := 2
	pir := NewPIR(Params{Q: uint64(q), K: uint8(d), M: uint8(m)})
	configFilename := "../config.json"
	pir.GenFromConfig(configFilename)

	if !checkServer(utils.GetParameterConfig(configFilename).ServerUrl) {
		b.Skip("Skipping test: server is not reachable")
	}

	for i := 0; i < q*q; i++ {
		sum, points := pir.Query(i, utils.GetParameterConfig(configFilename).ServerUrl)
		pir.Decode(sum, points)
	}
}

func checkServer(url string) bool {
	client := http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
