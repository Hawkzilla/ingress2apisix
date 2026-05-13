# ingress2apisix

> Language: **English** | [中文](README.zh-CN.md)

`ingress2apisix` converts nginx Ingress resources into APISIX Ingress Controller compatible resources. It supports file conversion, cluster apply, Helm chart scanning/migration, and a browser-based Web UI.

## Usage Preview

![Usage Preview](usage.png)

## Features

- Converts `nginx.ingress.kubernetes.io/*` and `ingress.kubernetes.io/*` annotations into APISIX-native annotations where possible.
- Rewrites `ingressClassName` from `nginx` to `apisix` and removes `kubernetes.io/ingress.class: nginx`.
- Generates `ApisixPluginConfig` resources for features that require APISIX plugins.
- Generates `BackendTrafficPolicy` for `affinity: cookie` session affinity.
- Ships custom Lua plugins for session cookie hashing, cookie flag rewriting, cookie path rewriting, and multi-region IDP proxy behavior.
- Supports single Ingress YAML, `IngressList`, Kubernetes `List`, and multi-document YAML.
- Can read Ingress resources from a Kubernetes cluster and apply converted resources back to the cluster.

Supported nginx annotation prefixes:

- `nginx.ingress.kubernetes.io/*`
- `ingress.kubernetes.io/*`

When both prefixes are present, `nginx.ingress.kubernetes.io/*` takes precedence.

## Design Principle

Use APISIX-native annotations whenever possible. Generate `ApisixPluginConfig` or other APISIX CRDs only when native annotations cannot express the original nginx behavior.

## Annotation Mapping

### Native APISIX Annotations

| nginx annotation suffix | APISIX output | Notes |
|---|---|---|
| `ssl-redirect: "true"` | `k8s.apisix.apache.org/http-to-https: "true"` | HTTP to HTTPS redirect |
| `force-ssl-redirect` | `k8s.apisix.apache.org/http-to-https: "true"` | Force HTTPS when present |
| `proxy-redirect-from: "http://"` + `proxy-redirect-to: "https://"` | `k8s.apisix.apache.org/http-to-https: "true"` | SSL redirect case |
| `rewrite-target` | `rewrite-target` or `rewrite-target-regex` | Path rewrite |
| single rewrite in `configuration-snippet` | `rewrite-target-regex` + `rewrite-target-regex-template` | Uses native annotations |
| `proxy-connect-timeout` | `upstream-connect-timeout` | Adds `s` when the value has no unit |
| `proxy-send-timeout` | `upstream-send-timeout` | Upstream send timeout |
| `proxy-read-timeout` | `upstream-read-timeout` | Upstream read timeout |
| `enable-cors` | `enable-cors` | CORS switch |
| `cors-allow-origin` | `cors-allow-origin` | CORS origins |
| `cors-allow-methods` | `cors-allow-methods` | CORS methods |
| `cors-allow-headers` | `cors-allow-headers` | CORS headers |
| `backend-protocol` | `upstream-scheme` | HTTP, HTTPS, GRPC, GRPCS |
| `whitelist-source-range` | `allowlist-source-range` | IP allowlist |
| `auth-url` | `auth-uri` | External auth endpoint |
| `auth-response-headers` | `auth-upstream-headers` | Headers forwarded from auth response |
| `auth-type: basic` | `auth-type: basicAuth` | HTTP Basic auth |

### PluginConfig and CRDs

| nginx annotation or scenario | Generated resource | Notes |
|---|---|---|
| `limit-rps` | `ApisixPluginConfig` + `limit-req` | Requests per second |
| `limit-rpm` | `ApisixPluginConfig` + `limit-req` | Requests per minute |
| `limit-connections` | `ApisixPluginConfig` + `limit-conn` | Connection limiting |
| multiple rewrites in `configuration-snippet` | `ApisixPluginConfig` + `proxy-rewrite` | Keeps rewrite order |
| `rewrite-target` plus snippet rewrite | `ApisixPluginConfig` + `proxy-rewrite` | Merges both rewrite sources |
| `proxy_cookie_flags` in `configuration-snippet` | custom `proxy-cookie-flags` plugin | Set-Cookie flag handling |
| `session-cookie-hash` | custom `session-cookie-hash` plugin | Generates/uses session cookie hash |
| `affinity: cookie` | `BackendTrafficPolicy` | Cookie-based session affinity |

## Cookie Affinity

Input:

```yaml
metadata:
  annotations:
    nginx.ingress.kubernetes.io/affinity: "cookie"
    nginx.ingress.kubernetes.io/session-cookie-name: "route"
    nginx.ingress.kubernetes.io/session-cookie-hash: "sha1"
```

Output includes:

- A `BackendTrafficPolicy` using `loadbalancer.hashOn: cookie` and `key: route`.
- An `ApisixPluginConfig` using the custom `session-cookie-hash` plugin.
- A `k8s.apisix.apache.org/plugin-config-name` annotation on the converted Ingress.

If `session-cookie-hash` is missing but `affinity: cookie` exists, the converter defaults to `sha1` and emits a warning. If `session-cookie-name` is missing, the converter defaults to `INGRESSCOOKIE` and emits a warning.

Example generated `BackendTrafficPolicy`:

```yaml
apiVersion: apisix.apache.org/v1alpha1
kind: BackendTrafficPolicy
metadata:
  name: app-cookie-affinity
spec:
  targetRefs:
    - group: ""
      kind: Service
      name: app-svc
  loadbalancer:
    type: chash
    hashOn: cookie
    key: route
```

## Combined Rewrite Handling

When `rewrite-target` and `configuration-snippet` both define rewrites, the converter emits a single `proxy-rewrite` plugin instead of mixing native rewrite annotations and plugin behavior.

Input:

```yaml
metadata:
  annotations:
    ingress.kubernetes.io/rewrite-target: /
    ingress.kubernetes.io/configuration-snippet: |
      rewrite ^/cinder_dashboard_api/(.*) /$1 break;
spec:
  rules:
    - http:
        paths:
          - path: /cinder_dashboard_api
            pathType: ImplementationSpecific
            backend:
              service:
                name: cinder-golem
                port:
                  name: golem
```

Generated plugin:

```yaml
plugins:
  - name: proxy-rewrite
    enable: true
    config:
      regex_uri:
        - (?i)/cinder_dashboard_api
        - /
        - ^/cinder_dashboard_api/(.*)
        - /$1
```

## Custom Lua Plugins

Custom plugins are stored in `plugins/`:

- `session-cookie-hash.lua`
- `proxy-cookie-flags.lua`
- `proxy-cookie-path.lua`
- `multi-region-idp-proxy.lua`

Deploy a plugin to APISIX:

```bash
cp plugins/session-cookie-hash.lua /usr/local/apisix/apisix/plugins/
```

Enable it in APISIX `config.yaml`:

```yaml
plugins:
  - session-cookie-hash
```

## Installation

```bash
make build
# or
go install ./cmd/ingress2apisix
```

## Usage

### File Mode

```bash
ingress2apisix -f ingress.yaml
ingress2apisix -f ingress.yaml -o apisix-ready.yaml
```

Common options:

```bash
ingress2apisix -f ingress.yaml -o output.yaml \
  --default-namespace production \
  --ingress-class my-apisix \
  --ssl-redirect=false
```

### Cluster Mode

```bash
ingress2apisix --apply --dry-run
ingress2apisix --apply --namespace production
ingress2apisix --apply --kubeconfig ~/.kube/config --context my-cluster
```

Cluster mode:

1. Reads Ingress resources from the cluster.
2. Converts nginx annotations to APISIX-compatible configuration.
3. Patches the original Ingress annotations and `ingressClassName`.
4. Creates or updates generated `ApisixPluginConfig` resources.
5. Creates or updates generated `BackendTrafficPolicy` resources.

### Check Mode

Scan Helm charts or YAML directories and report migration status:

```bash
ingress2apisix --check ./charts
ingress2apisix --check ./charts --verbose
ingress2apisix --check ./charts --check-output report.md
```

### Migrate Mode

Modify Helm chart templates in place:

```bash
ingress2apisix --migrate ./charts
ingress2apisix --migrate ./charts --migrate-dry-run
ingress2apisix --migrate ./charts --migrate-backup
```

### Web UI

```bash
ingress2apisix --web
ingress2apisix --web --web-addr 0.0.0.0:3000
```

Open `http://localhost:8080`.

The Web UI supports:

- Convert: paste or upload nginx Ingress YAML and view converted APISIX resources.
- Check: inspect annotation migration status.
- Migrate: preview Helm chart migration output.
- Docs, feedback, and announcement panels.

## CLI Options

| Option | Default | Description |
|---|---|---|
| `-f` | required in file mode | Input Ingress YAML file |
| `-o` | stdout | Output file |
| `--apply` | `false` | Enable cluster apply mode |
| `--dry-run` | `false` | Preview only in cluster mode |
| `--kubeconfig` | `~/.kube/config` | kubeconfig path |
| `--context` | current context | Kubernetes context |
| `--namespace` | all namespaces | Namespace filter |
| `--default-namespace` | `default` | Namespace used when input has none |
| `--api-version` | `apisix.apache.org/v2` | APISIX CRD API version |
| `--ingress-class` | `apisix` | Target Ingress class |
| `--ssl-redirect` | `true` | Add HTTPS redirect for TLS Ingresses |
| `--check` | - | Scan chart/YAML directory |
| `--check-output` | stdout | Markdown report output path |
| `--verbose` | `false` | Print detailed check results |
| `--migrate` | - | Migrate Helm chart directory in place |
| `--migrate-dry-run` | `false` | Preview migration changes |
| `--migrate-backup` | `false` | Create `.bak` files before modification |
| `--web` | `false` | Start Web UI |
| `--web-addr` | `localhost:8080` | Web UI listen address |

## Project Layout

```text
ingress2apisix/
├── cmd/ingress2apisix/main.go
├── pkg/
│   ├── apisix/
│   ├── charts/
│   ├── converter/
│   ├── ingress/
│   ├── k8s/
│   └── web/
├── plugins/
│   ├── multi-region-idp-proxy.lua
│   ├── proxy-cookie-flags.lua
│   ├── proxy-cookie-path.lua
│   └── session-cookie-hash.lua
├── examples/
├── README.md
└── README.zh-CN.md
```

## Testing

```bash
go test ./...
```

## License

MIT
