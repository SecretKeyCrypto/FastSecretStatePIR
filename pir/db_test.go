package pir

import "testing"

func TestMatrixCoordinateIndexRoundTrip(t *testing.T) {
	matrix2D := Matrix2D{data: make([][]int, 4), q: 4}
	for i := range matrix2D.data {
		matrix2D.data[i] = make([]int, 4)
	}
	for index := 0; index < matrix2D.Size(); index++ {
		coordinate := matrix2D.IndexToCoordinate(index)
		if got := matrix2D.CoordinateToIndex(coordinate[0], coordinate[1]); got != index {
			t.Fatalf("2D index %d maps back to %d through coordinate %v", index, got, coordinate)
		}
	}

	matrix3D := Matrix3D{data: make([][][]int, 4), q: 4}
	for i := range matrix3D.data {
		matrix3D.data[i] = make([][]int, 4)
		for j := range matrix3D.data[i] {
			matrix3D.data[i][j] = make([]int, 4)
		}
	}
	for index := 0; index < matrix3D.Size(); index++ {
		coordinate := matrix3D.IndexToCoordinate(index)
		if got := matrix3D.CoordinateToIndex(coordinate[0], coordinate[1], coordinate[2]); got != index {
			t.Fatalf("3D index %d maps back to %d through coordinate %v", index, got, coordinate)
		}
	}
}
