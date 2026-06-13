-- +goose Up
-- LiteLLM integration: OPORD is the governance control plane, LiteLLM the data
-- plane. An approved AI access request mints a scoped LiteLLM VIRTUAL KEY (budget
-- + model allow-list + expiry) instead of handing out the org's real provider
-- key; LiteLLM enforces at runtime and reports spend back.

alter table ai_providers drop constraint if exists ai_providers_type_check;
alter table ai_providers add constraint ai_providers_type_check
    check (type in ('mock_ai', 'openai', 'anthropic', 'gemini', 'github_copilot', 'cursor', 'litellm'));

-- +goose Down
alter table ai_providers drop constraint if exists ai_providers_type_check;
alter table ai_providers add constraint ai_providers_type_check
    check (type in ('mock_ai', 'openai', 'anthropic', 'gemini', 'github_copilot', 'cursor'));
