package utils

func SingleIndexToRowCol(q, idx int) (int, int) {
	row := idx % q
	col := idx / q
	return row, col
}

func RowColToSingleIndex(q, row, col int) int {
	return col*q + row
}
