# 权限管理

## 概述
AxonHub 采用单租户部署：所有已登录用户都拥有全部功能与数据的访问权限。系统不再包含角色、项目以及按用户分配的权限，因此无需维护 RBAC 配置。

## API Key 作用域
API Key 是唯一受权限限制的主体。每个 API Key 携带一组作用域，用于限定其可执行的操作；使用 API Key 认证的请求只能访问其作用域允许的接口。

## 最佳实践
- 定期轮换 API Key，并在管理后台删除不再使用的凭证。
- 为每个 API Key 仅授予其工作负载所需的作用域。
- 为自动化流水线和 CI/CD 工作流使用独立的 API Key。

## 相关资源
- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)
- [Claude Code / Codex 集成指南](claude-code-integration.md)
