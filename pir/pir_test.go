package pir

import (
	"fmt"
	"math/rand"
	"rme/utils"
	"testing"
	"time"
)

func BenchmarkEncode(b *testing.B) {
	q := 31
	d := 4
	pir := NewPIR(Params{uint64(q), uint8(d)})
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
	pir := NewPIR(Params{uint64(q), uint8(d)})
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

func TestEndToEnd(b *testing.T) {
	q := 31
	d := 2
	pir := NewPIR(Params{uint64(q), uint8(d)})
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
	pir := NewPIR(Params{uint64(q), uint8(d)})
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
	pir := NewPIR(Params{uint64(q), uint8(d)})
	configFilename := "../config.json"
	pir.GenFromConfig(configFilename)

	for i := 0; i < q*q; i++ {
		sum, points := pir.Query(i, utils.GetParameterConfig(configFilename).ServerUrl)
		pir.Decode(sum, points)
	}
}
