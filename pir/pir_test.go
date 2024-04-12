package pir

import (
	"fmt"
	"math/rand"
	"rme/utils"
	"testing"
	"time"
)

func BenchmarkNewPIR(b *testing.B) {
	pir := NewPIR(GetParamsFromConfig())
	degree := int(pir.params.K)
	q := int(pir.params.Q)
	pir.Gen()

	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	var totalDuration time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := rand.Intn(q * q)
		start := time.Now()
		utils.GenerateCurvePoints(degree, q, target)
		duration := time.Since(start)
		totalDuration += duration
	}

	b.StopTimer()
	fmt.Printf("Average time per operation: %v\n", totalDuration)
}

func BenchmarkEncode(b *testing.B) {
	pir := NewPIR(GetParamsFromConfig())
	q := int(pir.params.Q)
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
	fmt.Printf("Average time per operation: %v\n", totalDuration)
}

func BenchmarkDecoding(b *testing.B) {
	pir := NewPIR(GetParamsFromConfig())
	q := int(pir.params.Q)
	FakeDB(q, 6)
	var totalDuration time.Duration

	pir.Encode("../input/db.csv", "../output/matrix.csv")
	ori, _ := utils.ReadMatrixFromFile("../output/inter.csv")

	b.ResetTimer()

	for i := 0; i < q*q; i++ {
		start := time.Now()
		sum, points := pir.Query(i, false)
		dec := pir.Decode(sum, points)
		duration := time.Since(start)
		totalDuration += duration
		row, col := utils.SingleIndexToRowCol(q, i)
		if ori[row][col] != dec {
			panic("Decoding ERROR")
		}
	}
	b.StopTimer()
	fmt.Printf("Average time per operation: %v\n", (totalDuration))
}
