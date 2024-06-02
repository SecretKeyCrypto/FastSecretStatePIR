package utils

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
)

func Write2DMatrixToFile(matrix [][]int, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, row := range matrix {
		strRow := make([]string, len(row))
		for i, val := range row {
			strRow[i] = strconv.Itoa(val)
		}
		if err := writer.Write(strRow); err != nil {
			return fmt.Errorf("error writing row to file: %v", err)
		}
	}

	return nil
}

func Write3DMatrixToFile(matrix [][][]int, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, layer := range matrix {
		for _, row := range layer {
			strRow := make([]string, len(row))
			for i, val := range row {
				strRow[i] = strconv.Itoa(val)
			}
			if err := writer.Write(strRow); err != nil {
				return fmt.Errorf("error writing row to file: %v", err)
			}
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

func Read3DMatrixFromFile(filename string, q int) ([][][]int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	matrix3D := make([][][]int, q)
	for i := range matrix3D {
		matrix3D[i] = make([][]int, q)
		for j := range matrix3D[i] {
			matrix3D[i][j] = make([]int, q)
		}
	}

	reader := csv.NewReader(file)
	layer := 0
	row := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Avoid empty line between layers
		if len(record) == 0 {
			continue
		}

		for col, val := range record {
			num, err := strconv.Atoi(val)
			if err != nil {
				return nil, err
			}
			matrix3D[layer][row][col] = num
		}
		row++
		layer += row / q
		row %= q
	}

	return matrix3D, nil
}
