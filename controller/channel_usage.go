package controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	channelUsageHour int64 = 3600
	channelUsageDay  int64 = 24 * 3600

	// channelUsageAutoHourlyMaxSpan is the longest range that is charted hourly when the
	// caller does not choose a granularity.
	channelUsageAutoHourlyMaxSpan = 3 * channelUsageDay
	channelUsageDefaultSpan       = channelUsageDay
	channelUsageMaxSpan           = 400 * channelUsageDay
	channelUsageMaxBuckets        = 1000
	channelUsageMaxTZOffsetMin    = 14 * 60
)

// channelUsageRange is the validated query of a channel consumption trend.
type channelUsageRange struct {
	Start         int64
	End           int64
	BucketSize    int64
	OffsetSeconds int64
	Granularity   string
}

// resolveChannelUsageRange validates the trend query. start and end are Unix seconds
// (default: the last 24 hours up to now); granularity is "hour", "day" or empty for automatic;
// tzOffsetMinutes is the viewer's UTC offset in minutes, east positive, used to align day
// buckets to the viewer's midnight.
func resolveChannelUsageRange(startRaw string, endRaw string, granularityRaw string, tzOffsetRaw string, now int64) (channelUsageRange, error) {
	result := channelUsageRange{End: now}
	if strings.TrimSpace(endRaw) != "" {
		end, err := strconv.ParseInt(strings.TrimSpace(endRaw), 10, 64)
		if err != nil || end <= 0 {
			return result, errors.New("invalid end_timestamp")
		}
		result.End = end
	}
	result.Start = result.End - channelUsageDefaultSpan
	if strings.TrimSpace(startRaw) != "" {
		start, err := strconv.ParseInt(strings.TrimSpace(startRaw), 10, 64)
		if err != nil || start < 0 {
			return result, errors.New("invalid start_timestamp")
		}
		result.Start = start
	}
	if result.Start >= result.End {
		return result, errors.New("start_timestamp must be earlier than end_timestamp")
	}
	span := result.End - result.Start
	if span > channelUsageMaxSpan {
		return result, fmt.Errorf("time range is too long (at most %d days)", channelUsageMaxSpan/channelUsageDay)
	}

	switch strings.ToLower(strings.TrimSpace(granularityRaw)) {
	case "hour":
		result.Granularity = "hour"
	case "day":
		result.Granularity = "day"
	case "":
		if span <= channelUsageAutoHourlyMaxSpan {
			result.Granularity = "hour"
		} else {
			result.Granularity = "day"
		}
	default:
		return result, errors.New("granularity must be hour or day")
	}
	result.BucketSize = channelUsageHour
	if result.Granularity == "day" {
		result.BucketSize = channelUsageDay
	}

	if strings.TrimSpace(tzOffsetRaw) != "" {
		minutes, err := strconv.Atoi(strings.TrimSpace(tzOffsetRaw))
		if err != nil || minutes < -channelUsageMaxTZOffsetMin || minutes > channelUsageMaxTZOffsetMin {
			return result, errors.New("invalid tz_offset")
		}
		result.OffsetSeconds = int64(minutes) * 60
	}

	if span/result.BucketSize+2 > channelUsageMaxBuckets {
		return result, errors.New("too many data points for this range; use day granularity or a shorter range")
	}
	return result, nil
}

// channelUsageBucketStart is the start of the bucket that contains ts.
func channelUsageBucketStart(ts int64, r channelUsageRange) int64 {
	shifted := ts + r.OffsetSeconds
	start := shifted - ((shifted%r.BucketSize)+r.BucketSize)%r.BucketSize
	return start - r.OffsetSeconds
}

// fillChannelUsageBuckets returns a continuous series covering the whole range: buckets
// without traffic are present with zero values so charts keep an even time axis.
func fillChannelUsageBuckets(rows []model.ChannelUsageBucket, r channelUsageRange) []model.ChannelUsageBucket {
	byBucket := make(map[int64]model.ChannelUsageBucket, len(rows))
	for _, row := range rows {
		byBucket[row.Bucket] = row
	}
	filled := make([]model.ChannelUsageBucket, 0, (r.End-r.Start)/r.BucketSize+2)
	for bucket := channelUsageBucketStart(r.Start, r); bucket <= r.End; bucket += r.BucketSize {
		row, ok := byBucket[bucket]
		if !ok {
			row = model.ChannelUsageBucket{Bucket: bucket}
		}
		filled = append(filled, row)
	}
	return filled
}

// GetChannelUsageTrend returns one channel's token consumption over time, for the trend
// dialog of the channel list. Query: start_timestamp, end_timestamp, granularity=hour|day,
// tz_offset (minutes east of UTC).
func GetChannelUsageTrend(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid channel id")
		return
	}
	if _, err := model.GetChannelById(id, false); err != nil {
		common.ApiError(c, err)
		return
	}
	r, err := resolveChannelUsageRange(
		c.Query("start_timestamp"),
		c.Query("end_timestamp"),
		c.Query("granularity"),
		c.Query("tz_offset"),
		time.Now().Unix(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rows, err := model.GetChannelUsageTrend(id, r.Start, r.End, r.BucketSize, r.OffsetSeconds)
	if err != nil {
		common.SysError(common.LogText("failed to get channel usage trend: %s", err.Error()))
		common.ApiErrorMsg(c, "failed to load the consumption trend")
		return
	}

	var totalTokens, totalQuota, totalCount int64
	for _, row := range rows {
		totalTokens += row.TokenUsed
		totalQuota += row.Quota
		totalCount += row.Count
	}
	common.ApiSuccess(c, gin.H{
		"channel_id":      id,
		"start_timestamp": r.Start,
		"end_timestamp":   r.End,
		"granularity":     r.Granularity,
		"buckets":         fillChannelUsageBuckets(rows, r),
		"total_tokens":    totalTokens,
		"total_quota":     totalQuota,
		"total_count":     totalCount,
	})
}

// attachChannelUsedTokens fills Channel.UsedTokens (cumulative token consumption) for a page
// of channels. A failure only costs the column: the list itself still renders.
func attachChannelUsedTokens(channels []*model.Channel) {
	if len(channels) == 0 {
		return
	}
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		if channel != nil {
			ids = append(ids, channel.Id)
		}
	}
	totals, err := model.GetChannelTokenTotals(ids)
	if err != nil {
		common.SysError(common.LogText("failed to load channel token totals: %s", err.Error()))
		return
	}
	for _, channel := range channels {
		if channel != nil {
			channel.UsedTokens = totals[channel.Id]
		}
	}
}
