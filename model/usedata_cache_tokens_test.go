package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuotaDataCacheTokens(t *testing.T) {
	anthropic := func(read, write any) *LogOther {
		other := NewLogOther()
		other.SetPublic("usage_semantic", "anthropic")
		if read != nil {
			other.SetPublic("cache_tokens", read)
		}
		if write != nil {
			other.SetPublic("cache_creation_tokens", write)
		}
		return other
	}

	assert.Equal(t, 0, quotaDataCacheTokens(nil), "nil log metadata")
	assert.Equal(t, 0, quotaDataCacheTokens(NewLogOther()), "no usage semantic")

	assert.Equal(t, 741027+1810, quotaDataCacheTokens(anthropic(741027, 1810)), "Anthropic usage: cache read + cache write")
	assert.Equal(t, 313590, quotaDataCacheTokens(anthropic(313590, nil)), "only cache read")
	assert.Equal(t, 2009, quotaDataCacheTokens(anthropic(nil, 2009)), "only cache write")
	assert.Equal(t, 15, quotaDataCacheTokens(anthropic(float64(10), int64(5))), "numbers decoded from JSON or widened")
	assert.Equal(t, 0, quotaDataCacheTokens(anthropic(-5, "x")), "negative or non-numeric values are ignored")

	// OpenAI-style usage already counts cached tokens inside prompt_tokens: never add them again.
	openai := NewLogOther()
	openai.SetPublic("cache_tokens", 5000)
	assert.Equal(t, 0, quotaDataCacheTokens(openai))
	openai.SetPublic("usage_semantic", "openai")
	assert.Equal(t, 0, quotaDataCacheTokens(openai))
}
