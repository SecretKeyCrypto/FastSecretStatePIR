package utils

func SingleIndexToRowColGo(q, idx int) (int, int) {
	row := idx % q
	col := idx / q
	return row, col
}

func RowColToSingleIndexGo(q, row, col int) int {
	return col*q + row
}

// Julia is column-major order and start with 1
func SingleIndexToRowColJulia(q, idx int) (int, int) {
	col := idx/q + 1
	row := idx % q
	return row, col
}

func GoRowColToSingleIdxJulia(q, row, col int) int {
	return col*q + row + 1
}

func RowColToSingleIndexJulia(q, row, col int) int {
	return (col-1)*q + row
}

func SingleIndexGoToJulia(q, idx int) int {
	row, col := SingleIndexToRowColGo(q, idx)
	return RowColToSingleIndexGo(q, row+1, col+1)
}
