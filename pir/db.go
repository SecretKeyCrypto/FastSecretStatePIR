package pir

import (
	"math/rand"
	"rme/utils"
	"time"
)

func FakeDB(q int, n int, filename string) []int {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))

	message := make([]int, n) // Create a slice to hold the numbers.
	for i := range message {
		message[i] = rand.Intn(q) // Generate a random number in [0, q).
	}

	utils.WriteSliceToCSV(filename, message)
	return message
}

type Matrix interface {
	Get(indices ...int) int
	GetByIndex(index int) int
	Set(value int, indices ...int)
	SetByIndex(value int, index int)
	Dimensions() []int
	SetData(data interface{})
	Size() int
	Data() interface{}
}

// 2D matrix
type Matrix2D struct {
	data [][]int
	q    int
}

func (m Matrix2D) Data() interface{} {
	return m.data
}

// Get element from 2D matrix
func (m Matrix2D) Get(indices ...int) int {
	return m.data[indices[0]][indices[1]]
}

func (m Matrix2D) GetByIndex(index int) int {
	return m.Get(m.IndexToCoordinate(index)...)
}

// Set element in 2D matrix
func (m Matrix2D) Set(value int, indices ...int) {
	m.data[indices[0]][indices[1]] = value
}

func (m Matrix2D) SetByIndex(index int, value int) {
	coordinates := m.IndexToCoordinate(index)
	m.Set(value, coordinates...)
}

func (m Matrix2D) Dimensions() []int {
	if len(m.data) == 0 {
		return []int{0, 0}
	}
	return []int{len(m.data), len(m.data[0])}
}

func (m Matrix2D) Size() int {
	if len(m.data) == 0 {
		return 0
	}
	return len(m.data) * len(m.data[0])
}

func (m *Matrix2D) SetData(data interface{}) {
	if d, ok := data.([][]int); ok {
		m.data = d
		m.q = len(d)
	} else {
		panic("Invalid data type for Matrix2D")
	}
}

func (m Matrix2D) CoordinateToIndex(i, j int) int {
	return i*m.q + j
}

func (m Matrix2D) IndexToCoordinate(index int) []int {
	i := index % m.q
	j := index / m.q
	return []int{i, j}
}

// 3D matrix
type Matrix3D struct {
	data [][][]int
	q    int
}

// Get element from 3D matrix
func (m Matrix3D) Get(indices ...int) int {
	return m.data[indices[0]][indices[1]][indices[2]]
}

func (m Matrix3D) GetByIndex(index int) int {
	return m.Get(m.IndexToCoordinate(index)...)
}

// Set element in 3D matrix
func (m Matrix3D) Set(value int, indices ...int) {
	m.data[indices[0]][indices[1]][indices[2]] = value
}

func (m Matrix3D) SetByIndex(index int, value int) {
	coordinates := m.IndexToCoordinate(index)
	m.Set(value, coordinates...)
}

// Dimensions returns the lengths of each dimension for the 3D matrix
func (m Matrix3D) Dimensions() []int {
	if len(m.data) == 0 || len(m.data[0]) == 0 {
		return []int{0, 0, 0}
	}
	return []int{len(m.data), len(m.data[0]), len(m.data[0][0])}
}

func (m Matrix3D) Data() interface{} {
	return m.data
}

func (m Matrix3D) Size() int {
	if len(m.data) == 0 || len(m.data[0]) == 0 {
		return 0
	}
	return len(m.data) * len(m.data[0]) * len(m.data[0][0])
}

func (m *Matrix3D) SetData(data interface{}) {
	if d, ok := data.([][][]int); ok {
		m.data = d
	} else {
		panic("Invalid data type for Matrix2D")
	}
}

func (m Matrix3D) CoordinateToIndex(i, j, k int) int {
	rows := len(m.data)
	cols := len(m.data[0])
	return i*cols*rows + j*cols + k
}

func (m Matrix3D) IndexToCoordinate(index int) []int {
	k := index / (m.q * m.q)
	index = index % (m.q * m.q)
	j := index / m.q
	i := index % m.q
	return []int{i, j, k}
}
