package utils

import (
	"math/big"
	"math/rand"
)

// The polynomial is defined by its coefficients (in ascending order of degree).
func EvaluatePolynomial(coeffs []int, x int, q int) int {
	result := 0
	xPower := 1
	for _, coeff := range coeffs {
		result = (result + coeff*xPower) % q
		xPower = (xPower * x) % q
	}
	return result
}

func EvaluatePolynomialForLargeField(coeffs []int, x int, q int) int {
	bigQ := big.NewInt(int64(q))
	bigX := big.NewInt(int64(x))
	result := big.NewInt(0)
	xPower := big.NewInt(1)

	for _, coeff := range coeffs {
		bigCoeff := big.NewInt(int64(coeff))
		term := new(big.Int).Mul(bigCoeff, xPower)
		term.Mod(term, bigQ)

		result.Add(result, term)
		result.Mod(result, bigQ)

		xPower.Mul(xPower, bigX)
		xPower.Mod(xPower, bigQ)
	}

	return int(result.Int64())
}

func GenerateRandomPolynomialCoeffs(degree, q, t, a int) []int {
	coeffs := make([]int, degree+1)
	for i := range coeffs {
		coeffs[i] = rand.Intn(q) // Random int in [0, q-1]
	}

	coeffs[0] = (coeffs[0] - (EvaluatePolynomial(coeffs, t, q) - a) + q) % q
	return coeffs
}

func GenerateCurvePoints(degree, q, i int) []int {
	x, y := SingleIndexToRowCol(q, i)
	t := rand.Intn(q)
	uni_x := GenerateRandomPolynomialCoeffs(degree, q, t, x)
	uni_y := GenerateRandomPolynomialCoeffs(degree, q, t, y)

	var points []int
	for i = 0; i < q; i++ {
		if i != t {
			point_x := EvaluatePolynomial(uni_x, i, q)
			point_y := EvaluatePolynomial(uni_y, i, q)
			points = append(points, RowColToSingleIndex(q, point_x, point_y))
		}
	}
	return points
}
