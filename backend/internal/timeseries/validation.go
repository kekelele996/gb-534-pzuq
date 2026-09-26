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

// PhaseWindow mirrors a recipe phase boundary expressed in elapsed hours
// relative to the start of the series.
type PhaseWindow struct {
	Phase     string  `json:"phase"`
	StartHour float64 `json:"start_hour"`
	EndHour   float64 `json:"end_hour"`
}

// PhaseQuality reports per-channel missing rates inside one recipe phase.
type PhaseQuality struct {
	Phase             string             `json:"phase"`
	StartHour         float64            `json:"start_hour"`
	EndHour           float64            `json:"end_hour"`
	SampleCount       int                `json:"sample_count"`
	MissingRate       map[string]float64 `json:"missing_rate"`
	WorstChannel      string             `json:"worst_channel"`
	WorstMissingRate  float64            `json:"worst_missing_rate"`
	Enforced          bool               `json:"enforced"`
}

type QualitySummary struct {
	OriginalPointCount int                `json:"original_point_count"`
	UniquePointCount   int                `json:"unique_point_count"`
	DuplicateCount     int                `json:"duplicate_count"`
	LongGapCount       int                `json:"long_gap_count"`
	MaxGapSeconds      int64              `json:"max_gap_seconds"`
	MissingRate        map[string]float64 `json:"missing_rate"`
	PhaseQuality       []PhaseQuality     `json:"phase_quality,omitempty"`
	Channels           []string           `json:"channels"`
	Warnings           []string           `json:"warnings"`
	RejectionReason    string             `json:"rejection_reason,omitempty"`
	Valid              bool               `json:"valid"`
}

type wirePoint struct {
	Timestamp string              `json:"timestamp"`
	Values    map[string]*float64 `json:"values"`
	Value     *float64            `json:"value"`
}

const (
	overallMissingLimit = 0.35
	reviewMissingLevel  = 0.10
	phaseMissingLimit   = 0.20
)

// phaseEnforced holds the phases where any channel above the 20% phase limit
// must reject the series: growth and production.
var phaseEnforced = map[string]bool{"growth": true, "production": true}

func Validate(raw []byte, primaryChannel string, sampleIntervalSeconds int, phases []PhaseWindow) ([]Point, QualitySummary, error) {
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
	failures := make([]string, 0)
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
		if rate > overallMissingLimit {
			summary.Valid = false
			message := fmt.Sprintf("%s missing rate %.1f%% exceeds %.0f%% across the whole series",
				channel, rate*100, overallMissingLimit*100)
			summary.Warnings = append(summary.Warnings, message)
			failures = append(failures, message)
		} else if rate > reviewMissingLevel {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("%s missing rate %.1f%% requires review", channel, rate*100))
		}
	}
	if len(phases) > 0 {
		summary.PhaseQuality = evaluatePhaseQuality(points, channelList, phases, &summary)
		for _, phase := range summary.PhaseQuality {
			if !phase.Enforced {
				continue
			}
			for _, channel := range channelList {
				rate := phase.MissingRate[channel]
				if rate > phaseMissingLimit {
					summary.Valid = false
					message := fmt.Sprintf("%s phase channel %s missing rate %.1f%% exceeds %.0f%%",
						phase.Phase, channel, rate*100, phaseMissingLimit*100)
					summary.Warnings = append(summary.Warnings, message)
					failures = append(failures, message)
				}
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
	if len(failures) > 0 {
		summary.RejectionReason = strings.Join(failures, "; ")
	}
	return points, summary, nil
}

// evaluatePhaseQuality computes the missing rate of every channel inside each
// recipe phase window. Elapsed hours are measured from the first observation,
// matching the phase membership used by the deviation evaluator.
func evaluatePhaseQuality(points []Point, channels []string, phases []PhaseWindow, summary *QualitySummary) []PhaseQuality {
	startedAt := points[0].Timestamp
	results := make([]PhaseQuality, 0, len(phases))
	for _, phase := range phases {
		inPhase := make([]Point, 0)
		for _, point := range points {
			elapsed := point.Timestamp.Sub(startedAt).Hours()
			if elapsed >= phase.StartHour && elapsed <= phase.EndHour {
				inPhase = append(inPhase, point)
			}
		}
		rates := make(map[string]float64, len(channels))
		worstChannel, worstRate := "", 0.0
		if len(inPhase) > 0 {
			for _, channel := range channels {
				missing := 0
				for _, point := range inPhase {
					value, ok := point.Values[channel]
					if !ok || value == nil {
						missing++
					}
				}
				rate := float64(missing) / float64(len(inPhase))
				rates[channel] = round(rate, 6)
				if rate > worstRate || (rate == worstRate && (worstChannel == "" || channel < worstChannel)) {
					worstChannel, worstRate = channel, rate
				}
			}
		} else {
			// A phase with no observations cannot be aligned; enforce the
			// strict growth/production gate and surface it in every report.
			for _, channel := range channels {
				rates[channel] = 1
			}
			worstChannel, worstRate = channels[0], 1
		}
		enforced := phaseEnforced[strings.ToLower(phase.Phase)]
		results = append(results, PhaseQuality{
			Phase:            phase.Phase,
			StartHour:        phase.StartHour,
			EndHour:          phase.EndHour,
			SampleCount:      len(inPhase),
			MissingRate:      rates,
			WorstChannel:     worstChannel,
			WorstMissingRate: round(worstRate, 6),
			Enforced:         enforced,
		})
		if enforced && worstRate > reviewMissingLevel {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf(
				"%s phase worst channel is %s with %.1f%% missing", phase.Phase, worstChannel, worstRate*100))
		}
	}
	return results
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
