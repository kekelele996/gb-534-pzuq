package constants
type DeviationLevel string
const (
	DeviationNormal   DeviationLevel = "normal"
	DeviationWatch    DeviationLevel = "watch"
	DeviationMajor    DeviationLevel = "major"
	DeviationCritical DeviationLevel = "critical"
)
func (d DeviationLevel) Valid() bool {
	switch d {
	case DeviationNormal, DeviationWatch, DeviationMajor, DeviationCritical:
		return true
	default:
		return false
	}
}
func DeviationLevelForScore(score float64) DeviationLevel {
	switch {
	case score >= 0.65:
		return DeviationCritical
	case score >= 0.40:
		return DeviationMajor
	case score >= 0.20:
		return DeviationWatch
	default:
		return DeviationNormal
	}
}
func DeviationLevelValues() []string {
	return []string{string(DeviationNormal), string(DeviationWatch), string(DeviationMajor), string(DeviationCritical)}
}
