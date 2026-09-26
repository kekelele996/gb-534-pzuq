package timeseries
import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)
type Point struct {
	Timestamp time.Time           `json:"timestamp"`
	Values    map[string]*float64 `json:"values"`
}
// PhaseWindow is the elapsed-hour boundary of one recipe phase, measured from
// the first observation. Boundaries are treated as inclusive on both ends.
type PhaseWindow struct {
	Phase     string  `json:"phase"`
	StartHour float64 `json:"start_hour"`
	EndHour   float64 `json:"end_hour"`
}
type PhaseMissingSummary struct {
	Phase              string             `json:"phase"`
	ObservedPointCount int                `json:"observed_point_count"`
	MissingRate        map[string]float64 `json:"missing_rate"`
	WorstChannel       string             `json:"worst_channel"`
	WorstMissingRate   float64            `json:"worst_missing_rate"`
}
type QualitySummary struct {
	OriginalPointCount int                     `json:"original_point_count"`
	UniquePointCount   int                     `json:"unique_point_count"`
	DuplicateCount     int                     `json:"duplicate_count"`
	LongGapCount       int                     `json:"long_gap_count"`
	MaxGapSeconds      int64                   `json:"max_gap_seconds"`
	MissingRate        map[string]float64     `json:"missing_rate"`
	PhaseMissing       []PhaseMissingSummary   `json:"phase_missing,omitempty"`
	Channels           []string                `json:"channels"`
	Warnings           []string                `json:"warnings"`
	RejectionReason    string                  `json:"rejection_reason,omitempty"`
	Valid              bool                    `json:"valid"`
}
type wirePoint struct {
	Timestamp string              `json:"timestamp"`
	Values    map[string]*float64 `json:"values"`
	Value     *float64            `json:"value"`
}
const (
	// OverallMissingLimit rejects any channel whose whole-run missing rate
	// exceeds this share.
	OverallMissingLimit = 0.35
	// CriticalPhaseMissingLimit rejects any channel in growth or production
	// whose in-phase missing rate exceeds this share.
	CriticalPhaseMissingLimit = 0.20
)
// Validate decodes, sorts and deduplicates the observations, then measures
// channel missing rates over the whole run and, when the recipe's four phase
// windows are supplied, within each phase. Growth and production enforce a
// stricter per-channel limit; the remaining phases are covered by the
// whole-run limit.
func Validate(
	raw []byte, primaryChannel string, sampleIntervalSeconds int, phases []PhaseWindow,
) ([]Point, QualitySummary, error) {
	var input []wirePoint
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, QualitySummary{}, fmt.Errorf("decode points_json: %w", err)
	}
	if len(input) < 4 {
		return nil, QualitySummary{}, fmt.Errorf("points_json must contain at least four observations")
	}
	if sampleIntervalSeconds < 1 {
		return nil, QualitySummary{}, fmt.Errorf("sample interval must be positive")
	}
	primaryChannel = strings.ToLower(strings.TrimSpace(primaryChannel))
	latest := make(map[int64]Point, len(input))
	channels := make(map[string]struct{})
	for index, rawPoint := range input {
		timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(rawPoint.Timestamp))
		if err != nil {
			return nil, QualitySummary{}, fmt.Errorf("point %d timestamp must use RFC3339: %w", index, err)
		}
		values := make(map[string]*float64, len(rawPoint.Values)+1)
		for channel, value := range rawPoint.Values {
			channel = strings.ToLower(strings.TrimSpace(channel))
			if channel == "" {
				return nil, QualitySummary{}, fmt.Errorf("point %d contains an empty channel name", index)
			}
			if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
				return nil, QualitySummary{}, fmt.Errorf("point %d channel %s is not finite", index, channel)
			}
			values[channel] = value
			channels[channel] = struct{}{}
		}
		if rawPoint.Value != nil && primaryChannel != "" {
			value := *rawPoint.Value
			values[primaryChannel] = &value
			channels[primaryChannel] = struct{}{}
		}
		if len(values) == 0 {
			return nil, QualitySummary{}, fmt.Errorf("point %d has no channel values", index)
		}
		latest[timestamp.UTC().UnixNano()] = Point{Timestamp: timestamp.UTC(), Values: values}
	}
	if len(channels) == 0 {
		return nil, QualitySummary{}, fmt.Errorf("points_json has no channels")
	}
	points := make([]Point, 0, len(latest))
	for _, point := range latest {
		points = append(points, point)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Timestamp.Before(points[j].Timestamp) })
	channelList := make([]string, 0, len(channels))
	for channel := range channels {
		channelList = append(channelList, channel)
	}
	sort.Strings(channelList)
	summary := QualitySummary{
		OriginalPointCount: len(input), UniquePointCount: len(points),
		DuplicateCount: len(input) - len(points), MissingRate: make(map[string]float64, len(channelList)),
		Channels: channelList, Warnings: []string{}, Valid: true,
	}
	if len(points) < 4 {
		return nil, QualitySummary{}, fmt.Errorf("deduplication left fewer than four unique observations")
	}
	for _, channel := range channelList {
		missing := 0
		for _, point := range points {
			value, ok := point.Values[channel]
			if !ok || value == nil {
				missing++
			}
		}
		rate := float64(missing) / float64(len(points))
		summary.MissingRate[channel] = round(rate, 6)
		if rate > OverallMissingLimit {
			summary.Valid = false
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("%s missing rate %.1f%% exceeds %.0f%%", channel, rate*100, OverallMissingLimit*100))
		} else if rate > 0.10 {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("%s missing rate %.1f%% requires review", channel, rate*100))
		}
	}
	if len(phases) > 0 {
		startedAt := points[0].Timestamp
		summary.PhaseMissing = make([]PhaseMissingSummary, 0, len(phases))
		for _, phase := range phases {
			phaseSummary := summarizePhaseMissing(points, startedAt, phase, channelList)
			summary.PhaseMissing = append(summary.PhaseMissing, phaseSummary)
			if phase.Phase != "growth" && phase.Phase != "production" {
				continue
			}
			if phaseSummary.ObservedPointCount == 0 {
				summary.Valid = false
				summary.Warnings = append(summary.Warnings,
					fmt.Sprintf("%s phase contains no observations; every channel exceeds %.0f%% missing", phase.Phase, CriticalPhaseMissingLimit*100))
				continue
			}
			if phaseSummary.WorstMissingRate > CriticalPhaseMissingLimit {
				summary.Valid = false
				summary.Warnings = append(summary.Warnings,
					fmt.Sprintf("%s phase channel %s missing rate %.1f%% exceeds %.0f%%",
						phase.Phase, phaseSummary.WorstChannel, phaseSummary.WorstMissingRate*100, CriticalPhaseMissingLimit*100))
			}
		}
	}
	expected := time.Duration(sampleIntervalSeconds) * time.Second
	for i := 1; i < len(points); i++ {
		gap := points[i].Timestamp.Sub(points[i-1].Timestamp)
		if seconds := int64(gap.Seconds()); seconds > summary.MaxGapSeconds {
			summary.MaxGapSeconds = seconds
		}
		if gap > 3*expected {
			summary.LongGapCount++
		}
	}
	if summary.LongGapCount > 0 {
		summary.Warnings = append(summary.Warnings,
			fmt.Sprintf("%d long gaps were preserved without interpolation", summary.LongGapCount))
	}
	if summary.DuplicateCount > 0 {
		summary.Warnings = append(summary.Warnings,
			fmt.Sprintf("%d duplicate timestamps were deterministically replaced by the last observation", summary.DuplicateCount))
	}
	return points, summary, nil
}
// summarizePhaseMissing measures each channel's missing rate among the
// observations whose elapsed time falls inside the phase window. The worst
// channel follows channel order so the result is deterministic.
func summarizePhaseMissing(points []Point, startedAt time.Time, phase PhaseWindow, channelList []string) PhaseMissingSummary {
	summary := PhaseMissingSummary{Phase: phase.Phase, MissingRate: make(map[string]float64, len(channelList))}
	inPhase := make([]Point, 0)
	for _, point := range points {
		elapsedHours := point.Timestamp.Sub(startedAt).Hours()
		if elapsedHours >= phase.StartHour && elapsedHours <= phase.EndHour {
			inPhase = append(inPhase, point)
		}
	}
	summary.ObservedPointCount = len(inPhase)
	for _, channel := range channelList {
		if len(inPhase) == 0 {
			summary.MissingRate[channel] = 1
			continue
		}
		missing := 0
		for _, point := range inPhase {
			value, ok := point.Values[channel]
			if !ok || value == nil {
				missing++
			}
		}
		summary.MissingRate[channel] = round(float64(missing)/float64(len(inPhase)), 6)
		if rate := summary.MissingRate[channel]; rate > summary.WorstMissingRate || summary.WorstChannel == "" {
			summary.WorstChannel = channel
			summary.WorstMissingRate = rate
		}
	}
	return summary
}
func EncodePoints(points []Point) (string, error) {
	data, err := json.Marshal(points)
	if err != nil {
		return "", fmt.Errorf("encode canonical points: %w", err)
	}
	return string(data), nil
}
func DecodePoints(raw string) ([]Point, error) {
	var points []Point
	if err := json.Unmarshal([]byte(raw), &points); err != nil {
		return nil, fmt.Errorf("decode canonical points: %w", err)
	}
	if len(points) == 0 {
		return nil, fmt.Errorf("canonical points are empty")
	}
	return points, nil
}
func round(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}
