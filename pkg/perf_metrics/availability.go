package perfmetrics

import (
	"cmp"
	"slices"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// Availability counts are exposed only by the authenticated availability API.
// Public model-square metrics continue to omit traffic volumes.
type AvailabilityPoint struct {
	Ts           int64 `json:"ts"`
	RequestCount int64 `json:"request_count"`
	SuccessCount int64 `json:"success_count"`
}

type AvailabilityModel struct {
	ModelName    string              `json:"model_name"`
	Icon         string              `json:"icon,omitempty"`
	RequestCount int64               `json:"request_count"`
	Series       []AvailabilityPoint `json:"series"`
}

type AvailabilityResult struct {
	WindowStart int64               `json:"window_start"`
	WindowEnd   int64               `json:"window_end"`
	Models      []AvailabilityModel `json:"models"`
}

func QueryAvailability(hours int, groups []string) (AvailabilityResult, error) {
	startTs, endTs := queryWindow(time.Now(), hours)
	rows, err := model.GetPerfMetricsSummaryBucketsAll(startTs, endTs, groups)
	if err != nil {
		return AvailabilityResult{}, err
	}

	buckets := map[string]map[int64]counters{}
	for _, row := range rows {
		mergeModelBucket(buckets, row.ModelName, row.BucketTs-row.BucketTs%3600, counters{
			requestCount: row.RequestCount,
			successCount: row.SuccessCount,
		})
	}
	allowedGroups := allowedGroupSet(groups)
	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs < startTs || k.bucketTs > endTs {
			return true
		}
		if allowedGroups != nil {
			if _, allowed := allowedGroups[k.group]; !allowed {
				return true
			}
		}
		mergeModelBucket(buckets, k.model, k.bucketTs-k.bucketTs%3600, value.(*atomicBucket).snapshot())
		return true
	})

	models := make([]AvailabilityModel, 0, len(buckets))
	for name, hourly := range buckets {
		entry := AvailabilityModel{ModelName: name, Series: make([]AvailabilityPoint, 0, len(hourly))}
		for ts, count := range hourly {
			entry.RequestCount += count.requestCount
			entry.Series = append(entry.Series, AvailabilityPoint{
				Ts: ts, RequestCount: count.requestCount, SuccessCount: count.successCount,
			})
		}
		slices.SortFunc(entry.Series, func(a, b AvailabilityPoint) int { return cmp.Compare(a.Ts, b.Ts) })
		models = append(models, entry)
	}
	slices.SortFunc(models, func(a, b AvailabilityModel) int {
		if order := cmp.Compare(b.RequestCount, a.RequestCount); order != 0 {
			return order
		}
		return cmp.Compare(a.ModelName, b.ModelName)
	})
	return AvailabilityResult{WindowStart: startTs, WindowEnd: endTs, Models: models}, nil
}
