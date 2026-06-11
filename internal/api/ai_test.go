package api

import (
	"testing"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/google/uuid"
)

func TestAIProviderDTORedactsSensitiveConfig(t *testing.T) {
	dto := aiProviderToDTO(db.AiProvider{
		ID:     uuid.New(),
		Name:   "openai-main",
		Type:   "openai",
		Config: []byte(`{"api_key":"sk-test","token":"secret-token","base_url":"https://api.openai.com"}`),
		Status: "active",
	})
	if dto.Config["api_key"] == "sk-test" || dto.Config["token"] == "secret-token" {
		t.Fatalf("sensitive config leaked: %+v", dto.Config)
	}
	if dto.Config["base_url"] != "https://api.openai.com" {
		t.Fatalf("non-secret config was not preserved: %+v", dto.Config)
	}
}
