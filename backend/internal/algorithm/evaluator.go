package algorithm
import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/model"
	"fermentation-kinetics-deviation-analysis/backend/internal/timeseries"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
)
const Version = "phase-dtw-v1.0.0"
type PhaseBoundary struct {
	Phase     constants.FermentationPhase `json:"phase"`
	StartHour float64                     `json:"start_hour"`
	EndHour   float64                     `json:"end_hour"`
}
type CurvePoint struct {
	ElapsedHour float64 `json:"elapsed_h"`
	Value       float64 `json:"value"`
}
type ChannelTolerance struct {
	Weight      float64 `json:"weight"`
	MaxDistance float64 `json:"max_distance"`
}
type Snapshot struct {
	SeriesID             uint      `json:"series_id"`
	VesselID             uint      `json:"vessel_id"`
	RecipeID             uint      `json:"recipe_id"`
	RecipeVersion        int       `json:"recipe_version"`
	RunCode              string    `json:"run_code"`
	Channel              string    `json:"channel"`
	SampleIntervalS      int       `json:"sample_interval_s"`
	PointsJSON           string    `json:"points_json"`
	SourceChecksum       string    `json:"source_checksum"`
	StartedAt            time.Time `json:"started_at"`
	EndedAt              time.Time `json:"ended_at"`
	PhaseBoundariesJSON  string    `json:"phase_boundaries_json"`
	ReferenceCurvesJSON  string    `json:"reference_curves_json"`
	ToleranceProfileJSON string    `json:"tolerance_profile_json"`
	AlgorithmVersion     string    `json:"algorithm_version"`
}
type PhaseEvidence struct {
	Phase             string             `json:"phase"`
	DurationDeviation float64            `json:"duration_deviation"`
	SlopeDeviation    float64            `json:"slope_deviation"`
	PeakTimeDeviation float64            `json:"peak_time_deviation"`
	CurveDistance     float64            `json:"curve_distance"`
	WeightedDeviation float64            `json:"weighted_deviation"`
	ChannelScores     map[string]float64 `json:"channel_scores"`
	ObservedPoints    int                `json:"observed_points"`
}
type AlignedPoint struct {
	Phase                string  `json:"phase"`
	Channel              string  `json:"channel"`
	ActualElapsedHour    float64 `json:"actual_elapsed_h"`
	ActualValue          float64 `json:"actual_value"`
	ReferenceElapsedHour float64 `json:"reference_elapsed_h"`
	ReferenceValue       float64 `json:"reference_value"`
}
type Result struct {
	PhaseScoresJSON     string
	DeviationLevel      constants.DeviationLevel
	AlignedCurveJSON    string
	SuspectedCausesJSON string
	Explanation         string
	OverallScore        float64
}
type Evaluator struct{}
func NewEvaluator() *Evaluator { return &Evaluator{} }
func NewSnapshot(series model.SensorSeries, recipe model.CultureRecipe) Snapshot {
	return Snapshot{
		SeriesID: series.ID, VesselID: series.VesselID, RecipeID: recipe.ID, RecipeVersion: recipe.Version,
		RunCode: series.RunCode, Channel: series.Channel, SampleIntervalS: series.SampleIntervalS,
		PointsJSON: series.PointsJSON, SourceChecksum: series.SourceChecksum,
		StartedAt: series.StartedAt.UTC(), EndedAt: series.EndedAt.UTC(),
		PhaseBoundariesJSON: recipe.PhaseBoundariesJSON, ReferenceCurvesJSON: recipe.ReferenceCurvesJSON,
		ToleranceProfileJSON: recipe.ToleranceProfileJSON, AlgorithmVersion: Version,
	}
}
func (s Snapshot) Canonical() (string, error) { return util.CanonicalJSON(s) }
func (s Snapshot) Hash() (string, error) {
	canonical, err := s.Canonical()
	if err != nil {
		return "", err
	}
	return util.HashString(canonical), nil
}
func DecodeSnapshot(raw string) (Snapshot, error) {
	var snapshot Snapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode analysis snapshot: %w", err)
	}
	if snapshot.AlgorithmVersion != Version {
		return Snapshot{}, fmt.Errorf("snapshot algorithm %s is not supported by %s", snapshot.AlgorithmVersion, Version)
	}
	return snapshot, nil
}
func ValidateRecipeConfiguration(boundariesRaw, curvesRaw, toleranceRaw []byte, targetDuration float64) error {
	boundaries, curves, tolerances, err := parseConfiguration(boundariesRaw, curvesRaw, toleranceRaw)
	if err != nil {
		return err
	}
	if targetDuration <= 0 {
		return fmt.Errorf("target duration must be positive")
	}
	expected := constants.FermentationPhaseValues()
	if len(boundaries) != len(expected) {
		return fmt.Errorf("phase boundaries must contain exactly lag, growth, production, and harvest")
	}
	for i, boundary := range boundaries {
		if string(boundary.Phase) != expected[i] {
			return fmt.Errorf("phase %d must be %s", i+1, expected[i])
		}
		if boundary.StartHour < 0 || boundary.EndHour <= boundary.StartHour {
			return fmt.Errorf("phase %s has an invalid time range", boundary.Phase)
		}
		if i > 0 && math.Abs(boundary.StartHour-boundaries[i-1].EndHour) > 1e-6 {
			return fmt.Errorf("phase %s must start when the previous phase ends", boundary.Phase)
		}
	}
	if math.Abs(boundaries[len(boundaries)-1].EndHour-targetDuration) > 1e-6 {
		return fmt.Errorf("final phase must end at target_duration_h")
	}
	if len(curves) == 0 {
		return fmt.Errorf("reference curves must contain at least one channel")
	}
	for channel, points := range curves {
		if strings.TrimSpace(channel) == "" || len(points) < 4 {
			return fmt.Errorf("reference channel %s must contain at least four points", channel)
		}
		for i, point := range points {
			if i > 0 && point.ElapsedHour <= points[i-1].ElapsedHour {
				return fmt.Errorf("reference channel %s elapsed_h values must increase", channel)
			}
			if point.ElapsedHour < 0 || point.ElapsedHour > targetDuration {
				return fmt.Errorf("reference channel %s has a point outside the target duration", channel)
			}
		}
		if tolerance, ok := tolerances[channel]; ok {
			if tolerance.Weight <= 0 || tolerance.MaxDistance <= 0 {
				return fmt.Errorf("tolerance for channel %s must have positive weight and max_distance", channel)
			}
		}
	}
	return nil
}
func (e *Evaluator) Evaluate(snapshot Snapshot) (Result, error) {
	if snapshot.AlgorithmVersion != Version {
		return Result{}, fmt.Errorf("unsupported algorithm version %q", snapshot.AlgorithmVersion)
	}
	points, err := timeseries.DecodePoints(snapshot.PointsJSON)
	if err != nil {
		return Result{}, err
	}
	boundaries, references, tolerances, err := parseConfiguration(
		[]byte(snapshot.PhaseBoundariesJSON), []byte(snapshot.ReferenceCurvesJSON), []byte(snapshot.ToleranceProfileJSON),
	)
	if err != nil {
		return Result{}, err
	}
	evidence := make([]PhaseEvidence, 0, len(boundaries))
	aligned := make([]AlignedPoint, 0, len(points)*2)
	causes := make(map[string]string)
	overallWeighted, overallWeight := 0.0, 0.0
	for _, boundary := range boundaries {
		phaseEvidence, phaseAligned, phaseCauses, weight, phaseErr := evaluatePhase(
			points, snapshot.StartedAt, boundary, references, tolerances,
		)
		if phaseErr != nil {
			return Result{}, phaseErr
		}
		evidence = append(evidence, phaseEvidence)
		aligned = append(aligned, phaseAligned...)
		for key, cause := range phaseCauses {
			causes[key] = cause
		}
		overallWeighted += phaseEvidence.WeightedDeviation * weight
		overallWeight += weight
	}
	if overallWeight == 0 {
		return Result{}, fmt.Errorf("no comparable channel observations were found")
	}
	overall := clamp(overallWeighted / overallWeight)
	level := constants.DeviationLevelForScore(overall)
	causeList := make([]string, 0, len(causes))
	for _, cause := range causes {
		causeList = append(causeList, cause)
	}
	sort.Strings(causeList)
	phaseJSON, err := json.Marshal(evidence)
	if err != nil {
		return Result{}, fmt.Errorf("encode phase evidence: %w", err)
	}
	alignedJSON, err := json.Marshal(aligned)
	if err != nil {
		return Result{}, fmt.Errorf("encode aligned curve: %w", err)
	}
	causesJSON, err := json.Marshal(causeList)
	if err != nil {
		return Result{}, fmt.Errorf("encode suspected causes: %w", err)
	}
	explanation := fmt.Sprintf(
		"Deterministic phase-constrained DTW produced an overall deviation of %.3f (%s) across %d phases. "+
			"Long gaps and missing values remain explicit; this result supports offline review only and contains no equipment control instructions.",
		overall, level, len(evidence),
	)
	return Result{
		PhaseScoresJSON: string(phaseJSON), DeviationLevel: level, AlignedCurveJSON: string(alignedJSON),
		SuspectedCausesJSON: string(causesJSON), Explanation: explanation, OverallScore: overall,
	}, nil
}
func parseConfiguration(boundariesRaw, curvesRaw, toleranceRaw []byte) (
	[]PhaseBoundary, map[string][]CurvePoint, map[string]ChannelTolerance, error,
) {
	var boundaries []PhaseBoundary
	if err := json.Unmarshal(boundariesRaw, &boundaries); err != nil {
		return nil, nil, nil, fmt.Errorf("decode phase_boundaries_json: %w", err)
	}
	var curves map[string][]CurvePoint
	if err := json.Unmarshal(curvesRaw, &curves); err != nil {
		return nil, nil, nil, fmt.Errorf("decode reference_curves_json: %w", err)
	}
	var tolerances map[string]ChannelTolerance
	if err := json.Unmarshal(toleranceRaw, &tolerances); err != nil {
		return nil, nil, nil, fmt.Errorf("decode tolerance_profile_json: %w", err)
	}
	if tolerances == nil {
		tolerances = map[string]ChannelTolerance{}
	}
	return boundaries, curves, tolerances, nil
}
func evaluatePhase(
	points []timeseries.Point,
	startedAt time.Time,
	boundary PhaseBoundary,
	references map[string][]CurvePoint,
	tolerances map[string]ChannelTolerance,
) (PhaseEvidence, []AlignedPoint, map[string]string, float64, error) {
	evidence := PhaseEvidence{Phase: string(boundary.Phase), ChannelScores: map[string]float64{}}
	aligned := []AlignedPoint{}
	causes := map[string]string{}
	total, totalWeight := 0.0, 0.0
	durationScores, slopeScores, peakScores, distanceScores := []float64{}, []float64{}, []float64{}, []float64{}
	channels := make([]string, 0, len(references))
	for channel := range references {
		channels = append(channels, channel)
	}
	sort.Strings(channels)
	for _, channel := range channels {
		referenceAll := references[channel]
		actualTimes, actualValues := actualInPhase(points, startedAt, boundary, channel)
		referenceTimes, referenceValues := referenceInPhase(referenceAll, boundary)
		if len(actualValues) < 2 || len(referenceValues) < 2 {
			continue
		}
		median, scale := robustReferenceScale(referenceValues)
		actualScaled := scaleValues(actualValues, median, scale)
		referenceScaled := scaleValues(referenceValues, median, scale)
		distance, path, err := DTW(actualScaled, referenceScaled, maxInt(len(actualScaled), len(referenceScaled))/2+1)
		if err != nil {
			return PhaseEvidence{}, nil, nil, 0, fmt.Errorf("align phase %s channel %s: %w", boundary.Phase, channel, err)
		}
		tolerance := tolerances[channel]
		if tolerance.Weight <= 0 {
			tolerance.Weight = 1
		}
		if tolerance.MaxDistance <= 0 {
			tolerance.MaxDistance = 1
		}
		curveScore := clamp(distance / tolerance.MaxDistance)
		slopeScore := clamp(math.Abs(slope(actualTimes, actualValues)-slope(referenceTimes, referenceValues)) /
			(math.Abs(slope(referenceTimes, referenceValues)) + 0.1))
		peakScore := clamp(math.Abs(peakTime(actualTimes, actualValues)-peakTime(referenceTimes, referenceValues)) /
			math.Max(boundary.EndHour-boundary.StartHour, 0.001))
		actualDuration := actualTimes[len(actualTimes)-1] - actualTimes[0]
		referenceDuration := referenceTimes[len(referenceTimes)-1] - referenceTimes[0]
		durationScore := clamp(math.Abs(actualDuration-referenceDuration) / math.Max(referenceDuration, 0.001))
		channelScore := clamp(0.50*curveScore + 0.20*slopeScore + 0.15*peakScore + 0.15*durationScore)
		evidence.ChannelScores[channel] = round6(channelScore)
		total += channelScore * tolerance.Weight
		totalWeight += tolerance.Weight
		durationScores = append(durationScores, durationScore)
		slopeScores = append(slopeScores, slopeScore)
		peakScores = append(peakScores, peakScore)
		distanceScores = append(distanceScores, curveScore)
		evidence.ObservedPoints += len(actualValues)
		for _, pair := range path {
			aligned = append(aligned, AlignedPoint{
				Phase: string(boundary.Phase), Channel: channel,
				ActualElapsedHour: round6(actualTimes[pair.ActualIndex]), ActualValue: round6(actualValues[pair.ActualIndex]),
				ReferenceElapsedHour: round6(referenceTimes[pair.ReferenceIndex]), ReferenceValue: round6(referenceValues[pair.ReferenceIndex]),
			})
		}
		if channelScore >= 0.40 {
			direction := mean(actualScaled) - mean(referenceScaled)
			causes[channel] = causeFor(channel, direction, boundary.Phase)
		}
	}
	if totalWeight == 0 {
		return PhaseEvidence{}, nil, nil, 0, fmt.Errorf("phase %s has fewer than two comparable observations per channel", boundary.Phase)
	}
	evidence.DurationDeviation = round6(mean(durationScores))
	evidence.SlopeDeviation = round6(mean(slopeScores))
	evidence.PeakTimeDeviation = round6(mean(peakScores))
	evidence.CurveDistance = round6(mean(distanceScores))
	evidence.WeightedDeviation = round6(total / totalWeight)
	return evidence, aligned, causes, totalWeight, nil
}
func actualInPhase(points []timeseries.Point, startedAt time.Time, boundary PhaseBoundary, channel string) ([]float64, []float64) {
	times, values := []float64{}, []float64{}
	for _, point := range points {
		elapsed := point.Timestamp.Sub(startedAt).Hours()
		value := point.Values[channel]
		if elapsed >= boundary.StartHour && elapsed <= boundary.EndHour && value != nil {
			times = append(times, elapsed)
			values = append(values, *value)
		}
	}
	return times, values
}
func referenceInPhase(points []CurvePoint, boundary PhaseBoundary) ([]float64, []float64) {
	times, values := []float64{}, []float64{}
	for _, point := range points {
		if point.ElapsedHour >= boundary.StartHour && point.ElapsedHour <= boundary.EndHour {
			times = append(times, point.ElapsedHour)
			values = append(values, point.Value)
		}
	}
	return times, values
}
func robustReferenceScale(values []float64) (float64, float64) {
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	median := interpolateQuantile(copyValues, 0.5)
	scale := interpolateQuantile(copyValues, 0.75) - interpolateQuantile(copyValues, 0.25)
	if math.Abs(scale) < 1e-9 {
		scale = 1
	}
	return median, scale
}
func interpolateQuantile(sorted []float64, probability float64) float64 {
	position := probability * float64(len(sorted)-1)
	lower, upper := int(math.Floor(position)), int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	return sorted[lower]*(float64(upper)-position) + sorted[upper]*(position-float64(lower))
}
func scaleValues(values []float64, median, scale float64) []float64 {
	result := make([]float64, len(values))
	for i, value := range values {
		result[i] = (value - median) / scale
	}
	return result
}
func slope(times, values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	delta := times[len(times)-1] - times[0]
	if math.Abs(delta) < 1e-12 {
		return 0
	}
	return (values[len(values)-1] - values[0]) / delta
}
func peakTime(times, values []float64) float64 {
	index := 0
	for i := 1; i < len(values); i++ {
		if values[i] > values[index] {
			index = i
		}
	}
	return times[index]
}
func causeFor(channel string, direction float64, phase constants.FermentationPhase) string {
	position := "above"
	if direction < 0 {
		position = "below"
	}
	switch strings.ToLower(channel) {
	case "do", "dissolved_oxygen", "oxygen":
		return fmt.Sprintf("%s dissolved oxygen trajectory is %s the recipe reference; review offline biomass and gas-transfer evidence.", phase, position)
	case "ph":
		return fmt.Sprintf("%s pH trajectory is %s the recipe reference; review offline metabolite and sampling evidence.", phase, position)
	case "temperature":
		return fmt.Sprintf("%s temperature trajectory is %s the recipe reference; review offline batch and sensor-calibration evidence.", phase, position)
	case "agitation", "rpm":
		return fmt.Sprintf("%s agitation observation is %s the recipe reference; review historical process and sensor evidence.", phase, position)
	default:
		return fmt.Sprintf("%s %s trajectory is %s the recipe reference; review offline process evidence.", phase, channel, position)
	}
}
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}
func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
func round6(value float64) float64 { return math.Round(value*1e6) / 1e6 }
