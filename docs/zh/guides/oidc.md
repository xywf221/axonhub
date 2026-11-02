# OpenID Connect (OIDC) 集成指南

AxonHub 支持通过任何符合 OIDC 标准的身份提供商（IdP）进行身份验证，例如 Google、GitHub 或 Logto。

## 配置

在 `conf/config.yml` 文件的 `oidc` 部分配置您的 OIDC 提供商。

```yaml
# AxonHub 实例的公共 URL（对于构建重定向 URI 至关重要）
server:
  public_url: "https://axonhub.example.com"

oidc:
  providers:
    - id: "google"
      name: "google"
      display_name: "Google SSO"
      issuer_url: "https://accounts.google.com"
      client_id: "YOUR_GOOGLE_CLIENT_ID"
      client_secret: "YOUR_GOOGLE_CLIENT_SECRET"
      extra_scopes: ["openid", "profile", "email"]
      jit_enabled: true              # 首次登录时自动创建用户 (Just-In-Time)
      auto_link_by_email: true       # 通过已验证的邮箱链接到现有用户
      require_email_verified: true   # 仅在 IdP 已验证邮箱时才进行链接
      enable_pkce: true              # 推荐启用以增强安全性 (RFC 7636)
      sync_user_info: true           # 每次登录时同步名称和头像
      button_color: "#DB4437"        # UI 自定义：按钮颜色
      icon_url: "https://www.google.com/favicon.ico" # UI 自定义：图标 URL
```

### 提供商选项 (Provider Options)

| 选项 | 描述 |
|---|---|
| `id` | 提供商的唯一标识符（用于回调 URL）。 |
| `name` | 人类可读的名称。 |
| `display_name` | 登录按钮上显示的名称。 |
| `issuer_url` | OIDC 发行者 URL（发现端点）。 |
| `client_id` | OAuth2 客户端 ID。 |
| `client_secret` | OAuth2 客户端密钥。 |
| `extra_scopes` | 额外的 OAuth2 作用域。注意：如果提供此项，必须包含 `openid`。 |
| `redirect_url` | （可选）手动覆盖绝对重定向 URI。 |
| `jit_enabled` | 如果为 true，新用户将在首次登录时自动创建。 |
| `auto_link_by_email` | 如果为 true，将通过邮箱匹配现有用户。 |
| `require_email_verified` | 如果为 true，仅链接 IdP 标记为已验证的邮箱。 |
| `enable_pkce` | 启用 Proof Key for Code Exchange (RFC 7636)。强烈建议启用。 |
| `sync_user_info` | 从 IdP Claims 更新用户资料（名称、头像）。 |
| `button_color` | 登录页按钮的十六进制颜色代码。 |
| `icon_url` | 登录页显示的图标 URL 或本地路径（支持 base64）。 |
| `oidc_login_only` | 如果为 true，将强制该提供商的用户仅能通过 SSO 登录。 |

## 设置示例

### Logto (私有部署)

1. 在 Logto 中创建一个 **Traditional Web** 应用程序。
2. 设置 **Redirect URI**：
   - 如果您有**多个** OIDC 提供商：`<public_url>/oauth/oidc/callback/logto`
   - 如果这是**唯一**的提供商：`<public_url>/oauth/oidc/callback`
3. 设置 **Post sign-out URI** 为 `<public_url>/sign-in`。
4. 在 AxonHub 配置中添加以下内容：

```yaml
oidc:
  providers:
    - id: "logto"
      name: "logto"
      display_name: "Logto SSO"
      issuer_url: "https://your-logto-domain.com/oidc"
      client_id: "<App ID>"
      client_secret: "<App Secret>"
      enable_pkce: true
      jit_enabled: true
      button_color: "#5C5CFF"
```

## 安全注意事项

- **Public URL (重要)**: 在生产环境中，请务必设置 `server.public_url`。如果未设置，AxonHub 将回退到请求中的 `Host` 标头来构建回调地址，这在反向代理环境下可能导致 **Host 标头注入攻击**。
- **PKCE**: 强烈建议为所有提供商启用 `enable_pkce: true`。
- **仅限 SSO**: 如果您想强制执行 SSO 并阻止特定用户的密码登录，可以为提供商启用 `oidc_login_only: true`。**注意：开启此项后，前端登录页面将自动隐藏账号密码登录表单，仅显示 SSO 选项。** 您也可以在用户管理 UI 中将个别用户的密码设置为魔法占位符 `!OIDC_SSO_ONLY!` 来实现单用户锁定。
