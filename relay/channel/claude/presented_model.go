package claude

import (
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/tidwall/sjson"
)

// presentedModel is the model name the client should see in the response.
//
// When the channel maps the requested model to a different upstream model
// (channel model_mapping), the upstream answers with its own name. Returning
// that name makes a client that asked for "claude-opus-5.5" see
// "claude-sonnet-5.5", which breaks clients and SDKs that verify or display the
// model they requested. So for mapped requests the requested name is returned.
// Billing, routing and the usage log keep using the upstream model, and the
// log's response_model diagnostics still record what the upstream declared:
// ObserveResponseModel must be called with the upstream value before this.
func presentedModel(info *relaycommon.RelayInfo, upstreamModel string) string {
	if info != nil && info.HasChannelMeta() && info.IsModelMapped && info.OriginModelName != "" {
		return info.OriginModelName
	}
	return upstreamModel
}

// presentedModelName is presentedModel for places that only have the channel's
// upstream model name at hand (stream state, final usage chunk).
func presentedModelName(info *relaycommon.RelayInfo) string {
	if info == nil {
		return ""
	}
	return presentedModel(info, info.UpstreamModelName)
}

// rewriteModelJSON sets the JSON field at path to model. It returns the input
// unchanged when model is empty or the edit fails, so a response is never lost
// because of cosmetic rewriting.
func rewriteModelJSON(data []byte, path string, model string) []byte {
	if model == "" {
		return data
	}
	patched, err := sjson.SetBytes(data, path, model)
	if err != nil {
		return data
	}
	return patched
}
