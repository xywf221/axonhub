# Permissions

## Overview
AxonHub is a single-tenant deployment: every authenticated user has full access to all features and data. Roles, projects, and per-user permission assignments are not part of the system, so there is no RBAC configuration to maintain.

## API Key Scopes
API keys are the only principals with restricted permissions. Each API key carries a list of scopes that determine which operations it may perform; requests authenticated with a key are limited to those scopes.

## Best Practices
- Rotate API keys regularly and revoke unused credentials from the admin console.
- Grant each API key only the scopes it needs for its workload.
- Use separate API keys for automation pipelines and CI/CD flows.

## Related Resources
- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)
- [Claude Code & Codex Integration](claude-code-integration.md)
