package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveChannelUsageRangeDefaultsToTheLast24HoursHourly(t *testing.T) {
	const now = int64(1_800_000_000)
	r, err := resolveChannelUsageRange("", "", "", "", now)
	require.NoError(t, err)
	assert.Equal(t, now-24*3600, r.Start)
	assert.Equal(t, now, r.End)
	assert.Equal(t, "hour", r.Granularity)
	assert.Equal(t, int64(3600), r.BucketSize)
	assert.Equal(t, int64(0), r.OffsetSeconds)
}

func TestResolveChannelUsageRangePicksGranularityFromTheSpan(t *testing.T) {
	const now = int64(1_800_000_000)
	three, err := resolveChannelUsageRange("1799740800", "1800000000", "", "", now) // 3 days
	require.NoError(t, err)
	assert.Equal(t, "hour", three.Granularity)

	thirty, err := resolveChannelUsageRange("1797408000", "1800000000", "", "", now) // 30 days
	require.NoError(t, err)
	assert.Equal(t, "day", thirty.Granularity)
	assert.Equal(t, int64(86400), thirty.BucketSize)

	forced, err := resolveChannelUsageRange("1797408000", "1800000000", "day", "", now)
	require.NoError(t, err)
	assert.Equal(t, "day", forced.Granularity)
}

func TestResolveChannelUsageRangeRejectsBadInput(t *testing.T) {
	const now = int64(1_800_000_000)
	cases := map[string][4]string{
		"start after end":        {"1800000000", "1700000000", "", ""},
		"start equals end":       {"1700000000", "1700000000", "", ""},
		"non numeric start":      {"yesterday", "", "", ""},
		"non numeric end":        {"", "now", "", ""},
		"negative start":         {"-5", "", "", ""},
		"range over 400 days":    {"1700000000", "1800000000", "day", ""},
		"unknown granularity":    {"", "", "minute", ""},
		"timezone out of range":  {"", "", "", "900"},
		"timezone not a number":  {"", "", "", "+8h"},
		"hourly over 41 days":    {"1790000000", "1800000000", "hour", ""},
	}
	for name, in := range cases {
		_, err := resolveChannelUsageRange(in[0], in[1], in[2], in[3], now)
		assert.Error(t, err, name)
	}
}

func TestResolveChannelUsageRangeAcceptsTheViewersTimezone(t *testing.T) {
	r, err := resolveChannelUsageRange("", "", "day", "480", 1_800_000_000)
	require.NoError(t, err)
	assert.Equal(t, int64(8*3600), r.OffsetSeconds)

	west, err := resolveChannelUsageRange("", "", "day", "-300", 1_800_000_000)
	require.NoError(t, err)
	assert.Equal(t, int64(-5*3600), west.OffsetSeconds)
}

func TestFillChannelUsageBucketsKeepsAContinuousAxis(t *testing.T) {
	const h = int64(3600)
	r := channelUsageRange{Start: 10*h + 600, End: 14*h + 5, BucketSize: h}
	rows := []model.ChannelUsageBucket{
		{Bucket: 11 * h, TokenUsed: 5, Quota: 1, Count: 1},
		{Bucket: 13 * h, TokenUsed: 9, Quota: 2, Count: 3},
	}
	got := fillChannelUsageBuckets(rows, r)
	require.Len(t, got, 5)
	assert.Equal(t, []int64{10 * h, 11 * h, 12 * h, 13 * h, 14 * h}, []int64{got[0].Bucket, got[1].Bucket, got[2].Bucket, got[3].Bucket, got[4].Bucket})
	assert.Equal(t, int64(5), got[1].TokenUsed)
	assert.Equal(t, int64(0), got[2].TokenUsed, "a gap hour is present with zeros")
	assert.Equal(t, int64(9), got[3].TokenUsed)
}

func TestFillChannelUsageBucketsAlignsTheFirstBucketToTheViewersDay(t *testing.T) {
	const h = int64(3600)
	const day = 24 * h
	r := channelUsageRange{Start: 5*day + 2*h, End: 7*day + 3*h, BucketSize: day, OffsetSeconds: 8 * h}
	got := fillChannelUsageBuckets(nil, r)
	require.NotEmpty(t, got)
	// every bucket starts at a local midnight: (bucket + offset) is a multiple of a day
	for _, b := range got {
		assert.Equal(t, int64(0), (b.Bucket+r.OffsetSeconds)%day)
	}
	assert.LessOrEqual(t, got[0].Bucket, r.Start)
	assert.Greater(t, got[0].Bucket+day, r.Start, "the first bucket contains the start of the range")
}

func TestMergeChannelUsageBreakdownCombinesBothSources(t *testing.T) {
	const h = int64(3600)
	stats := []model.ChannelUsageBucket{
		{Bucket: 1 * h, TokenUsed: 100, Quota: 5, Count: 2},
		{Bucket: 2 * h, TokenUsed: 40, Quota: 1, Count: 1},
	}
	logs := []model.ChannelUsageBucket{
		{Bucket: 1 * h, InputTokens: 60, OutputTokens: 10, CacheReadTokens: 30},
		{Bucket: 3 * h, InputTokens: 4}, // only in the logs
	}
	got := mergeChannelUsageBreakdown(stats, logs)
	require.Len(t, got, 3)
	assert.Equal(t, model.ChannelUsageBucket{Bucket: 1 * h, TokenUsed: 100, Quota: 5, Count: 2, InputTokens: 60, OutputTokens: 10, CacheReadTokens: 30}, got[0])
	assert.Equal(t, model.ChannelUsageBucket{Bucket: 2 * h, TokenUsed: 40, Quota: 1, Count: 1}, got[1], "no logs for that hour")
	assert.Equal(t, int64(4), got[2].InputTokens)

	assert.Equal(t, stats, mergeChannelUsageBreakdown(stats, nil), "without logs the statistics pass through")
}

