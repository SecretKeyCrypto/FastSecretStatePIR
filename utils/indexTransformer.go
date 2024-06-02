package utils

func SingleIndexToRowCol(q, idx int) (int, int) {
	row := idx % q
	col := idx / q
	return row, col
}

func RowColToSingleIndex(q, row, col int) int {
	return col*q + row
}

func SingleIndexToRowCol3D(q, idx int) (int, int, int) {
	x := idx % q
	idx /= q
	y := idx % q
	z := idx / q

	return x, y, z
}

func RowColToSingleIndex3D(q, x, y, z int) int {
	return z*q*q + y*q + x
}

func SingleIndexToRowCol4D(q, idx int) (int, int, int, int) {
	x := idx % q
	idx /= q
	y := idx % q
	idx /= q
	z := idx % q
	v := idx / q

	return x, y, z, v
}

func RowColToSingleIndex4D(q, x, y, z, v int) int {
	return v*q*q*q + z*q*q + y*q + x
}
