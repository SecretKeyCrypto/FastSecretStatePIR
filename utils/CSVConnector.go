package utils

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
)

func WriteMatrixToFile(matrix [][]int, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, row := range matrix {
		stringRow := make([]string, len(row))
		for i, value := range row {
			stringRow[i] = strconv.Itoa(value) // Convert each integer to a string
		}
		if err := writer.Write(stringRow); err != nil {
			return err // Return early on write error
		}
	}

	return nil
}

func WriteSliceToCSV(filename string, numbers []int) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("could not create file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	stringNumbers := make([]string, len(numbers))
	for i, num := range numbers {
		stringNumbers[i] = strconv.Itoa(num)
	}

	if err := writer.Write(stringNumbers); err != nil {
		return fmt.Errorf("could not write to CSV: %v", err)
	}

	if err := writer.Error(); err != nil {
		return fmt.Errorf("error writing CSV: %v", err)
	}

	return nil
}

func ReadMatrixFromFile(filename string) ([][]int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)

	var matrix [][]int
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		intRow := make([]int, len(record))
		for i, valueStr := range record {
			value, err := strconv.Atoi(valueStr) // Convert each string back to an integer
			if err != nil {
				return nil, err // Return early on conversion error
			}
			intRow[i] = value
		}
		matrix = append(matrix, intRow)
	}

	return matrix, nil
}
