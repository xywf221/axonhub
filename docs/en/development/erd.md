# AxonHub Entity Relationship Diagram (ERD)

## Purpose

This document gives a compact view of AxonHub's data domains and core relationships. It intentionally omits field inventories, database types, defaults, and indexes. The Ent schemas under `internal/ent/schema/` are the source of truth for those implementation details.

## Domain Overview

| Domain | Main entities | Responsibility |
|---|---|---|
| Identity and access | User, APIKey, OIDCIdentity | Single-tenant authentication, API key ownership, and scopes |
| Provider configuration | Model, Channel, ChannelModelPrice, ChannelModelPriceVersion | Available models, provider connections, and pricing |
| Request lifecycle | Request, RequestExecution, UsageLog | Inbound requests, provider attempts, usage, and cost |
| Observability | Thread, Trace, ChannelProbe, ProviderQuotaStatus | Request grouping and provider health |
| Storage and configuration | DataStorage, System, Prompt, PromptProtectionRule | Payload storage and reusable system configuration |
| Supporting configuration | APIKeyProfileTemplate, ChannelOverrideTemplate | Reusable API key and channel override templates |

## Core Relationships

```mermaid
erDiagram
    User ||--o{ APIKey : owns
    User ||--o{ ChannelOverrideTemplate : owns
    User ||--o{ OIDCIdentity : authenticates

    Thread o|--o{ Trace : groups
    Trace o|--o{ Request : groups
    APIKey o|--o{ Request : authenticates
    Channel o|--o{ Request : routes
    DataStorage o|--o{ Request : stores

    Request ||--o{ RequestExecution : attempts
    Request ||--o{ UsageLog : accounts
    Channel ||--o{ RequestExecution : executes
    Channel o|--o{ UsageLog : attributes
    DataStorage o|--o{ RequestExecution : stores

    Channel ||--o{ ChannelModelPrice : prices
    ChannelModelPrice ||--o{ ChannelModelPriceVersion : versions
    Channel ||--o{ ChannelProbe : probes
    Channel o|--o| ProviderQuotaStatus : reports
```

## Relationship Notes

- AxonHub is single-tenant: users are not grouped into projects or assigned roles, and every authenticated user has full access.
- Every request records the authenticated `user_id`. API key, trace, channel, and data-storage associations may be absent depending on the request source and processing stage.
- `Request` represents the client-facing operation. `RequestExecution` represents an individual provider attempt, so one request can have multiple executions because of retries or fallback.
- `UsageLog` records accounting data for a request and may attribute usage to a channel.
- `Thread` groups traces, and a trace groups related requests.

## Request Lifecycle

```text
API key or admin request
  -> Request
  -> one or more RequestExecution records
  -> UsageLog
```

Request and response payloads may remain in the primary database or be stored through `DataStorage`. The entity relationships remain the same in either case.

## Data Boundaries

- Global resources such as channels, models, system settings, prompts, and data-storage definitions are shared across the deployment.
- API keys are the only principals with restricted permissions, through their scopes.
- Soft deletion is applied by the Ent schemas where historical identity or uniqueness must be preserved.

## Source of Truth

Use this document for domain orientation only. For exact fields, constraints, indexes, and generated database definitions, consult:

- `internal/ent/schema/` — authored entity definitions and relationships
- `internal/ent/migrate/schema.go` — generated migration schema
- `internal/server/biz/` — lifecycle and business invariants

## Related Resources

- [Transformation Flow Architecture](transformation-flow.md)
- [Permission Guide](../guides/permissions.md)
- [Tracing Guide](../guides/tracing.md)
