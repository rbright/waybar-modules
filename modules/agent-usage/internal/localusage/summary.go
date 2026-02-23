package localusage

import "github.com/rbright/waybar-agent-usage/internal/domain"

type dayBucket struct {
	tokens   int64
	costUSD  float64
	costSeen bool
}

func summarizeDays(days map[string]*dayBucket) domain.LocalUsageSummary {
	if len(days) == 0 {
		return domain.LocalUsageSummary{}
	}

	latest := ""
	for day := range days {
		if day > latest {
			latest = day
		}
	}

	var last30Tokens int64
	var last30Cost float64
	last30CostSeen := false
	for _, bucket := range days {
		last30Tokens += bucket.tokens
		if bucket.costSeen {
			last30Cost += bucket.costUSD
			last30CostSeen = true
		}
	}

	result := domain.LocalUsageSummary{}
	if latest != "" {
		latestBucket := days[latest]
		if latestBucket != nil {
			if latestBucket.tokens > 0 {
				result.TodayTokens = domain.Int64Ptr(latestBucket.tokens)
			}
			if latestBucket.costSeen {
				result.TodayCostUSD = domain.Float64Ptr(latestBucket.costUSD)
			}
		}
	}
	if last30Tokens > 0 {
		result.Last30Tokens = domain.Int64Ptr(last30Tokens)
	}
	if last30CostSeen {
		result.Last30CostUSD = domain.Float64Ptr(last30Cost)
	}

	return result
}
