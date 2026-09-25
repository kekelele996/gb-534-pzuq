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
type QualitySummary struct {
	OriginalPointCount int                `json:"original_point_count"`
	UniquePointCount   int                `json:"unique_point_count"`
	DuplicateCount     int                `json:"duplicate_count"`
	LongGapCount       int                `json:"long_gap_count"`
	MaxGapSeconds      int64              `json:"max_gap_seconds"`
	MissingRate        map[string]float64 `json:"missing_rate"`
	Channels           []string           `json:"channels"`
	Warnings           []string           `json:"warnings"`
	Valid              bool               `json:"valid"`
}
type wirePoint struct {
	Timestamp string              `json:"timestamp"`
	Values    map[string]*float64 `json:"values"`
	Value     *float64            `json:"value"`
}
func Validate(raw []byte, primaryChannel string, sampleIntervalSeconds int) ([]Point, QualitySummary, error) {
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
		if rate > 0.35 {
			summary.Valid = false
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("%s missing rate %.1f%% exceeds 35%%", channel, rate*100))
		} else if rate > 0.10 {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("%s missing rate %.1f%% requires review", channel, rate*100))
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
