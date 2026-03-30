# CLIProxyAPI Plus

English | [Chinese](README_CN.md)

This is the Plus version of [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), adding support for third-party providers on top of the mainline project.

All third-party provider support is maintained by community contributors; CLIProxyAPI does not provide technical support. Please contact the corresponding community maintainer if you need assistance.

The Plus release stays in lockstep with the mainline features.

## Per-Credential RPM Rate Limiting

CLIProxyAPI Plus supports per-credential RPM (Requests Per Minute) rate limiting. When a credential exceeds its RPM threshold, the scheduler automatically skips it and selects the next available credential. If all credentials are rate-limited, the proxy returns HTTP 429 with a `Retry-After` header.

### Quick Start

Add to your `config.yaml`:

```yaml
# Global default: max requests per minute per credential (0 = unlimited)
default-rpm: 60
```

### Per-Credential Override

Individual credentials can override the global default by setting the `rpm_limit` attribute:

```yaml
gemini-api-key:
  - api-key: "your-key-here"
    rpm_limit: "30"     # This credential: max 30 req/min
    priority: 1
```

### How It Works

- **Sliding window algorithm**: Tracks request timestamps within a 60-second window per credential
- **Scheduler integration**: RPM-exhausted credentials are automatically skipped during selection
- **Conductor double-check**: A secondary check before execution ensures concurrency safety
- **Automatic recovery**: Credentials become available again once requests slide out of the window
- **Zero config default**: `rpm_limit=0` or unset means unlimited (fully backward compatible)

### Configuration Reference

| Config | Location | Type | Default | Description |
|--------|----------|------|---------|-------------|
| `default-rpm` | `config.yaml` | int | `0` | Global default RPM limit for all credentials. `0` = unlimited |
| `rpm_limit` | credential attributes | string | — | Per-credential override. Takes priority over `default-rpm` |

### Management API

Query current RPM status for all credentials:

```bash
GET /v0/management/rpm-stats
```

Returns each credential's current request count, configured limit, and retry-after duration.

### Observability

When a credential hits its RPM limit, a warning log is emitted:

```
WARN RPM limit reached for credential  auth_id=xxx provider=gemini rpm_limit=60 current_rpm=60 retry_after_seconds=45
```

## Contributing

This project only accepts pull requests that relate to third-party provider support. Any pull requests unrelated to third-party provider support will be rejected.

If you need to submit any non-third-party provider changes, please open them against the [mainline](https://github.com/router-for-me/CLIProxyAPI) repository.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
