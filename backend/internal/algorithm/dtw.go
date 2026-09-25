package algorithm
import (
	"fmt"
	"math"
)
type AlignmentPair struct {
	ActualIndex    int `json:"actual_index"`
	ReferenceIndex int `json:"reference_index"`
}
func DTW(actual, reference []float64, window int) (float64, []AlignmentPair, error) {
	if len(actual) == 0 || len(reference) == 0 {
		return 0, nil, fmt.Errorf("DTW requires non-empty actual and reference vectors")
	}
	if window < 1 {
		window = maxInt(len(actual), len(reference))
	}
	lengthDifference := absInt(len(actual) - len(reference))
	if window < lengthDifference {
		window = lengthDifference
	}
	rows, columns := len(actual)+1, len(reference)+1
	cost := make([][]float64, rows)
	for i := range cost {
		cost[i] = make([]float64, columns)
		for j := range cost[i] {
			cost[i][j] = math.Inf(1)
		}
	}
	cost[0][0] = 0
	for i := 1; i < rows; i++ {
		from := maxInt(1, i-window)
		to := minInt(columns-1, i+window)
		for j := from; j <= to; j++ {
			step := math.Abs(actual[i-1] - reference[j-1])
			cost[i][j] = step + math.Min(cost[i-1][j], math.Min(cost[i][j-1], cost[i-1][j-1]))
		}
	}
	if math.IsInf(cost[rows-1][columns-1], 1) {
		return 0, nil, fmt.Errorf("DTW window does not permit a complete alignment")
	}
	i, j := len(actual), len(reference)
	reversed := make([]AlignmentPair, 0, len(actual)+len(reference))
	for i > 0 && j > 0 {
		reversed = append(reversed, AlignmentPair{ActualIndex: i - 1, ReferenceIndex: j - 1})
		diagonal := cost[i-1][j-1]
		up := cost[i-1][j]
		left := cost[i][j-1]
		switch {
		case diagonal <= up && diagonal <= left:
			i--
			j--
		case up <= left:
			i--
		default:
			j--
		}
	}
	for i > 0 {
		i--
		reversed = append(reversed, AlignmentPair{ActualIndex: i, ReferenceIndex: 0})
	}
	for j > 0 {
		j--
		reversed = append(reversed, AlignmentPair{ActualIndex: 0, ReferenceIndex: j})
	}
	path := make([]AlignmentPair, len(reversed))
	for index := range reversed {
		path[len(reversed)-1-index] = reversed[index]
	}
	return cost[len(actual)][len(reference)] / float64(len(path)), path, nil
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
