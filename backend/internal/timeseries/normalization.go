package timeseries
import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)
type ChannelScale struct {
	Median float64 `json:"median"`
	IQR    float64 `json:"iqr"`
	Scale  float64 `json:"scale"`
	Count  int     `json:"count"`
}
type NormalizationSummary struct {
	Method   string                  `json:"method"`
	Channels map[string]ChannelScale `json:"channels"`
}
func Normalize(points []Point) ([]Point, NormalizationSummary, error) {
	if len(points) == 0 {
		return nil, NormalizationSummary{}, fmt.Errorf("cannot normalize an empty time series")
	}
	values := make(map[string][]float64)
	for _, point := range points {
		for channel, value := range point.Values {
			if value != nil {
				values[channel] = append(values[channel], *value)
			}
		}
	}
	summary := NormalizationSummary{Method: "median_iqr", Channels: make(map[string]ChannelScale, len(values))}
	for channel, samples := range values {
		if len(samples) < 2 {
			return nil, NormalizationSummary{}, fmt.Errorf("channel %s needs at least two observed values", channel)
		}
		sort.Float64s(samples)
		median := quantile(samples, 0.5)
		iqr := quantile(samples, 0.75) - quantile(samples, 0.25)
		scale := iqr
		if math.Abs(scale) < 1e-12 {
			deviations := make([]float64, len(samples))
			for i, value := range samples {
				deviations[i] = math.Abs(value - median)
			}
			sort.Float64s(deviations)
			scale = quantile(deviations, 0.5) * 1.349
		}
		if math.Abs(scale) < 1e-12 {
			scale = 1
		}
		summary.Channels[channel] = ChannelScale{
			Median: round(median, 8), IQR: round(iqr, 8), Scale: round(scale, 8), Count: len(samples),
		}
	}
	normalized := make([]Point, len(points))
	for i, point := range points {
		normalized[i] = Point{Timestamp: point.Timestamp, Values: make(map[string]*float64, len(point.Values))}
		for channel, value := range point.Values {
			if value == nil {
				normalized[i].Values[channel] = nil
				continue
			}
			stats := summary.Channels[channel]
			scaled := round((*value-stats.Median)/stats.Scale, 8)
			normalized[i].Values[channel] = &scaled
		}
	}
	return normalized, summary, nil
}
func EncodeNormalization(summary NormalizationSummary) (string, error) {
	data, err := json.Marshal(summary)
	if err != nil {
		return "", fmt.Errorf("encode normalization summary: %w", err)
	}
	return string(data), nil
}
func quantile(sorted []float64, probability float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	position := probability * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	weight := position - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
