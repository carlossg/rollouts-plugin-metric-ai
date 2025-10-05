## rollouts-plugin-metric-ai

Standalone Argo Rollouts Metric Provider plugin written in Go. It:
- Collects stable/canary pod logs in the Rollout namespace
- Uses Gemini (Google Generative AI) to analyze logs and decide promote/fail
- On failure, uses GitHub API to open a PR with AI-proposed fixes (or a proposal file)

Configuration snippet in argo-rollouts-config ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argo-rollouts-config
data:
  metricProviderPlugins: |-
    - name: "argoproj-labs/metric-ai"
      location: "file://./rollouts-plugin-metric-ai/bin/metric-ai"
```

Use in an AnalysisTemplate:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AnalysisTemplate
metadata:
  name: canary-analysis
spec:
  metrics:
    - name: ai-analysis
      provider:
        plugin:
          argoproj-labs/metric-ai:
            model: gemini-2.0-flash-exp
            baseBranch: main
            githubUrl: https://github.com/carlossg/rollouts-demo
```

Environment variables:
- GOOGLE_API_KEY: required for Gemini
- GITHUB_TOKEN
- AUTO_PR_ENABLED=true|false
- LOG_LEVEL: log level for the plugin (panic, fatal, error, warn, info, debug, trace). Default: info

## Building

Build locally:

```bash
# Build Go binary
make build

# Build Docker image
make docker-build

# Build multi-platform Docker image and push to registry
make docker-buildx
```

## CI/CD

GitHub Actions automatically builds and publishes Docker images to GitHub Container Registry (ghcr.io) on pushes to main:
- Images are tagged with the commit SHA
- Multi-platform builds (amd64, arm64)
- Available at: `ghcr.io/carlossg/argo-rollouts/rollouts-plugin-metric-ai:<commit-sha>`

## Examples

See `examples/` directory for:
- Analysis template configuration
- Argo Rollouts ConfigMap setup

See `config/rollouts-examples/` for complete deployment examples including:
- Rollout with AI analysis
- Canary services and ingress
- Traffic generator for testing

## Debugging and Logging

The plugin supports configurable logging levels to help with debugging and monitoring. You can control the log level using the `LOG_LEVEL` environment variable.

### Available Log Levels

- `panic`: Only panic level messages
- `fatal`: Fatal and panic level messages  
- `error`: Error, fatal, and panic level messages
- `warn`: Warning, error, fatal, and panic level messages
- `info`: Info, warning, error, fatal, and panic level messages (default)
- `debug`: Debug, info, warning, error, fatal, and panic level messages
- `trace`: All log messages including trace level

### Setting Log Level

#### Via Environment Variable
```bash
export LOG_LEVEL=debug
```

#### Via Kubernetes Deployment
Update the Argo Rollouts deployment to include the environment variable:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argo-rollouts
spec:
  template:
    spec:
      containers:
      - name: argo-rollouts
        env:
        - name: LOG_LEVEL
          value: "debug"
```

#### Via Kustomize (recommended)
The deployment configuration in `config/argo-rollouts/kustomization.yaml` already includes the `LOG_LEVEL` environment variable set to `debug` by default.

### Viewing Plugin Logs

To view the plugin logs with debug information:

```bash
# View all logs
kubectl logs -f -n argo-rollouts deployment/argo-rollouts

# Filter for plugin-specific logs
kubectl logs -n argo-rollouts deployment/argo-rollouts | grep -E "metric-ai|AI metric|plugin"

# View logs with timestamps
kubectl logs -n argo-rollouts deployment/argo-rollouts --timestamps=true
```

### Debug Information

When `LOG_LEVEL=debug` or `LOG_LEVEL=trace`, the plugin will log:
- Detailed configuration parsing
- Pod log fetching operations
- AI analysis requests and responses
- GitHub API interactions
- Rate limiting and retry attempts
- Performance metrics

# Testing

```bash
make test
```

This will create a Kind cluster and run the e2e tests.

You can also run only the e2e tests locally with:

```bash
make test-e2e
```
