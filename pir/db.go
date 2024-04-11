package pir

import (
	"math/rand"
	"rme/utils"
	"time"
)

func FakeDB(q int, n int) []int {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))

	message := make([]int, n) // Create a slice to hold the numbers.
	for i := range message {
		message[i] = rand.Intn(q) // Generate a random number in [0, q).
	}

	utils.WriteSliceToCSV("../input/db.csv", message)
	return message
}
