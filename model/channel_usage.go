package model

import (
	"fmt"
	"sort"

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

	// The breakdown comes from the usage logs, not from quota_data, so it is zero for
	// periods whose logs were purged. For every request
	// input + output + cache read + cache write == prompt + completion + cache tokens
	// that quota_data counts into TokenUsed.
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`
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

// splitLogTokens splits one consume log into input, output, cache-read and cache-write
// tokens. Anthropic-style usage reports cache tokens next to the prompt; OpenAI-style usage
// counts them inside the prompt, so they are carved out of it to keep the four parts
// disjoint.
func splitLogTokens(promptTokens int, completionTokens int, other string) (input, output, cacheRead, cacheWrite int64) {
	var meta struct {
		UsageSemantic       string `json:"usage_semantic"`
		CacheTokens         any    `json:"cache_tokens"`
		CacheCreationTokens any    `json:"cache_creation_tokens"`
	}
	if other != "" {
		_ = common.UnmarshalJsonStr(other, &meta)
	}
	cacheRead = int64(logOtherInt(meta.CacheTokens))
	cacheWrite = int64(logOtherInt(meta.CacheCreationTokens))
	input = int64(max(promptTokens, 0))
	if meta.UsageSemantic != "anthropic" {
		input = max(input-cacheRead-cacheWrite, 0)
	}
	return input, int64(max(completionTokens, 0)), cacheRead, cacheWrite
}

// GetChannelTokenBreakdown splits one channel's consumed tokens over [startTime, endTime]
// into input, output, cache-read and cache-write tokens per bucket (oldest first), read from
// the usage logs. Only the four breakdown fields of the returned buckets are filled.
func GetChannelTokenBreakdown(channelID int, startTime int64, endTime int64, bucketSize int64, offsetSeconds int64) ([]ChannelUsageBucket, error) {
	if bucketSize <= 0 {
		bucketSize = 3600
	}
	rows, err := LOG_DB.Table("logs").
		Select("created_at, prompt_tokens, completion_tokens, other").
		Where("type = ? AND channel_id = ?", LogTypeConsume, channelID).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byBucket := make(map[int64]*ChannelUsageBucket)
	for rows.Next() {
		var (
			createdAt  int64
			prompt     int
			completion int
			other      *string
		)
		if err := rows.Scan(&createdAt, &prompt, &completion, &other); err != nil {
			return nil, err
		}
		shifted := createdAt + offsetSeconds
		start := shifted - ((shifted%bucketSize)+bucketSize)%bucketSize - offsetSeconds
		bucket, ok := byBucket[start]
		if !ok {
			bucket = &ChannelUsageBucket{Bucket: start}
			byBucket[start] = bucket
		}
		otherJSON := ""
		if other != nil {
			otherJSON = *other
		}
		input, output, cacheRead, cacheWrite := splitLogTokens(prompt, completion, otherJSON)
		bucket.InputTokens += input
		bucket.OutputTokens += output
		bucket.CacheReadTokens += cacheRead
		bucket.CacheWriteTokens += cacheWrite
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]ChannelUsageBucket, 0, len(byBucket))
	for _, bucket := range byBucket {
		result = append(result, *bucket)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Bucket < result[j].Bucket })
	return result, nil
}
