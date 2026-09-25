package constants
type FermentationPhase string
const (
	PhaseLag        FermentationPhase = "lag"
	PhaseGrowth     FermentationPhase = "growth"
	PhaseProduction FermentationPhase = "production"
	PhaseHarvest    FermentationPhase = "harvest"
)
func (p FermentationPhase) Valid() bool {
	switch p {
	case PhaseLag, PhaseGrowth, PhaseProduction, PhaseHarvest:
		return true
	default:
		return false
	}
}
func FermentationPhaseValues() []string {
	return []string{string(PhaseLag), string(PhaseGrowth), string(PhaseProduction), string(PhaseHarvest)}
}
