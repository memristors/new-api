package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedQuotaData(t *testing.T, rows []QuotaData) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM quota_data").Error)
	t.Cleanup(func() { DB.Exec("DELETE FROM quota_data") })
	for i := range rows {
		require.NoError(t, DB.Table("quota_data").Create(&rows[i]).Error)
	}
}

func TestGetChannelTokenTotalsSumsPerChannelAndSkipsUnusedOnes(t *testing.T) {
	seedQuotaData(t, []QuotaData{
		{UserID: 1, Username: "u", ModelName: "m", CreatedAt: 3600, ChannelID: 7, TokenUsed: 100, Count: 1},
		{UserID: 1, Username: "u", ModelName: "m", CreatedAt: 7200, ChannelID: 7, TokenUsed: 250, Count: 2},
		{UserID: 2, Username: "v", ModelName: "n", CreatedAt: 7200, ChannelID: 8, TokenUsed: 40, Count: 1},
		{UserID: 2, Username: "v", ModelName: "n", CreatedAt: 7200, ChannelID: 9, TokenUsed: 999, Count: 1},
	})

	totals, err := GetChannelTokenTotals([]int{7, 8, 10})
	require.NoError(t, err)
	assert.Equal(t, map[int]int64{7: 350, 8: 40}, totals, "channel 9 was not asked for, channel 10 has no usage")

	empty, err := GetChannelTokenTotals(nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestGetChannelUsageTrendBucketsByHourOnlyForTheChannelAndRange(t *testing.T) {
	const h = int64(3600)
	seedQuotaData(t, []QuotaData{
		{UserID: 1, Username: "a", ModelName: "m1", CreatedAt: 10 * h, ChannelID: 5, TokenUsed: 100, Quota: 10, Count: 1},
		{UserID: 2, Username: "b", ModelName: "m2", CreatedAt: 10 * h, ChannelID: 5, TokenUsed: 50, Quota: 5, Count: 2}, // same hour, other user and model
		{UserID: 1, Username: "a", ModelName: "m1", CreatedAt: 12 * h, ChannelID: 5, TokenUsed: 70, Quota: 7, Count: 1},
		{UserID: 1, Username: "a", ModelName: "m1", CreatedAt: 11 * h, ChannelID: 6, TokenUsed: 999, Quota: 99, Count: 9}, // other channel
		{UserID: 1, Username: "a", ModelName: "m1", CreatedAt: 40 * h, ChannelID: 5, TokenUsed: 888, Quota: 88, Count: 8}, // outside the range
	})

	rows, err := GetChannelUsageTrend(5, 9*h, 20*h, h, 0)
	require.NoError(t, err)
	assert.Equal(t, []ChannelUsageBucket{
		{Bucket: 10 * h, TokenUsed: 150, Quota: 15, Count: 3},
		{Bucket: 12 * h, TokenUsed: 70, Quota: 7, Count: 1},
	}, rows)
}

func TestGetChannelUsageTrendAlignsDayBucketsToTheViewersMidnight(t *testing.T) {
	const h = int64(3600)
	const day = 24 * h
	// Viewer at UTC+8: local midnight is 16:00 UTC of the previous day.
	offset := 8 * h
	localMidnight := 5*day - offset // local day starting at 2026-ish "day 5" in the viewer's calendar
	seedQuotaData(t, []QuotaData{
		{UserID: 1, Username: "a", ModelName: "m", CreatedAt: localMidnight + 1*h, ChannelID: 3, TokenUsed: 10, Count: 1},   // first hour of the local day
		{UserID: 1, Username: "a", ModelName: "m", CreatedAt: localMidnight + 23*h, ChannelID: 3, TokenUsed: 20, Count: 1},  // last hour of the same local day
		{UserID: 1, Username: "a", ModelName: "m", CreatedAt: localMidnight + 24*h, ChannelID: 3, TokenUsed: 40, Count: 1},  // next local day
		{UserID: 1, Username: "a", ModelName: "m", CreatedAt: localMidnight - 1*h, ChannelID: 3, TokenUsed: 5, Count: 1},    // previous local day
	})

	rows, err := GetChannelUsageTrend(3, localMidnight-2*day, localMidnight+3*day, day, offset)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, ChannelUsageBucket{Bucket: localMidnight - day, TokenUsed: 5, Count: 1}, rows[0])
	assert.Equal(t, ChannelUsageBucket{Bucket: localMidnight, TokenUsed: 30, Count: 2}, rows[1], "the two hours of the same local day merge")
	assert.Equal(t, ChannelUsageBucket{Bucket: localMidnight + day, TokenUsed: 40, Count: 1}, rows[2])

	utc, err := GetChannelUsageTrend(3, localMidnight-2*day, localMidnight+3*day, day, 0)
	require.NoError(t, err)
	assert.NotEqual(t, rows, utc, "UTC day boundaries split the same traffic differently")
}
