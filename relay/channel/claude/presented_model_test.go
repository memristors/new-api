package claude

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mappedInfo(format types.RelayFormat) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: "claude-opus-5.5",
		RelayFormat:     format,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-sonnet-5.5",
			IsModelMapped:     true,
		},
	}
}

const upstreamBody = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5.5","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":1}}`

func TestMappedModelClaudeNonStreamReturnsRequestedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := mappedInfo(types.RelayFormatClaude)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)

	require.Nil(t, HandleClaudeResponseData(c, info, &ClaudeResponseInfo{Usage: &dto.Usage{}}, nil, []byte(upstreamBody)))

	assert.JSONEq(t, `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5.5","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":1}}`, w.Body.String())
	// the log still shows what really happened
	assert.Equal(t, "claude-sonnet-5.5", info.UpstreamModelName)
	require.NotNil(t, info.ResponseModel)
	assert.Equal(t, "claude-sonnet-5.5", info.ResponseModel.ReturnedModel)
	assert.Equal(t, "claude-opus-5.5", info.ResponseModel.RequestedModel)
}

func TestMappedModelOpenAINonStreamReturnsRequestedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := mappedInfo(types.RelayFormatOpenAI)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	require.Nil(t, HandleClaudeResponseData(c, info, &ClaudeResponseInfo{Usage: &dto.Usage{}}, nil, []byte(upstreamBody)))

	assert.Contains(t, w.Body.String(), `"model":"claude-opus-5.5"`)
	assert.NotContains(t, w.Body.String(), "claude-sonnet-5.5")
	assert.Equal(t, "claude-sonnet-5.5", info.ResponseModel.ReturnedModel)
}

func TestMappedModelClaudeStreamRewritesMessageStartOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := mappedInfo(types.RelayFormatClaude)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{}}

	start := `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5.5","content":[],"usage":{"input_tokens":5,"output_tokens":0}}}`
	require.Nil(t, HandleStreamResponseData(c, info, claudeInfo, start))
	delta := `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`
	require.Nil(t, HandleStreamResponseData(c, info, claudeInfo, delta))

	out := w.Body.String()
	assert.Contains(t, out, `"model":"claude-opus-5.5"`)
	assert.NotContains(t, out, "claude-sonnet-5.5")
	assert.Contains(t, out, delta, "other events must pass through untouched")
	// billing / log keep the upstream model
	assert.Equal(t, "claude-sonnet-5.5", info.UpstreamModelName)
	assert.Equal(t, "claude-sonnet-5.5", info.ResponseModel.ReturnedModel)
}

func TestMappedModelOpenAIStreamAndFinalUsageUseRequestedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := mappedInfo(types.RelayFormatOpenAI)
	info.ShouldIncludeUsage = true
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	claudeInfo := &ClaudeResponseInfo{Usage: &dto.Usage{}, Model: presentedModelName(info)}

	start := `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5.5","content":[],"usage":{"input_tokens":5,"output_tokens":0}}}`
	require.Nil(t, HandleStreamResponseData(c, info, claudeInfo, start))
	delta := `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`
	require.Nil(t, HandleStreamResponseData(c, info, claudeInfo, delta))
	claudeInfo.Usage.CompletionTokens = 1
	claudeInfo.Done = true
	HandleStreamFinalResponse(c, info, claudeInfo)

	out := w.Body.String()
	assert.Contains(t, out, `"model":"claude-opus-5.5"`)
	assert.NotContains(t, out, "claude-sonnet-5.5")
}

func TestUnmappedModelIsLeftAlone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	info := mappedInfo(types.RelayFormatClaude)
	info.IsModelMapped = false
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)

	require.Nil(t, HandleClaudeResponseData(c, info, &ClaudeResponseInfo{Usage: &dto.Usage{}}, nil, []byte(upstreamBody)))

	assert.Equal(t, upstreamBody, w.Body.String())
}
