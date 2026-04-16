-- Migrate Antigravity model_mapping to add Opus 4.7 and retarget old Opus models
--
-- Background:
-- claude-opus-4-7-thinking is now the latest Opus model on Antigravity.
-- All older Opus variants (4.5, 4.6) should map to claude-opus-4-7-thinking.
-- Also adds new models added since last full mapping sync (gemini-3.1, etc.)
--
-- Strategy:
-- Full overwrite of model_mapping to match DefaultAntigravityModelMapping in constants.go

UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping}',
    '{
        "claude-opus-4-7-thinking": "claude-opus-4-7-thinking",
        "claude-opus-4-7": "claude-opus-4-7-thinking",
        "claude-opus-4-6-thinking": "claude-opus-4-7-thinking",
        "claude-opus-4-6": "claude-opus-4-7-thinking",
        "claude-opus-4-5-thinking": "claude-opus-4-7-thinking",
        "claude-opus-4-5-20251101": "claude-opus-4-7-thinking",
        "claude-sonnet-4-6": "claude-sonnet-4-6",
        "claude-sonnet-4-5": "claude-sonnet-4-5",
        "claude-sonnet-4-5-thinking": "claude-sonnet-4-5-thinking",
        "claude-sonnet-4-5-20250929": "claude-sonnet-4-5",
        "claude-haiku-4-5": "claude-sonnet-4-6",
        "claude-haiku-4-5-20251001": "claude-sonnet-4-6",
        "gemini-2.5-flash": "gemini-2.5-flash",
        "gemini-2.5-flash-image": "gemini-2.5-flash-image",
        "gemini-2.5-flash-image-preview": "gemini-2.5-flash-image",
        "gemini-2.5-flash-lite": "gemini-2.5-flash-lite",
        "gemini-2.5-flash-thinking": "gemini-2.5-flash-thinking",
        "gemini-2.5-pro": "gemini-2.5-pro",
        "gemini-3-flash": "gemini-3-flash",
        "gemini-3-pro-high": "gemini-3-pro-high",
        "gemini-3-pro-low": "gemini-3-pro-low",
        "gemini-3-flash-preview": "gemini-3-flash",
        "gemini-3-pro-preview": "gemini-3-pro-high",
        "gemini-3.1-pro-high": "gemini-3.1-pro-high",
        "gemini-3.1-pro-low": "gemini-3.1-pro-low",
        "gemini-3.1-pro-preview": "gemini-3.1-pro-high",
        "gemini-3.1-flash-image": "gemini-3.1-flash-image",
        "gemini-3.1-flash-image-preview": "gemini-3.1-flash-image",
        "gemini-3-pro-image": "gemini-3.1-flash-image",
        "gemini-3-pro-image-preview": "gemini-3.1-flash-image",
        "gpt-oss-120b-medium": "gpt-oss-120b-medium",
        "tab_flash_lite_preview": "tab_flash_lite_preview"
    }'::jsonb
)
WHERE platform = 'antigravity'
  AND deleted_at IS NULL
  AND credentials->'model_mapping' IS NOT NULL;
