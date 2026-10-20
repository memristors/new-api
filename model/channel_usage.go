package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// ChannelUsageBucket is one point of a channel's consumption trend. Token counts include
// prompt-cache reads and writes for Anthropic-style usage (see quotaDataCacheTokens).
type ChannelUsageBucket struct {
	// Bucket is the start of the bucket as a Unix timestamp.
	Bucket    int64 `json:"bucket"`
	TokenUsed int64 `json:"token_used"`
	Quota     int64 `json:"quota"`
	Count     int64 `json:"count"`
}

// GetChannelTokenTotals returns the cumulative token consumption of the given channels,
// summed from the hourly dashboard statistics (quota_data). Channels without any recorded
// usage are absent from the result.
func GetChannelTokenTotals(channelIDs []int) (map[int]int64, error) {
	totals := make(map[int]int64, len(channelIDs))
	if len(channelIDs) == 0 {
		return totals, nil
	}
	var rows []struct {
		ChannelID int
		TokenUsed int64
	}
	err := DB.Table("quota_data").
		Select("channel_id, sum(token_used) as token_used").
		Where("channel_id IN ?", channelIDs).
		Group("channel_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		totals[row.ChannelID] = row.TokenUsed
	}
	return totals, nil
}

// channelUsageBucketExpr buckets created_at into windows of bucketSize seconds that are
// aligned to the viewer's local midnight/hour: offsetSeconds is the viewer's UTC offset
// (east positive). Both arguments are integers formatted straight into the SQL text, so they
// must come from validated numeric input only.
func channelUsageBucketExpr(bucketSize int64, offsetSeconds int64) string {
	shifted := fmt.Sprintf("(created_at + %d)", offsetSeconds)
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		return fmt.Sprintf("FLOOR(%s / %d) * %d - %d", shifted, bucketSize, bucketSize, offsetSeconds)
	}
	return fmt.Sprintf("(%s / %d) * %d - %d", shifted, bucketSize, bucketSize, offsetSeconds)
}

// GetChannelUsageTrend aggregates one channel's token consumption, quota and request count
// over [startTime, endTime] into buckets of bucketSize seconds, oldest first. Buckets without
// traffic are not returned; the caller fills gaps when it needs a continuous axis.
func GetChannelUsageTrend(channelID int, startTime int64, endTime int64, bucketSize int64, offsetSeconds int64) ([]ChannelUsageBucket, error) {
	if bucketSize <= 0 {
		bucketSize = 3600
	}
	expr := channelUsageBucketExpr(bucketSize, offsetSeconds)
	var rows []ChannelUsageBucket
	err := DB.Table("quota_data").
		Select(fmt.Sprintf("%s as bucket, sum(token_used) as token_used, sum(quota) as quota, sum(count) as count", expr)).
		Where("channel_id = ?", channelID).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Group(expr).
		Order("bucket ASC").
		Find(&rows).Error
	return rows, err
}
