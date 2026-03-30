# CLIProxyAPI Plus

[English](README.md) | 中文 | [日本語](README_JA.md)

这是 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 的 Plus 版本，在原有基础上增加了第三方供应商的支持。

所有的第三方供应商支持都由第三方社区维护者提供，CLIProxyAPI 不提供技术支持。如需取得支持，请与对应的社区维护者联系。

## 凭证级 RPM 限流

CLIProxyAPI Plus 支持按凭证的 RPM（每分钟请求数）限流。当某个凭证的 RPM 达到阈值时，调度器自动跳过该凭证选择下一个可用凭证。当所有凭证都被限流时，代理返回 HTTP 429 并附带 `Retry-After` 头。

### 快速开始

在 `config.yaml` 中添加：

```yaml
# 全局默认：每个凭证每分钟最大请求数（0 = 不限制）
default-rpm: 60

# 全局默认：每个凭证最大同时在飞请求数（0 = 不限制）
default-max-concurrent: 10
```

### 单凭证覆盖

单个凭证可以通过在 auth JSON 文件（`auths/` 目录下）中设置 `rpm_limit` 属性来覆盖全局默认值：

```json
{
  "id": "gemini:apikey:xxxx",
  "provider": "gemini",
  "attributes": {
    "api_key": "AIzaSy...",
    "rpm_limit": "30"
  }
}
```

或通过管理 API 设置：

```bash
# 为指定凭证设置 RPM 限制
PATCH /v0/management/auths/{auth_id}
{
  "attributes": {
    "rpm_limit": "30"
  }
}
```

当凭证设置了 `rpm_limit` 时，优先于全局 `default-rpm`。

### 工作原理

- **滑动窗口算法**：在 60 秒窗口内按凭证跟踪请求时间戳
- **并发控制**：限制每个凭证同一时刻的在飞请求数（请求开始获取槽位，完成后释放）
- **调度器集成**：RPM 耗尽或并发已满的凭证在选择时被自动跳过
- **执行器双重检查**：执行前的二次检查确保并发安全
- **自动恢复**：请求滑出窗口后凭证自动恢复可用
- **零配置默认**：`rpm_limit=0` 或未设置表示不限制（完全向后兼容）

### 配置参考

| 配置项 | 位置 | 类型 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `default-rpm` | `config.yaml` | int | `0` | 所有凭证的全局默认 RPM 限制。`0` = 不限制 |
| `rpm_limit` | 凭证 attributes | string | — | 单凭证 RPM 覆盖，优先于 `default-rpm` |
| `default-max-concurrent` | `config.yaml` | int | `0` | 所有凭证的全局默认最大并发数。`0` = 不限制 |
| `max_concurrent` | 凭证 attributes | string | — | 单凭证并发覆盖，优先于 `default-max-concurrent` |

### 管理 API

查询限流状态：

```bash
GET /v0/management/rpm-stats            # 每个凭证的 RPM 状态
GET /v0/management/concurrency-stats    # 每个凭证的并发在飞请求数
```

### 可观测性

当凭证触发 RPM 限流时：

```
WARN RPM limit reached for credential  auth_id=xxx provider=gemini rpm_limit=60 current_rpm=60 retry_after_seconds=45
```

当凭证触发并发限制时：

```
WARN Concurrency limit reached for credential  auth_id=xxx provider=gemini max_concurrent=10 current=10
```

## 贡献

该项目仅接受第三方供应商支持的 Pull Request。任何非第三方供应商支持的 Pull Request 都将被拒绝。

如果需要提交任何非第三方供应商支持的 Pull Request，请提交到[主线](https://github.com/router-for-me/CLIProxyAPI)版本。

## 许可证

此项目根据 MIT 许可证授权 - 有关详细信息，请参阅 [LICENSE](LICENSE) 文件。
