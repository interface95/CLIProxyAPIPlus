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

# Global default: max concurrent in-flight requests per credential (0 = unlimited)
default-max-concurrent: 10
```

### Per-Credential Override

Individual credentials can have their own RPM limit by setting the `rpm_limit` attribute in the credential's auth JSON file (under the `auths/` directory):

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

Or via the Management API:

```bash
# Set RPM limit for a specific credential
PATCH /v0/management/auths/{auth_id}
{
  "attributes": {
    "rpm_limit": "30"
  }
}
```

When `rpm_limit` is set on a credential, it takes priority over the global `default-rpm`.

### How It Works

- **Sliding window algorithm**: Tracks request timestamps within a 60-second window per credential
- **Concurrency control**: Limits simultaneous in-flight requests per credential (acquire on start, release on finish)
- **Scheduler integration**: RPM-exhausted or concurrency-full credentials are automatically skipped during selection
- **Conductor double-check**: A secondary check before execution ensures concurrency safety
- **Automatic recovery**: Credentials become available again once requests slide out of the window
- **Zero config default**: `rpm_limit=0` or unset means unlimited (fully backward compatible)

### Configuration Reference

| Config | Location | Type | Default | Description |
|--------|----------|------|---------|-------------|
| `default-rpm` | `config.yaml` | int | `0` | Global default RPM limit for all credentials. `0` = unlimited |
| `rpm_limit` | credential attributes | string | — | Per-credential RPM override. Takes priority over `default-rpm` |
| `default-max-concurrent` | `config.yaml` | int | `0` | Global default max concurrent in-flight requests per credential. `0` = unlimited |
| `max_concurrent` | credential attributes | string | — | Per-credential concurrency override. Takes priority over `default-max-concurrent` |

### Management API

Query current rate limiting status:

```bash
GET /v0/management/rpm-stats            # RPM status per credential
GET /v0/management/concurrency-stats    # Concurrent in-flight requests per credential
```

### Observability

When a credential hits its RPM limit:

```
WARN RPM limit reached for credential  auth_id=xxx provider=gemini rpm_limit=60 current_rpm=60 retry_after_seconds=45
```

When a credential hits its concurrency limit:

```
WARN Concurrency limit reached for credential  auth_id=xxx provider=gemini max_concurrent=10 current=10
```

## Contributing

This project only accepts pull requests that relate to third-party provider support. Any pull requests unrelated to third-party provider support will be rejected.

If you need to submit any non-third-party provider changes, please open them against the [mainline](https://github.com/router-for-me/CLIProxyAPI) repository.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
