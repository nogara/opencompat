package api

import "time"

// SupportedModels is the list of models available through the API.
var SupportedModels = []Model{
	{ID: "gpt-5.2-codex", Object: "model", Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "openai"},
	{ID: "gpt-5.1-codex-max", Object: "model", Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "openai"},
	{ID: "gpt-5.1-codex", Object: "model", Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "openai"},
	{ID: "gpt-5-codex", Object: "model", Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "openai"},
	{ID: "gpt-5.2", Object: "model", Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "openai"},
	{ID: "gpt-5.1", Object: "model", Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "openai"},
	{ID: "gpt-5", Object: "model", Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "openai"},
	{ID: "gpt-5.1-codex-mini", Object: "model", Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "openai"},
}

// modelSet is a set of supported model IDs for fast lookup.
var modelSet = make(map[string]bool)

func init() {
	for _, m := range SupportedModels {
		modelSet[m.ID] = true
	}
}

// IsModelSupported returns true if the model ID is supported.
func IsModelSupported(modelID string) bool {
	return modelSet[modelID]
}

// GetModelsResponse returns the models list response.
func GetModelsResponse() ModelsResponse {
	return ModelsResponse{
		Object: "list",
		Data:   SupportedModels,
	}
}
