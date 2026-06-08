package converter

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ingress2apisix/pkg/apisix"
	"github.com/ingress2apisix/pkg/ingress"
)

// Converter transforms nginx ingress-nginx Ingress resources into
// APISIX-compatible Ingress resources (k8s.apisix.apache.org annotations)
// and ApisixPluginConfig CRDs.
type Converter struct {
	opts apisix.ConversionOptions
}

// New creates a Converter with the given options.
func New(opts apisix.ConversionOptions) *Converter {
	return &Converter{opts: opts}
}

// Convert transforms a single Ingress into converted Ingress + ApisixPluginConfig resources.
func (c *Converter) Convert(ing ingress.Ingress) apisix.ConversionResult {
	result := apisix.ConversionResult{}

	ns := ing.Metadata.Namespace
	if ns == "" {
		ns = c.opts.DefaultNamespace
	}

	// Build converted Ingress (also collects warnings into &result)
	out := c.buildConvertedIngress(ing, ns, &result)

	// Build ApisixPluginConfig for complex plugin configs
	if pc, warns := c.buildPluginConfig(ing, ns); len(warns) > 0 || pc != nil {
		result.Warnings = append(result.Warnings, warns...)
		if pc != nil {
			result.PluginConfigs = append(result.PluginConfigs, *pc)

			// Link PluginConfig to Ingress via annotation (critical for APISIX Ingress Controller)
			if out.Metadata.Annotations == nil {
				out.Metadata.Annotations = make(map[string]string)
			}
			out.Metadata.Annotations["k8s.apisix.apache.org/plugin-config-name"] = pc.Metadata.Name
		}
	}

	if btps, warns := c.buildBackendTrafficPolicies(ing, ns); len(warns) > 0 || len(btps) > 0 {
		result.Warnings = append(result.Warnings, warns...)
		result.BackendTrafficPolicies = append(result.BackendTrafficPolicies, btps...)
	}

	// Build ApisixUpstream for health checks and other upstream config
	if au, auWarns := c.buildApisixUpstream(ing, ns); au != nil || len(auWarns) > 0 {
		result.Warnings = append(result.Warnings, auWarns...)
		if au != nil {
			result.ApisixUpstreams = append(result.ApisixUpstreams, *au)
		}
	}

	// Build ApisixTls for TLS configuration from Ingress spec.tls
	if tls := c.buildApisixTls(ing, ns); len(tls) > 0 {
		result.ApisixTls = append(result.ApisixTls, tls...)
	}

	result.Ingresses = append(result.Ingresses, out)

	return result
}

// ConvertList converts multiple Ingress resources, preserving the input format.
func (c *Converter) ConvertList(input ParsedInput) apisix.ConversionResult {
	result := apisix.ConversionResult{InputFormat: input.Format}
	for _, ing := range input.Ingresses {
		r := c.Convert(ing)
		result.Ingresses = append(result.Ingresses, r.Ingresses...)
		result.PluginConfigs = append(result.PluginConfigs, r.PluginConfigs...)
		result.BackendTrafficPolicies = append(result.BackendTrafficPolicies, r.BackendTrafficPolicies...)
		result.ApisixUpstreams = append(result.ApisixUpstreams, r.ApisixUpstreams...)
		result.ApisixTls = append(result.ApisixTls, r.ApisixTls...)
		result.Errors = append(result.Errors, r.Errors...)
		result.Warnings = append(result.Warnings, r.Warnings...)
	}
	return result
}

// Annotation prefix constants.
const (
	prefixNginx = "nginx.ingress.kubernetes.io/"
	prefixPlain = "ingress.kubernetes.io/"
)

// getAnnotation looks up an annotation by suffix, trying the nginx prefix first,
// then the plain ingress prefix. Returns the value and true if found.
func getAnnotation(annotations map[string]string, suffix string) (string, bool) {
	if v, ok := annotations[prefixNginx+suffix]; ok && v != "" {
		return v, true
	}
	if v, ok := annotations[prefixPlain+suffix]; ok && v != "" {
		return v, true
	}
	return "", false
}

// hasAnnotation returns true if the annotation exists under either prefix
// (regardless of value, for bool-style annotations like force-ssl-redirect).
func hasAnnotation(annotations map[string]string, suffix string) bool {
	if _, ok := annotations[prefixNginx+suffix]; ok {
		return true
	}
	_, ok := annotations[prefixPlain+suffix]
	return ok
}

// isNginxAnnotation returns true if the key uses either known nginx/ingress prefix.
func isNginxAnnotation(key string) bool {
	return strings.HasPrefix(key, prefixNginx) || strings.HasPrefix(key, prefixPlain)
}

// annotationSuffix extracts the suffix after the prefix.
func annotationSuffix(key string) string {
	if strings.HasPrefix(key, prefixNginx) {
		return key[len(prefixNginx):]
	}
	if strings.HasPrefix(key, prefixPlain) {
		return key[len(prefixPlain):]
	}
	return ""
}

// regexCapturePattern matches $1, $2, ... in rewrite targets.
var regexCapturePattern = regexp.MustCompile(`\$\d+`)

var rewriteDirectivePattern = regexp.MustCompile(`rewrite\s+(\S+)\s+(\S+)\s+(break|last|redirect|permanent)`)

// pathHasRegex returns true if the path looks like a regex.
var pathRegexPattern = regexp.MustCompile(`[()[\]{}+?\\^$|]`)

func pathHasRegex(path string) bool {
	return pathRegexPattern.MatchString(path)
}

func supportedSessionCookieHash(hashAlgo string) bool {
	switch strings.ToLower(strings.TrimSpace(hashAlgo)) {
	case "sha1", "md5", "sha256":
		return true
	default:
		return false
	}
}

func hasCookieAffinity(annotations map[string]string) bool {
	v, ok := getAnnotation(annotations, "affinity")
	return ok && strings.EqualFold(strings.TrimSpace(v), "cookie")
}

func sessionCookieName(annotations map[string]string) string {
	cookieName, ok := getAnnotation(annotations, "session-cookie-name")
	cookieName = strings.TrimSpace(cookieName)
	if !ok || cookieName == "" {
		return "INGRESSCOOKIE"
	}
	return cookieName
}

// --- known annotation suffixes that are fully handled (converted or plugin-generated) ---

var handledAnnotations = map[string]bool{
	// Fully converted to APISIX native annotations
	"enable-cors":            true,
	"cors-allow-origin":      true,
	"cors-allow-methods":     true,
	"cors-allow-headers":     true,
	"ssl-redirect":           true,
	"force-ssl-redirect":     true,
	"rewrite-target":         true,
	"proxy-connect-timeout":  true,
	"proxy-send-timeout":     true,
	"proxy-read-timeout":     true,
	"backend-protocol":       true,
	"whitelist-source-range": true,
	"auth-url":               true,
	"auth-method":            true,
	"auth-request-headers":   true,
	"auth-response-headers":  true,
	"websocket-services":     true,
	"use-regex":              true,
	// Auth via native APISIX annotation
	"auth-type":   true,
	"auth-signin": true,
	// Session cookie hash via custom APISIX plugin
	"session-cookie-hash": true,
	// Redirect via native APISIX annotation
	"proxy-redirect-from": true,
	"proxy-redirect-to":   true,
	// Configuration snippet (may produce PluginConfig or native annotations)
	"configuration-snippet": true,
	// proxy-cookie-path → proxy-cookie-path custom plugin
	"proxy-cookie-path": true,
	// Rate limiting → PluginConfig (no native APISIX annotation for these)
	"limit-rps":         true,
	"limit-rpm":         true,
	"limit-connections": true,
	// limit-multiplier → multiplies limit-rps/limit-rpm values
	"limit-multiplier": true,
	// proxy-body-size → client-control plugin
	"proxy-body-size": true,
	// CORS credentials, max-age, and expose-headers → cors plugin config
	"cors-allow-credentials": true,
	"cors-max-age":           true,
	"cors-expose-headers":    true,
	// upstream-vhost → proxy-rewrite plugin host
	"upstream-vhost": true,
	// Real IP → real-ip plugin in PluginConfig
	"enable-real-ip":             true,
	"use-forwarded-headers":      true,
	"compute-full-forwarded-for": true,
	"forwarded-for-header":       true,
	// SSL verify → proxy-ssl-verify config
	"ssl-verify": true,
	// Access log → native APISIX annotation
	"enable-access-log": true,
	// upstream-hash-by → BackendTrafficPolicy with chash
	"upstream-hash-by": true,
	// Health checks → ApisixUpstream
	"health-check-interval": true,
	"health-check-path":     true,
	"health-check-retries":  true,
	"health-check-timeout":  true,
	// Session cookie extensions → extend session-cookie-hash plugin config
	"session-cookie-expires":                   true,
	"session-cookie-max-age":                   true,
	"session-cookie-path":                      true,
	"session-cookie-conditional-samesite-none": true,
	// denylist → blocklist conversion
	"denylist-source-range": true,
	// Redirect annotations → http-redirect
	"permanent-redirect": true,
	"temporal-redirect":  true,
	"app-root":           true,
	// Custom error pages → custom-error-page plugin via native annotation
	"custom-http-errors": true,
	// auth-realm → native APISIX auth-realm annotation
	"auth-realm": true,
	// proxy-request-buffering → proxy-control plugin
	"proxy-request-buffering": true,
	// affinity-mode → consumed by BackendTrafficPolicy with cookie affinity
	"affinity-mode": true,
}

// knownManualAnnotations are recognized but require manual intervention.
// The value is the recommended migration approach.
var knownManualAnnotations = map[string]string{
	"affinity":                       "需 BackendTrafficPolicy CRD 实现会话亲和，参见迁移文档 4.1.1",
	"session-cookie-name":            "需 BackendTrafficPolicy CRD 实现会话亲和，参见迁移文档 4.1.1",
	"auth-tls-secret":                "需 ApisixTls CRD 实现 mTLS，参见迁移文档 4.1.6",
	"auth-tls-verify-client":         "需 ApisixTls CRD 实现 mTLS，参见迁移文档 4.1.6",
	"proxy-buffer-size":              "需全局配置 nginx_config.http_configuration_snippet.proxy_buffer_size，参见迁移文档 3.1",
	"proxy-buffers-number":           "需全局配置 nginx_config.http_configuration_snippet.proxy_buffers，参见迁移文档 3.1",
	"proxy-set-headers":              "需 proxy-rewrite 插件的 headers.set 配置，但注解引用的是 ConfigMap 中的 header 值，需手动迁移",
	"canary":                         "需 ApisixRoute + traffic-split 插件或 header/cookie 条件路由",
	"canary-weight":                  "需 ApisixRoute + traffic-split 插件实现权重分流",
	"canary-by-header":               "需 ApisixRoute 条件路由（基于 header）",
	"canary-by-header-value":         "需配合 canary-by-header 使用",
	"canary-by-cookie":               "需 ApisixRoute 条件路由（基于 cookie）",
	"ssl-passthrough":                "需 APISIX stream_proxy 配置 + ApisixTls CRD",
	"ssl-protocols":                  "需全局配置 apisix.ssl.protocols",
	"ssl-ciphers":                    "需全局配置 apisix.ssl.ciphers",
	"hsts":                           "需 response-rewrite 插件或全局配置注入 Strict-Transport-Security 头",
	"hsts-max-age":                   "需配合 hsts 使用",
	"hsts-include-subdomains":        "需配合 hsts 使用",
	"hsts-preload":                   "需配合 hsts 使用",
	"server-snippet":                 "服务级别 NGINX 配置片段，需逐条分析迁移方案",
	"proxy-http-version":             "需全局配置上游 HTTP 版本",
	"service-upstream":               "需 APISIX 全局 DNS 解析模式配置",
	"disable-access-log":             "需全局配置 nginx_config.http.access_log",
	"connection-proxy-header":        "需 proxy-rewrite 插件设置 Connection 头",
	"x-forwarded-prefix":             "需 proxy-rewrite 插件添加 X-Forwarded-Prefix 头",
	"preserve-trailing-slash":        "需 proxy-rewrite 插件保留尾斜杠",
	"retry-non-idempotent":           "需 APISIX 全局重试配置",
	"satisfy":                        "需 APISIX 多认证插件组合配置",
	"enable-opentracing":             "需 APISIX 全局 OpenTracing 配置",
	"enable-influxdb":                "需 APISIX 全局 InfluxDB 配置",
	"mirror-target":                  "需 APISIX plugin_attr.mirror 配置实现流量镜像",
	"mirror-path":                    "需配合 mirror-target 使用",
	"enable-owasp-modsecurity-crc":   "需 APISIX waf 插件配置",
	"modsecurity-transaction-id":     "需配合 ModSecurity 使用",
	"modsecurity-snippet":            "需配合 ModSecurity 使用",
	"proxy-buffering":                "需全局 nginx snippet proxy_buffering 配置",
	"auth-secret":                    "AIC 不支持此注解；需手动创建 ApisixConsumer CRD 配置 basic-auth 凭证",
	"upstream-keepalive-connections": "AIC ApisixUpstream CRD 不支持 keepalive；需在 APISIX 全局配置中设置",
	"upstream-keepalive-requests":    "AIC ApisixUpstream CRD 不支持 keepalive；需在 APISIX 全局配置中设置",
	"upstream-keepalive-timeout":     "AIC ApisixUpstream CRD 不支持 keepalive；需在 APISIX 全局配置中设置",
}

// buildConvertedIngress copies the Ingress, replaces nginx annotations with
// APISIX equivalents, and sets the ingressClassName to apisix.
func (c *Converter) buildConvertedIngress(ing ingress.Ingress, ns string, result *apisix.ConversionResult) ingress.Ingress {
	out := ing // shallow copy
	out.Metadata.Namespace = ns

	// Set APISIX ingress class
	out.Spec.IngressClassName = &c.opts.TargetIngressClassName

	// Copy and transform annotations, stripping nginx-specific ones
	newAnnotations := make(map[string]string)
	for k, v := range ing.Metadata.Annotations {
		if isNginxAnnotation(k) {
			continue // will be converted below
		}
		if k == "kubernetes.io/ingress.class" && strings.EqualFold(v, "nginx") {
			continue // strip nginx ingress.class annotation, use spec.ingressClassName instead
		}
		newAnnotations[k] = v
	}

	// Add "managed-by" label
	if out.Metadata.Labels == nil {
		out.Metadata.Labels = make(map[string]string)
	}
	out.Metadata.Labels["managed-by"] = "ingress2apisix"

	// Convert nginx annotations to APISIX annotations, tracking sources
	sources := make(map[string]string)
	c.convertAnnotations(ing.Metadata.Annotations, newAnnotations, sources)

	// Post-check: auth-type=digest has no APISIX equivalent, produce warning
	if v, ok := getAnnotation(ing.Metadata.Annotations, "auth-type"); ok && strings.ToLower(v) == "digest" {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("[%s/%s] auth-type=digest: HTTP Digest 认证在 APISIX 中无等价实现，需手动评估替代方案",
				ns, ing.Metadata.Name))
	}

	// Default SSL redirect when TLS is configured and option is enabled
	if c.opts.SSLRedirect && len(ing.Spec.TLS) > 0 {
		if _, exists := newAnnotations["k8s.apisix.apache.org/http-to-https"]; !exists {
			newAnnotations["k8s.apisix.apache.org/http-to-https"] = "true"
			sources["k8s.apisix.apache.org/http-to-https"] = "DEFAULT_HINT:auto-enabled because spec.tls is configured"
		}
	}

	out.Metadata.Annotations = newAnnotations
	out.Metadata.SourceAnnotations = sources

	// Fix pathType per migration doc section 3.2 and set use-regex when needed
	c.fixPathTypes(&out, newAnnotations)

	// Warn about unhandled annotations
	c.warnUnhandledAnnotations(ing, result)

	return out
}

// warnUnhandledAnnotations scans the original Ingress for nginx/ingress annotations
// that were neither converted nor passed through, and adds warnings.
func (c *Converter) warnUnhandledAnnotations(ing ingress.Ingress, result *apisix.ConversionResult) {
	if ing.Metadata.Annotations == nil {
		return
	}

	seen := make(map[string]bool) // deduplicate suffixes seen under both prefixes
	for k, v := range ing.Metadata.Annotations {
		if !isNginxAnnotation(k) {
			continue
		}
		suffix := annotationSuffix(k)
		if suffix == "" || seen[suffix] {
			continue
		}
		seen[suffix] = true

		// Already handled (converted to annotation or plugin)
		if handledAnnotations[suffix] {
			continue
		}
		// session-cookie-name is consumed by session-cookie-hash custom plugin conversion.
		if suffix == "session-cookie-name" && hasAnnotation(ing.Metadata.Annotations, "session-cookie-hash") {
			continue
		}
		// affinity=cookie and session-cookie-name are consumed by BackendTrafficPolicy conversion.
		if hasCookieAffinity(ing.Metadata.Annotations) && (suffix == "affinity" || suffix == "session-cookie-name") {
			continue
		}
		// Known manual annotations → specific migration guidance
		if hint, ok := knownManualAnnotations[suffix]; ok {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("[%s/%s] 注解 %s 无法自动转换: %s",
					ing.Metadata.Namespace, ing.Metadata.Name, k, hint))
			continue
		}

		// Unknown annotation → generic warning
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("[%s/%s] 注解 %s=%q 未被识别，无法自动转换，请手动迁移",
				ing.Metadata.Namespace, ing.Metadata.Name, k, v))
	}

	// Also check for configuration-snippet content that wasn't rewrite
	if snippet, ok := getAnnotation(ing.Metadata.Annotations, "configuration-snippet"); ok {
		if strings.Contains(snippet, "more_set_headers") && !seen["more_set_headers_in_snippet"] {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("[%s/%s] configuration-snippet 中包含 more_set_headers，建议使用 ApisixPluginConfig + proxy-rewrite 插件实现，参见迁移文档 4.1.5",
					ing.Metadata.Namespace, ing.Metadata.Name))
		}
		if strings.Contains(snippet, "custom-http-errors") && !seen["custom_http_errors_in_snippet"] {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("[%s/%s] configuration-snippet 中包含自定义错误处理，需自定义 Lua 插件，参见迁移文档 4.1.3",
					ing.Metadata.Namespace, ing.Metadata.Name))
		}
	}
}

// srcAnnot returns the full original nginx annotation key for a suffix.
// It tries the nginx prefix first, then the plain ingress prefix.
func srcAnnot(annotations map[string]string, suffix string) string {
	if _, ok := annotations[prefixNginx+suffix]; ok {
		return prefixNginx + suffix
	}
	if _, ok := annotations[prefixPlain+suffix]; ok {
		return prefixPlain + suffix
	}
	return prefixNginx + suffix // fallback
}

// convertAnnotations maps nginx/ingress annotations to APISIX annotations.
// sources, if non-nil, is populated with output key → original nginx annotation key mappings.
func (c *Converter) convertAnnotations(src, out map[string]string, sources map[string]string) {
	if src == nil {
		return
	}

	// --- CORS ---
	// CORS is fully handled by ApisixPluginConfig (cors plugin) in buildPluginConfig,
	// which includes credentials, max-age, expose-headers that AIC annotations don't support.
	// No AIC annotations are emitted here to avoid duplicate/conflicting CORS config.

	// --- SSL redirect → http-to-https ---
	if v, ok := getAnnotation(src, "ssl-redirect"); ok && v == "true" {
		out["k8s.apisix.apache.org/http-to-https"] = "true"
		if sources != nil {
			sources["k8s.apisix.apache.org/http-to-https"] = srcAnnot(src, "ssl-redirect")
		}
	}
	if hasAnnotation(src, "force-ssl-redirect") {
		out["k8s.apisix.apache.org/http-to-https"] = "true"
		if sources != nil {
			sources["k8s.apisix.apache.org/http-to-https"] = srcAnnot(src, "force-ssl-redirect")
		}
	}

	// --- Proxy redirect: http:// → https:// maps to http-to-https annotation ---
	if from, ok := getAnnotation(src, "proxy-redirect-from"); ok {
		if to, ok := getAnnotation(src, "proxy-redirect-to"); ok {
			if isSSLRedirect(from, to) {
				out["k8s.apisix.apache.org/http-to-https"] = "true"
				if sources != nil {
					sources["k8s.apisix.apache.org/http-to-https"] = srcAnnot(src, "proxy-redirect-from") + ", " + srcAnnot(src, "proxy-redirect-to")
				}
			}
		}
	}

	combinedRewrite := hasCombinedRewrite(src)

	// --- Rewrite target ---
	if v, ok := getAnnotation(src, "rewrite-target"); ok && !combinedRewrite {
		if regexCapturePattern.MatchString(v) {
			out["k8s.apisix.apache.org/rewrite-target-regex"] = v
			out["k8s.apisix.apache.org/rewrite-target-regex-template"] = v
			if sources != nil {
				s := srcAnnot(src, "rewrite-target")
				sources["k8s.apisix.apache.org/rewrite-target-regex"] = s
				sources["k8s.apisix.apache.org/rewrite-target-regex-template"] = s
			}
		} else {
			out["k8s.apisix.apache.org/rewrite-target"] = v
			if sources != nil {
				sources["k8s.apisix.apache.org/rewrite-target"] = srcAnnot(src, "rewrite-target")
			}
		}
	}

	// --- Proxy timeouts → upstream timeouts with 's' suffix ---
	if v, ok := getAnnotation(src, "proxy-connect-timeout"); ok {
		out["k8s.apisix.apache.org/upstream-connect-timeout"] = ensureTimeSuffix(v)
		if sources != nil {
			sources["k8s.apisix.apache.org/upstream-connect-timeout"] = srcAnnot(src, "proxy-connect-timeout")
		}
	}
	if v, ok := getAnnotation(src, "proxy-send-timeout"); ok {
		out["k8s.apisix.apache.org/upstream-send-timeout"] = ensureTimeSuffix(v)
		if sources != nil {
			sources["k8s.apisix.apache.org/upstream-send-timeout"] = srcAnnot(src, "proxy-send-timeout")
		}
	}
	if v, ok := getAnnotation(src, "proxy-read-timeout"); ok {
		out["k8s.apisix.apache.org/upstream-read-timeout"] = ensureTimeSuffix(v)
		if sources != nil {
			sources["k8s.apisix.apache.org/upstream-read-timeout"] = srcAnnot(src, "proxy-read-timeout")
		}
	}

	// --- Backend protocol → upstream-scheme ---
	if v, ok := getAnnotation(src, "backend-protocol"); ok {
		out["k8s.apisix.apache.org/upstream-scheme"] = strings.ToLower(v)
		if sources != nil {
			sources["k8s.apisix.apache.org/upstream-scheme"] = srcAnnot(src, "backend-protocol")
		}
	}

	// --- Whitelist → allowlist-source-range ---
	if v, ok := getAnnotation(src, "whitelist-source-range"); ok {
		out["k8s.apisix.apache.org/allowlist-source-range"] = v
		if sources != nil {
			sources["k8s.apisix.apache.org/allowlist-source-range"] = srcAnnot(src, "whitelist-source-range")
		}
	}

	// --- External auth: auth-url → auth-uri (always use native annotation) ---
	if v, ok := getAnnotation(src, "auth-url"); ok {
		out["k8s.apisix.apache.org/auth-uri"] = v
		if sources != nil {
			sources["k8s.apisix.apache.org/auth-uri"] = srcAnnot(src, "auth-url")
		}
	}
	if v, ok := getAnnotation(src, "auth-method"); ok && v != "" {
		out["k8s.apisix.apache.org/auth-method"] = strings.ToUpper(v)
		if sources != nil {
			sources["k8s.apisix.apache.org/auth-method"] = srcAnnot(src, "auth-method")
		}
	}
	if v, ok := getAnnotation(src, "auth-request-headers"); ok && v != "" {
		out["k8s.apisix.apache.org/auth-request-headers"] = v
		if sources != nil {
			sources["k8s.apisix.apache.org/auth-request-headers"] = srcAnnot(src, "auth-request-headers")
		}
	}
	if v, ok := getAnnotation(src, "auth-response-headers"); ok {
		out["k8s.apisix.apache.org/auth-upstream-headers"] = v
		if sources != nil {
			sources["k8s.apisix.apache.org/auth-upstream-headers"] = srcAnnot(src, "auth-response-headers")
		}
	}
	// --- auth-signin → native APISIX auth-signin annotation ---
	if v, ok := getAnnotation(src, "auth-signin"); ok {
		out["k8s.apisix.apache.org/auth-signin"] = v
		if sources != nil {
			sources["k8s.apisix.apache.org/auth-signin"] = srcAnnot(src, "auth-signin")
		}
	}

	// --- Auth type: basic/key → native APISIX auth-type annotation ---
	if v, ok := getAnnotation(src, "auth-type"); ok {
		switch strings.ToLower(v) {
		case "basic":
			out["k8s.apisix.apache.org/auth-type"] = "basicAuth"
		case "digest":
			// HTTP Digest auth has no APISIX equivalent; warn instead of mapping
		default:
			out["k8s.apisix.apache.org/auth-type"] = v
		}
		if strings.ToLower(v) != "digest" && sources != nil {
			sources["k8s.apisix.apache.org/auth-type"] = srcAnnot(src, "auth-type")
		}
	}

	// --- auth-secret → NOT supported by AIC, warn user ---
	// AIC does not have a handler for auth-secret. Credentials must be provided
	// via an ApisixConsumer CRD with embedded basic-auth credentials.
	// (removed: was silently ignored by AIC)

	// --- auth-realm → native APISIX auth-realm annotation ---
	if v, ok := getAnnotation(src, "auth-realm"); ok && v != "" {
		out["k8s.apisix.apache.org/auth-realm"] = v
		if sources != nil {
			sources["k8s.apisix.apache.org/auth-realm"] = srcAnnot(src, "auth-realm")
		}
	}

	// --- custom-http-errors → custom-error-codes native APISIX annotation ---
	if v, ok := getAnnotation(src, "custom-http-errors"); ok && v != "" {
		out["k8s.apisix.apache.org/custom-error-codes"] = v
		if sources != nil {
			sources["k8s.apisix.apache.org/custom-error-codes"] = srcAnnot(src, "custom-http-errors")
		}
	}

	// --- WebSocket ---
	if v, ok := getAnnotation(src, "websocket-services"); ok && v != "" {
		out["k8s.apisix.apache.org/enable-websocket"] = "true"
		if sources != nil {
			sources["k8s.apisix.apache.org/enable-websocket"] = srcAnnot(src, "websocket-services")
		}
	}

	// --- use-regex ---
	if v, ok := getAnnotation(src, "use-regex"); ok && v == "true" {
		out["k8s.apisix.apache.org/use-regex"] = "true"
		if sources != nil {
			sources["k8s.apisix.apache.org/use-regex"] = srcAnnot(src, "use-regex")
		}
	}

	// --- enable-access-log → native APISIX annotation ---
	if v, ok := getAnnotation(src, "enable-access-log"); ok {
		if v == "false" || v == "off" {
			out["k8s.apisix.apache.org/enable-access-log"] = "false"
		} else {
			out["k8s.apisix.apache.org/enable-access-log"] = "true"
		}
		if sources != nil {
			sources["k8s.apisix.apache.org/enable-access-log"] = srcAnnot(src, "enable-access-log")
		}
	}

	// --- denylist-source-range → blocklist-source-range ---
	if v, ok := getAnnotation(src, "denylist-source-range"); ok {
		out["k8s.apisix.apache.org/blocklist-source-range"] = v
		if sources != nil {
			sources["k8s.apisix.apache.org/blocklist-source-range"] = srcAnnot(src, "denylist-source-range")
		}
	}

	// --- permanent-redirect → http-redirect + 308 ---
	if v, ok := getAnnotation(src, "permanent-redirect"); ok && v != "" {
		out["k8s.apisix.apache.org/http-redirect"] = v
		out["k8s.apisix.apache.org/http-redirect-code"] = "308"
		if sources != nil {
			s := srcAnnot(src, "permanent-redirect")
			sources["k8s.apisix.apache.org/http-redirect"] = s
			sources["k8s.apisix.apache.org/http-redirect-code"] = s
		}
	}

	// --- temporal-redirect → http-redirect + 302 ---
	if v, ok := getAnnotation(src, "temporal-redirect"); ok && v != "" {
		out["k8s.apisix.apache.org/http-redirect"] = v
		out["k8s.apisix.apache.org/http-redirect-code"] = "302"
		if sources != nil {
			s := srcAnnot(src, "temporal-redirect")
			sources["k8s.apisix.apache.org/http-redirect"] = s
			sources["k8s.apisix.apache.org/http-redirect-code"] = s
		}
	}

	// --- app-root → http-redirect ---
	if v, ok := getAnnotation(src, "app-root"); ok && v != "" {
		out["k8s.apisix.apache.org/http-redirect"] = v
		if sources != nil {
			sources["k8s.apisix.apache.org/http-redirect"] = srcAnnot(src, "app-root")
		}
	}

	// --- configuration-snippet: single rewrite → native rewrite-target-regex annotations ---
	if snippet, ok := getAnnotation(src, "configuration-snippet"); ok && !combinedRewrite {
		rewriteURIs := c.extractRewriteURIs(snippet)
		if len(rewriteURIs) == 2 {
			// Single rewrite directive → use native APISIX annotations
			out["k8s.apisix.apache.org/rewrite-target-regex"] = rewriteURIs[0]
			out["k8s.apisix.apache.org/rewrite-target-regex-template"] = rewriteURIs[1]
			if sources != nil {
				s := srcAnnot(src, "configuration-snippet")
				sources["k8s.apisix.apache.org/rewrite-target-regex"] = s
				sources["k8s.apisix.apache.org/rewrite-target-regex-template"] = s
			}
		}
		// Multiple rewrites (len > 2) are handled by buildPluginConfig
	}
}

func hasCombinedRewrite(annotations map[string]string) bool {
	if _, ok := getAnnotation(annotations, "rewrite-target"); !ok {
		return false
	}
	snippet, ok := getAnnotation(annotations, "configuration-snippet")
	if !ok {
		return false
	}
	return len(extractRewriteURIs(snippet)) > 0
}

// isSSLRedirect returns true if from/to pair represents an HTTP→HTTPS redirect.
func isSSLRedirect(from, to string) bool {
	f := strings.TrimSpace(strings.ToLower(from))
	t := strings.TrimSpace(strings.ToLower(to))
	return (f == "http://" || f == "http") && (t == "https://" || t == "https")
}

// buildPluginConfig creates an ApisixPluginConfig for annotations that cannot
// be mapped to APISIX native annotations. The PluginConfig must be linked
// to the Ingress via k8s.apisix.apache.org/plugin-config-name annotation.
func (c *Converter) buildPluginConfig(ing ingress.Ingress, ns string) (*apisix.ApisixPluginConfig, []string) {
	anns := ing.Metadata.Annotations
	if anns == nil {
		return nil, nil
	}

	var plugins []apisix.Plugin
	var warnings []string
	pluginSources := make(map[string]string)       // plugin name → source annotation suffix(es)
	pluginFieldDefaults := make(map[string]string) // plugin field defaults for comment injection

	// --- Rate limiting → limit-req plugin ---
	// APISIX has no native annotation for rate limiting, so PluginConfig is needed.
	if v, ok := getAnnotation(anns, "limit-rps"); ok && v != "" {
		rejectedCode := "429"
		if snippet, ok := getAnnotation(anns, "configuration-snippet"); ok {
			re := regexp.MustCompile(`limit_req_status\s+(\d{3})`)
			if m := re.FindStringSubmatch(snippet); len(m) > 1 {
				rejectedCode = m[1]
			}
		}
		rateValue := applyLimitMultiplier(v, anns)
		plugins = append(plugins, apisix.Plugin{
			Name:   "limit-req",
			Enable: true,
			Config: map[string]interface{}{
				"rate":          rateValue,
				"burst":         0,
				"key":           "remote_addr",
				"rejected_code": rejectedCode,
			},
		})
		pluginSources["limit-req"] = srcAnnot(anns, "limit-rps")
		pluginFieldDefaults["limit-req.burst"] = "DEFAULT_HINT:hardcoded, same as nginx default"
		pluginFieldDefaults["limit-req.key"] = "DEFAULT_HINT:hardcoded, same as nginx default"
		if rejectedCode == "429" {
			pluginFieldDefaults["limit-req.rejected_code"] = "DEFAULT_HINT:hardcoded, same as nginx default"
		}
	} else if v, ok := getAnnotation(anns, "limit-rpm"); ok && v != "" {
		rejectedCode := "429"
		rateBase := applyLimitMultiplier(v, anns)
		// APISIX limit-req schema requires rate as requests/sec (number).
		// Convert rpm to rps by dividing by 60.
		rateFloat, err := strconv.ParseFloat(strings.TrimSpace(rateBase), 64)
		if err != nil {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] limit-rpm=%q 无法解析为数字",
					ing.Metadata.Namespace, ing.Metadata.Name, rateBase))
		} else {
			rps := rateFloat / 60.0
			plugins = append(plugins, apisix.Plugin{
				Name:   "limit-req",
				Enable: true,
				Config: map[string]interface{}{
					"rate":          rps,
					"burst":         0,
					"key":           "remote_addr",
					"rejected_code": rejectedCode,
				},
			})
			pluginSources["limit-req"] = srcAnnot(anns, "limit-rpm")
			pluginFieldDefaults["limit-req.burst"] = "DEFAULT_HINT:hardcoded, same as nginx default"
			pluginFieldDefaults["limit-req.key"] = "DEFAULT_HINT:hardcoded, same as nginx default"
			pluginFieldDefaults["limit-req.rejected_code"] = "DEFAULT_HINT:hardcoded, same as nginx default"
		}
	}

	// --- Limit connections → limit-conn plugin ---
	if v, ok := getAnnotation(anns, "limit-connections"); ok && v != "" {
		plugins = append(plugins, apisix.Plugin{
			Name:   "limit-conn",
			Enable: true,
			Config: map[string]interface{}{
				"conn":          v,
				"burst":         0,
				"key":           "remote_addr",
				"rejected_code": "503",
			},
		})
		pluginSources["limit-conn"] = srcAnnot(anns, "limit-connections")
		pluginFieldDefaults["limit-conn.burst"] = "DEFAULT_HINT:hardcoded, same as nginx default"
		pluginFieldDefaults["limit-conn.key"] = "DEFAULT_HINT:hardcoded, same as nginx default"
		pluginFieldDefaults["limit-conn.rejected_code"] = "DEFAULT_HINT:hardcoded, same as nginx default"
	}

	// --- proxy-body-size → client-control plugin ---
	if v, ok := getAnnotation(anns, "proxy-body-size"); ok && v != "" {
		parsedBytes, err := parseBodySize(v)
		if err != nil {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] proxy-body-size=%q 无法解析为有效的字节数，跳过自动转换: %v",
					ing.Metadata.Namespace, ing.Metadata.Name, v, err))
		} else {
			plugins = append(plugins, apisix.Plugin{
				Name:   "client-control",
				Enable: true,
				Config: map[string]interface{}{
					"max_body_size": parsedBytes,
				},
			})
			pluginSources["client-control"] = srcAnnot(anns, "proxy-body-size")
		}
	}

	// --- proxy-request-buffering → proxy-control plugin ---
	if v, ok := getAnnotation(anns, "proxy-request-buffering"); ok {
		if v == "off" || v == "false" {
			plugins = append(plugins, apisix.Plugin{
				Name:   "proxy-control",
				Enable: true,
				Config: map[string]interface{}{
					"request_buffering": false,
				},
			})
			pluginSources["proxy-control"] = srcAnnot(anns, "proxy-request-buffering")
		}
	}

	// --- Complex rewrite: multiple rewrites or rewrite-target + snippet rewrites ---
	if rewriteURIs := c.buildProxyRewriteURIs(ing); len(rewriteURIs) > 0 {
		if len(rewriteURIs) > 2 || hasCombinedRewrite(anns) {
			plugins = append(plugins, apisix.Plugin{
				Name:   "proxy-rewrite",
				Enable: true,
				Config: map[string]interface{}{
					"regex_uri": rewriteURIs,
				},
			})
			var srcs []string
			if _, ok := getAnnotation(anns, "rewrite-target"); ok {
				srcs = append(srcs, srcAnnot(anns, "rewrite-target"))
			}
			if _, ok := getAnnotation(anns, "configuration-snippet"); ok {
				srcs = append(srcs, srcAnnot(anns, "configuration-snippet"))
			}
			pluginSources["proxy-rewrite"] = strings.Join(srcs, ", ")
		}
		// Single snippet rewrite without rewrite-target is handled as native annotation above.
	}

	// --- proxy-redirect-from/to: non-SSL redirects need manual handling ---
	if from, ok := getAnnotation(anns, "proxy-redirect-from"); ok {
		if to, ok := getAnnotation(anns, "proxy-redirect-to"); ok {
			if !isSSLRedirect(from, to) {
				warnings = append(warnings,
					fmt.Sprintf("[%s/%s] proxy-redirect-from=%s proxy-redirect-to=%s 不是 HTTP→HTTPS 场景，无法自动转换，需手动处理 scheme 重定向",
						ing.Metadata.Namespace, ing.Metadata.Name, from, to))
			}
		}
	}

	// --- proxy-cookie-path → proxy-cookie-path custom plugin ---
	if cookiePathAnnot, ok := getAnnotation(anns, "proxy-cookie-path"); ok {
		pathPairs := parseProxyCookiePath(cookiePathAnnot)
		if len(pathPairs) > 0 {
			plugins = append(plugins, apisix.Plugin{
				Name:   "proxy-cookie-path",
				Enable: true,
				Config: map[string]interface{}{
					"path_pairs": pathPairs,
				},
			})
			pluginSources["proxy-cookie-path"] = srcAnnot(anns, "proxy-cookie-path")
		} else {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] proxy-cookie-path=%q 无法解析为有效的路径替换规则，跳过自动转换",
					ing.Metadata.Namespace, ing.Metadata.Name, cookiePathAnnot))
		}
	}

	// --- proxy_cookie_flags in configuration-snippet → proxy-cookie-flags plugin ---
	if snippet, ok := getAnnotation(anns, "configuration-snippet"); ok {
		rules := parseProxyCookieFlags(snippet)
		if len(rules) > 0 {
			plugins = append(plugins, apisix.Plugin{
				Name:   "proxy-cookie-flags",
				Enable: true,
				Config: map[string]interface{}{
					"rules": rules,
				},
			})
			pluginSources["proxy-cookie-flags"] = srcAnnot(anns, "configuration-snippet")
		}
	}

	// --- Session cookie hash → session-cookie-hash custom plugin ---
	hashAlgo, hasHashAlgo := getAnnotation(anns, "session-cookie-hash")
	algoIsDefault := false
	if !hasHashAlgo && hasCookieAffinity(anns) {
		hashAlgo = "sha1"
		hasHashAlgo = true
		algoIsDefault = true
		warnings = append(warnings,
			fmt.Sprintf("[%s/%s] affinity=cookie 未配置 session-cookie-hash，默认使用 sha1 生成 session-cookie-hash 插件；如需其他算法请显式配置 sha1/md5/sha256",
				ing.Metadata.Namespace, ing.Metadata.Name))
	}
	if hasHashAlgo {
		algo := strings.ToLower(strings.TrimSpace(hashAlgo))
		if supportedSessionCookieHash(algo) {
			if !hasAnnotation(anns, "session-cookie-name") {
				warnings = append(warnings,
					fmt.Sprintf("[%s/%s] 未配置 session-cookie-name，session-cookie-hash 插件将使用默认 cookie_name=INGRESSCOOKIE（首次请求无该 Cookie 时插件会自动生成并返回 Set-Cookie）",
						ing.Metadata.Namespace, ing.Metadata.Name))
			}
			cookieConfig := map[string]interface{}{
				"cookie_name":     sessionCookieName(anns),
				"algorithm":       algo,
				"header_name":     "X-Session-Hash",
				"fallback":        "pass",
				"generate_cookie": true,
				"cookie_httponly": false,
			}
			fieldDefaults := make(map[string]string)
			if !hasAnnotation(anns, "session-cookie-name") {
				fieldDefaults["session-cookie-hash.cookie_name"] = "DEFAULT:" + srcAnnot(anns, "session-cookie-name")
			}
			fieldDefaults["session-cookie-hash.header_name"] = "DEFAULT_HINT:hardcoded default"
			fieldDefaults["session-cookie-hash.fallback"] = "DEFAULT_HINT:hardcoded default"
			fieldDefaults["session-cookie-hash.generate_cookie"] = "DEFAULT_HINT:hardcoded default"
			fieldDefaults["session-cookie-hash.cookie_httponly"] = "DEFAULT_HINT:hardcoded default"
			if algoIsDefault {
				fieldDefaults["session-cookie-hash.algorithm"] = "DEFAULT:" + srcAnnot(anns, "session-cookie-hash")
			}
			var sessionSrcs []string
			sessionSrcs = append(sessionSrcs, srcAnnot(anns, "session-cookie-hash"))
			// Extend with session-cookie-* options
			if v, ok := getAnnotation(anns, "session-cookie-max-age"); ok && v != "" {
				if maxAge, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					cookieConfig["max_age"] = maxAge
					sessionSrcs = append(sessionSrcs, srcAnnot(anns, "session-cookie-max-age"))
				}
			} else if v, ok := getAnnotation(anns, "session-cookie-expires"); ok && v != "" {
				if expires, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					cookieConfig["max_age"] = expires
					sessionSrcs = append(sessionSrcs, srcAnnot(anns, "session-cookie-expires"))
				}
			}
			if v, ok := getAnnotation(anns, "session-cookie-path"); ok && v != "" {
				cookieConfig["cookie_path"] = strings.TrimSpace(v)
				sessionSrcs = append(sessionSrcs, srcAnnot(anns, "session-cookie-path"))
			}
			if _, ok := getAnnotation(anns, "session-cookie-conditional-samesite-none"); ok {
				warnings = append(warnings,
					fmt.Sprintf("[%s/%s] session-cookie-conditional-samesite-none: APISIX session-cookie-hash 插件无此参数，SameSite=None 需通过代理 Cookie 标志或应用层处理",
						ing.Metadata.Namespace, ing.Metadata.Name))
			}
			plugins = append(plugins, apisix.Plugin{
				Name:   "session-cookie-hash",
				Enable: true,
				Config: cookieConfig,
			})
			pluginSources["session-cookie-hash"] = strings.Join(sessionSrcs, ", ")
			for k, v := range fieldDefaults {
				pluginFieldDefaults[k] = v
			}
		} else {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] session-cookie-hash=%q 不支持自动转换，仅支持 sha1/md5/sha256；请改用支持值并配合 session-cookie-name。注意：APISIX Ingress 没有 upstream-hash 注解，会话亲和仍需 BackendTrafficPolicy 等资源承载",
					ing.Metadata.Namespace, ing.Metadata.Name, hashAlgo))
		}
	}

	// --- CORS → cors plugin config (always when enable-cors is true) ---
	// ingress-nginx defaults: allow_credential=true, max_age=1728000,
	// methods/headers = specific lists (not "*"). When allow_credential is true,
	// APISIX forbids "*" for allow_origins/allow_methods/allow_headers.
	// For wildcard origin + credentials, we use allow_origins_by_regex instead.
	_, enableCors := getAnnotation(anns, "enable-cors")
	_, hasExplicitCreds := getAnnotation(anns, "cors-allow-credentials")
	_, hasExplicitMaxAge := getAnnotation(anns, "cors-max-age")
	_, hasExplicitExpose := getAnnotation(anns, "cors-expose-headers")
	if enableCors {
		// Determine allow_credential value (default true per ingress-nginx)
		allowCredential := true
		corsSrcs := []string{srcAnnot(anns, "enable-cors")}
		if hasExplicitCreds {
			if v, _ := getAnnotation(anns, "cors-allow-credentials"); !strings.EqualFold(strings.TrimSpace(v), "true") {
				allowCredential = false
			}
			corsSrcs = append(corsSrcs, srcAnnot(anns, "cors-allow-credentials"))
		}

		// Determine allow_origins (default "*" per ingress-nginx)
		originVal := "*"
		if v, ok := getAnnotation(anns, "cors-allow-origin"); ok {
			originVal = v
			corsSrcs = append(corsSrcs, srcAnnot(anns, "cors-allow-origin"))
		}

		// Determine allow_methods (default ingress-nginx list)
		methodsVal := "GET, PUT, POST, DELETE, PATCH, OPTIONS"
		if v, ok := getAnnotation(anns, "cors-allow-methods"); ok {
			methodsVal = v
			corsSrcs = append(corsSrcs, srcAnnot(anns, "cors-allow-methods"))
		}

		// Determine allow_headers (default ingress-nginx list)
		headersVal := "DNT,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Range,Authorization"
		if v, ok := getAnnotation(anns, "cors-allow-headers"); ok {
			headersVal = v
			corsSrcs = append(corsSrcs, srcAnnot(anns, "cors-allow-headers"))
		}

		// Determine max_age (default 1728000 per ingress-nginx)
		maxAge := 1728000
		if hasExplicitMaxAge {
			if v, ok := getAnnotation(anns, "cors-max-age"); ok {
				if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
					maxAge = parsed
				}
			}
			corsSrcs = append(corsSrcs, srcAnnot(anns, "cors-max-age"))
		}

		corsConfig := map[string]interface{}{
			"allow_methods":    methodsVal,
			"allow_headers":    headersVal,
			"allow_credential": allowCredential,
			"max_age":          maxAge,
		}

		// APISIX constraint: when allow_credential is true, "*" is not allowed
		// for allow_origins, allow_methods, allow_headers.
		// Ingress-nginx defaults methods/headers to specific lists (not "*"), so those are fine.
		// For origins: default "*" with credentials requires allow_origins_by_regex workaround.
		if allowCredential && originVal == "*" {
			// Use regex to match all origins, since "*" is forbidden with credentials
			corsConfig["allow_origins_by_regex"] = []string{".*"}
		} else {
			corsConfig["allow_origins"] = originVal
		}

		// Handle expose_headers (no ingress-nginx default — only set if explicitly provided)
		if hasExplicitExpose {
			if v, ok := getAnnotation(anns, "cors-expose-headers"); ok {
				corsConfig["expose_headers"] = v
				corsSrcs = append(corsSrcs, srcAnnot(anns, "cors-expose-headers"))
			}
		}

		plugins = append(plugins, apisix.Plugin{
			Name:   "cors",
			Enable: true,
			Config: corsConfig,
		})
		pluginSources["cors"] = strings.Join(corsSrcs, ", ")

		// Record defaults for source comment injection
		if !hasExplicitCreds {
			pluginFieldDefaults["cors.allow_credential"] = "DEFAULT_HINT:ingress-nginx default is true"
		}
		if !hasExplicitMaxAge {
			pluginFieldDefaults["cors.max_age"] = "DEFAULT_HINT:ingress-nginx default is 1728000"
		}
		if _, ok := getAnnotation(anns, "cors-allow-methods"); !ok {
			pluginFieldDefaults["cors.allow_methods"] = "DEFAULT:" + srcAnnot(anns, "cors-allow-methods")
		}
		if _, ok := getAnnotation(anns, "cors-allow-headers"); !ok {
			pluginFieldDefaults["cors.allow_headers"] = "DEFAULT:" + srcAnnot(anns, "cors-allow-headers")
		}
		if originVal == "*" && !allowCredential {
			pluginFieldDefaults["cors.allow_origins"] = "DEFAULT:" + srcAnnot(anns, "cors-allow-origin")
		}
		if originVal == "*" && allowCredential {
			pluginFieldDefaults["cors.allow_origins_by_regex"] = "DEFAULT_HINT:ingress-nginx default origin is *, using regex to allow all with credentials"
		}
	}

	// --- upstream-vhost → proxy-rewrite plugin with host ---
	if v, ok := getAnnotation(anns, "upstream-vhost"); ok && v != "" {
		plugins = append(plugins, apisix.Plugin{
			Name:   "proxy-rewrite",
			Enable: true,
			Config: map[string]interface{}{
				"host": v,
			},
		})
		pluginSources["proxy-rewrite"] = srcAnnot(anns, "upstream-vhost")
	}

	// --- Real IP → real-ip plugin ---
	// Triggered by enable-real-ip=true, or presence of forwarded-for-header/use-forwarded-headers
	_, hasEnableRealIP := getAnnotation(anns, "enable-real-ip")
	_, hasForwardedHeader := getAnnotation(anns, "forwarded-for-header")
	_, hasUseForwarded := getAnnotation(anns, "use-forwarded-headers")
	if hasEnableRealIP || hasForwardedHeader || hasUseForwarded {
		enableRealIP, _ := getAnnotation(anns, "enable-real-ip")
		if enableRealIP == "true" || hasForwardedHeader || hasUseForwarded {
			realIPConfig := map[string]interface{}{
				"source":            "http_x_forwarded_for",
				"trusted_addresses": []string{"0.0.0.0/0"},
				"recursive":         false,
			}
			realIPDefaults := make(map[string]string)
			realIPDefaults["real-ip.source"] = "DEFAULT_HINT:hardcoded, same as nginx default"
			realIPDefaults["real-ip.trusted_addresses"] = "DEFAULT_HINT:hardcoded to accept all"
			realIPDefaults["real-ip.recursive"] = "DEFAULT_HINT:hardcoded default"
			var realIPSrcs []string
			if hasEnableRealIP {
				realIPSrcs = append(realIPSrcs, srcAnnot(anns, "enable-real-ip"))
			}
			if hdr, ok := getAnnotation(anns, "forwarded-for-header"); ok && hdr != "" {
				// Map common header names to APISIX source format
				source := strings.ToLower(strings.ReplaceAll(hdr, "-", "_"))
				if !strings.HasPrefix(source, "http_") {
					source = "http_" + source
				}
				realIPConfig["source"] = source
				delete(realIPDefaults, "real-ip.source") // explicitly configured
				realIPSrcs = append(realIPSrcs, srcAnnot(anns, "forwarded-for-header"))
			}
			if useFwd, ok := getAnnotation(anns, "use-forwarded-headers"); ok && useFwd == "true" {
				realIPConfig["recursive"] = true
				delete(realIPDefaults, "real-ip.recursive") // explicitly configured
				realIPSrcs = append(realIPSrcs, srcAnnot(anns, "use-forwarded-headers"))
			}
			if computeFull, ok := getAnnotation(anns, "compute-full-forwarded-for"); ok && computeFull == "true" {
				// compute-full-forwarded-for has no direct APISIX equivalent;
				// real-ip plugin only extracts the last IP when recursive=true.
				// Emit a warning for manual configuration if needed.
				warnings = append(warnings,
					fmt.Sprintf("[%s/%s] compute-full-forwarded-for=true: APISIX real-ip 插件无等价参数，需人工确认 X-Forwarded-For 处理逻辑",
						ing.Metadata.Namespace, ing.Metadata.Name))
				realIPSrcs = append(realIPSrcs, srcAnnot(anns, "compute-full-forwarded-for"))
			}
			plugins = append(plugins, apisix.Plugin{
				Name:   "real-ip",
				Enable: true,
				Config: realIPConfig,
			})
			pluginSources["real-ip"] = strings.Join(realIPSrcs, ", ")
			for k, v := range realIPDefaults {
				pluginFieldDefaults[k] = v
			}
			if !hasEnableRealIP && (hasForwardedHeader || hasUseForwarded) {
				warnings = append(warnings,
					fmt.Sprintf("[%s/%s] forwarded-for-header/use-forwarded-headers 未配合 enable-real-ip 使用，已自动启用 real-ip 插件",
						ing.Metadata.Namespace, ing.Metadata.Name))
			}
		}
	}

	// --- SSL verify → proxy-ssl-verify configuration ---
	if v, ok := getAnnotation(anns, "ssl-verify"); ok {
		if v == "false" {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] ssl-verify=false 已识别；APISIX 默认不验证上游 TLS 证书。如需显式控制，请在 ApisixUpstream CRD 中配置 tls.client_cert/tls.client_key",
					ing.Metadata.Namespace, ing.Metadata.Name))
		} else {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] ssl-verify=true: 需在 ApisixUpstream CRD 中配置 TLS 验证（tls.client_cert/tls.client_key），或使用 proxy-ssl 插件",
					ing.Metadata.Namespace, ing.Metadata.Name))
		}
	}

	if len(plugins) == 0 {
		return nil, warnings
	}

	name := fmt.Sprintf("%s-plugins", ing.Metadata.Name)
	if len(name) > 64 {
		name = name[:64]
	}

	pc := &apisix.ApisixPluginConfig{
		APIVersion: c.opts.ApisixVersion,
		Kind:       "ApisixPluginConfig",
		Metadata: apisix.Metadata{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"managed-by":   "ingress2apisix",
				"ingress-name": ing.Metadata.Name,
			},
		},
		Spec: apisix.PluginConfigSpec{
			IngressClassName: c.opts.TargetIngressClassName,
			Plugins:          plugins,
		},
		PluginSources:       pluginSources,
		PluginFieldDefaults: pluginFieldDefaults,
	}

	// Sort warnings for deterministic output
	sort.Strings(warnings)
	return pc, warnings
}

func (c *Converter) buildBackendTrafficPolicies(ing ingress.Ingress, ns string) ([]apisix.BackendTrafficPolicy, []string) {
	anns := ing.Metadata.Annotations
	if anns == nil {
		return nil, nil
	}

	var policies []apisix.BackendTrafficPolicy
	var warnings []string

	// --- Cookie affinity → chash with cookie ---
	if hasCookieAffinity(anns) {
		services := collectBackendServiceNames(ing)
		if len(services) == 0 {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] affinity=cookie 已识别，但未找到可关联的 Service，无法生成 BackendTrafficPolicy",
					ing.Metadata.Namespace, ing.Metadata.Name))
		} else {
			cookieName := sessionCookieName(anns)
			if cookieName == "INGRESSCOOKIE" && !hasAnnotation(anns, "session-cookie-name") {
				warnings = append(warnings,
					fmt.Sprintf("[%s/%s] affinity=cookie 未配置 session-cookie-name，BackendTrafficPolicy 将使用默认 key=INGRESSCOOKIE（session-cookie-hash 插件会自动生成该 Cookie）",
						ing.Metadata.Namespace, ing.Metadata.Name))
			}

			name := truncateName(sanitizeK8sName(fmt.Sprintf("%s-cookie-affinity", ing.Metadata.Name)), 63)
			var loadbalancerSrc string
			if cookieName == "INGRESSCOOKIE" && !hasAnnotation(anns, "session-cookie-name") {
				loadbalancerSrc = "DEFAULT:" + srcAnnot(anns, "session-cookie-name")
			} else {
				loadbalancerSrc = srcAnnot(anns, "affinity") + ", " + srcAnnot(anns, "session-cookie-name")
			}
			if _, ok := getAnnotation(anns, "session-cookie-hash"); ok {
				loadbalancerSrc += ", " + srcAnnot(anns, "session-cookie-hash")
			}
			sources := map[string]string{
				"loadbalancer": loadbalancerSrc,
			}
			policies = append(policies, apisix.BackendTrafficPolicy{
				APIVersion: "apisix.apache.org/v1alpha1",
				Kind:       "BackendTrafficPolicy",
				Metadata: apisix.Metadata{
					Name:      name,
					Namespace: ns,
					Labels: map[string]string{
						"managed-by":   "ingress2apisix",
						"ingress-name": ing.Metadata.Name,
					},
				},
				Spec: apisix.BackendTrafficPolicySpec{
					TargetRefs: buildPolicyTargetRefs(services),
					LoadBalancer: apisix.BackendLoadBalancer{
						Type:   "chash",
						HashOn: "cookie",
						Key:    cookieName,
					},
				},
				SourceAnnotations: sources,
			})
		}
	}

	// --- upstream-hash-by → chash with header or arg ---
	if hashBy, ok := getAnnotation(anns, "upstream-hash-by"); ok && hashBy != "" {
		services := collectBackendServiceNames(ing)
		if len(services) == 0 {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] upstream-hash-by 已识别，但未找到可关联的 Service，无法生成 BackendTrafficPolicy",
					ing.Metadata.Namespace, ing.Metadata.Name))
		} else {
			hashOn, key := parseUpstreamHashBy(hashBy)
			name := truncateName(sanitizeK8sName(fmt.Sprintf("%s-hash-by", ing.Metadata.Name)), 63)
			policies = append(policies, apisix.BackendTrafficPolicy{
				APIVersion: "apisix.apache.org/v1alpha1",
				Kind:       "BackendTrafficPolicy",
				Metadata: apisix.Metadata{
					Name:      name,
					Namespace: ns,
					Labels: map[string]string{
						"managed-by":   "ingress2apisix",
						"ingress-name": ing.Metadata.Name,
					},
				},
				Spec: apisix.BackendTrafficPolicySpec{
					TargetRefs: buildPolicyTargetRefs(services),
					LoadBalancer: apisix.BackendLoadBalancer{
						Type:   "chash",
						HashOn: hashOn,
						Key:    key,
					},
				},
				SourceAnnotations: map[string]string{
					"loadbalancer": srcAnnot(anns, "upstream-hash-by"),
				},
			})
		}
	}

	if len(policies) == 0 {
		return nil, warnings
	}
	return policies, warnings
}

// buildApisixUpstream creates an ApisixUpstream CRD for annotations that map to
// upstream-level configuration (health checks).
// Note: keepalive pool config is not supported by AIC's ApisixUpstream CRD schema;
// those annotations produce a warning and are skipped.
func (c *Converter) buildApisixUpstream(ing ingress.Ingress, ns string) (*apisix.ApisixUpstream, []string) {
	anns := ing.Metadata.Annotations
	if anns == nil {
		return nil, nil
	}

	var cfg apisix.ApisixUpstreamConfig
	hasConfig := false
	var warnings []string
	sources := make(map[string]string)

	// --- Health check annotations → ApisixUpstream healthCheck ---
	if hc, hcSources, hcWarns := c.buildHealthCheck(anns, ing, ns); hc != nil || len(hcWarns) > 0 {
		warnings = append(warnings, hcWarns...)
		if hc != nil {
			cfg.HealthCheck = hc
			hasConfig = true
			for k, v := range hcSources {
				sources[k] = v
			}
		}
	}

	// --- upstream-keepalive-* → warn (not supported by AIC ApisixUpstream CRD) ---
	if hasAnnotation(anns, "upstream-keepalive-connections") ||
		hasAnnotation(anns, "upstream-keepalive-requests") ||
		hasAnnotation(anns, "upstream-keepalive-timeout") {
		warnings = append(warnings,
			fmt.Sprintf("[%s/%s] upstream-keepalive-* 已识别，但 AIC 的 ApisixUpstream CRD 不支持 keepalive 字段；需在 APISIX 全局配置或 apisix_upstream 中手动设置",
				ing.Metadata.Namespace, ing.Metadata.Name))
	}

	if !hasConfig {
		return nil, warnings
	}

	name := truncateName(sanitizeK8sName(fmt.Sprintf("%s-upstream", ing.Metadata.Name)), 63)
	return &apisix.ApisixUpstream{
		APIVersion: c.opts.ApisixVersion,
		Kind:       "ApisixUpstream",
		Metadata: apisix.Metadata{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"managed-by":   "ingress2apisix",
				"ingress-name": ing.Metadata.Name,
			},
		},
		Spec: apisix.ApisixUpstreamSpec{
			IngressClassName:     c.opts.TargetIngressClassName,
			ApisixUpstreamConfig: cfg,
		},
		SourceAnnotations: sources,
	}, warnings
}

// buildApisixTls creates ApisixTls CRDs from Ingress spec.tls sections.
// Each TLS entry with a secretName produces one ApisixTls CRD.
func (c *Converter) buildApisixTls(ing ingress.Ingress, ns string) []apisix.ApisixTls {
	if len(ing.Spec.TLS) == 0 {
		return nil
	}

	var result []apisix.ApisixTls
	for i, tls := range ing.Spec.TLS {
		if tls.SecretName == "" || len(tls.Hosts) == 0 {
			continue
		}
		suffix := ""
		if i > 0 {
			suffix = fmt.Sprintf("-%d", i)
		}
		name := truncateName(sanitizeK8sName(fmt.Sprintf("%s-tls%s", ing.Metadata.Name, suffix)), 63)
		atls := apisix.ApisixTls{
			APIVersion: c.opts.ApisixVersion,
			Kind:       "ApisixTls",
			Metadata: apisix.Metadata{
				Name:      name,
				Namespace: ns,
				Labels: map[string]string{
					"managed-by":   "ingress2apisix",
					"ingress-name": ing.Metadata.Name,
				},
			},
			Spec: apisix.ApisixTlsSpec{
				IngressClassName: c.opts.TargetIngressClassName,
				Hosts:            tls.Hosts,
				Secret: apisix.ApisixSecret{
					Name:      tls.SecretName,
					Namespace: ns,
				},
			},
			SourceAnnotations: map[string]string{
				"hosts":  fmt.Sprintf("spec.tls[%d].hosts", i),
				"secret": fmt.Sprintf("spec.tls[%d].secretName", i),
			},
		}
		result = append(result, atls)
	}
	return result
}

func collectBackendServiceNames(ing ingress.Ingress) []string {
	seen := make(map[string]bool)
	var services []string
	add := func(backend ingress.IngressBackend) {
		if backend.Service == nil || backend.Service.Name == "" || seen[backend.Service.Name] {
			return
		}
		seen[backend.Service.Name] = true
		services = append(services, backend.Service.Name)
	}

	if ing.Spec.DefaultBackend != nil {
		add(*ing.Spec.DefaultBackend)
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			add(path.Backend)
		}
	}

	sort.Strings(services)
	return services
}

func buildPolicyTargetRefs(services []string) []apisix.PolicyTargetRef {
	refs := make([]apisix.PolicyTargetRef, 0, len(services))
	for _, svc := range services {
		refs = append(refs, apisix.PolicyTargetRef{
			Group: "",
			Kind:  "Service",
			Name:  svc,
		})
	}
	return refs
}

func sanitizeK8sName(name string) string {
	name = strings.ToLower(name)
	re := regexp.MustCompile(`[^a-z0-9-]+`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "cookie-affinity"
	}
	return name
}

func truncateName(name string, max int) string {
	if len(name) <= max {
		return name
	}
	return strings.Trim(name[:max], "-")
}

// applyLimitMultiplier multiplies a rate limit value by the limit-multiplier annotation if present.
func applyLimitMultiplier(baseRate string, anns map[string]string) string {
	multiplierStr, ok := getAnnotation(anns, "limit-multiplier")
	if !ok || multiplierStr == "" {
		return baseRate
	}
	multiplier, err := strconv.ParseFloat(strings.TrimSpace(multiplierStr), 64)
	if err != nil || multiplier <= 0 {
		return baseRate
	}
	base, err := strconv.ParseFloat(strings.TrimSpace(baseRate), 64)
	if err != nil {
		return baseRate
	}
	result := base * multiplier
	// Use integer if result has no fractional part
	if result == float64(int(result)) {
		return strconv.Itoa(int(result))
	}
	return strconv.FormatFloat(result, 'f', -1, 64)
}

// parseUpstreamHashBy parses nginx $variable syntax into APISIX chash hashOn/key.
// Supports: $remote_addr → header, $arg_xxx → query, $http_xxx → header, $request_uri → header
func parseUpstreamHashBy(hashBy string) (hashOn, key string) {
	h := strings.TrimSpace(hashBy)
	if !strings.HasPrefix(h, "$") {
		return "header", h
	}
	varName := strings.TrimPrefix(h, "$")

	switch {
	case strings.HasPrefix(varName, "arg_"):
		// $arg_xxx → APISIX vars (query parameter)
		return "vars", varName
	case strings.HasPrefix(varName, "http_"):
		// $http_xxx → header
		return "header", strings.TrimPrefix(varName, "http_")
	case varName == "remote_addr":
		return "vars", "remote_addr"
	case varName == "request_uri":
		return "vars", "request_uri"
	default:
		// Generic: use as APISIX variable
		return "vars", varName
	}
}

// buildHealthCheck constructs a HealthCheck (ApisixUpstream-compatible) from nginx health-check-* annotations.
func (c *Converter) buildHealthCheck(anns map[string]string, ing ingress.Ingress, ns string) (*apisix.HealthCheck, map[string]string, []string) {
	hasHC := hasAnnotation(anns, "health-check-path") ||
		hasAnnotation(anns, "health-check-interval") ||
		hasAnnotation(anns, "health-check-timeout") ||
		hasAnnotation(anns, "health-check-retries")
	if !hasHC {
		return nil, nil, nil
	}

	var warnings []string
	sources := make(map[string]string)
	hcPath, _ := getAnnotation(anns, "health-check-path")
	if hcPath == "" {
		hcPath = "/"
		warnings = append(warnings,
			fmt.Sprintf("[%s/%s] health-check-interval/retries/timeout 已设置但未指定 health-check-path，默认使用 /",
				ing.Metadata.Namespace, ing.Metadata.Name))
		sources["healthCheck.active.httpPath"] = "DEFAULT:" + srcAnnot(anns, "health-check-path")
	} else {
		sources["healthCheck.active.httpPath"] = srcAnnot(anns, "health-check-path")
	}

	active := &apisix.ActiveHealthCheck{
		Type:     "http",
		HTTPPath: hcPath,
	}
	sources["healthCheck.active.type"] = "DEFAULT_HINT:hardcoded to http"

	// Parse interval (AIC expects duration string like "10s", placed in Healthy/Unhealthy Interval)
	intervalStr := ""
	if v, ok := getAnnotation(anns, "health-check-interval"); ok && v != "" {
		intervalStr = ensureTimeSuffix(strings.TrimSpace(v))
		sources["healthCheck.active.healthy.interval"] = srcAnnot(anns, "health-check-interval")
	}
	// Parse timeout (AIC expects duration string like "5s")
	if v, ok := getAnnotation(anns, "health-check-timeout"); ok && v != "" {
		active.Timeout = ensureTimeSuffix(strings.TrimSpace(v))
		sources["healthCheck.active.timeout"] = srcAnnot(anns, "health-check-timeout")
	}
	if v, ok := getAnnotation(anns, "health-check-retries"); ok && v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			active.Healthy = &apisix.ActiveHealthCheckHealthy{
				Successes: n,
				Interval:  intervalStr,
			}
			sources["healthCheck.active.healthy.successes"] = srcAnnot(anns, "health-check-retries")
		}
	}
	active.Unhealthy = &apisix.ActiveHealthCheckUnhealthy{
		HTTPCodes:   []int{500, 502, 503, 504},
		TCPFailures: 3,
		Timeouts:    3,
		Interval:    intervalStr,
	}
	sources["healthCheck.active.unhealthy.httpCodes"] = "DEFAULT_HINT:hardcoded, same as nginx default"
	sources["healthCheck.active.unhealthy.tcpFailures"] = "DEFAULT_HINT:hardcoded default"

	return &apisix.HealthCheck{Active: active}, sources, warnings
}

// fixPathTypes adjusts pathType values and sets the use-regex annotation
// according to migration doc section 3.2:
//   - No regex + ImplementationSpecific → Prefix
//   - Regex + Prefix → ImplementationSpecific + use-regex: "true"
//   - Regex + ImplementationSpecific → keep + use-regex: "true"
//   - No regex + Exact → keep (no annotation)
func (c *Converter) fixPathTypes(ing *ingress.Ingress, annotations map[string]string) {
	hasRegex := false
	for ri := range ing.Spec.Rules {
		rule := &ing.Spec.Rules[ri]
		if rule.HTTP == nil {
			continue
		}
		for pi := range rule.HTTP.Paths {
			p := &rule.HTTP.Paths[pi]
			if p.PathType == nil {
				continue
			}
			if pathHasRegex(p.Path) {
				hasRegex = true
				if *p.PathType == "Prefix" {
					is := "ImplementationSpecific"
					p.PathType = &is
				}
				// ImplementationSpecific + regex → stays as-is
			} else {
				if *p.PathType == "ImplementationSpecific" {
					prefix := "Prefix"
					p.PathType = &prefix
				}
			}
		}
	}
	if hasRegex {
		annotations["k8s.apisix.apache.org/use-regex"] = "true"
	}
}

// extractRewriteURIs extracts rewrite patterns from nginx configuration-snippet.
func (c *Converter) extractRewriteURIs(snippet string) []string {
	return extractRewriteURIs(snippet)
}

func extractRewriteURIs(snippet string) []string {
	matches := rewriteDirectivePattern.FindAllStringSubmatch(snippet, -1)
	if len(matches) == 0 {
		return nil
	}
	var uris []string
	for _, m := range matches {
		if len(m) >= 3 {
			uris = append(uris, m[1], m[2])
		}
	}
	return uris
}

func (c *Converter) buildProxyRewriteURIs(ing ingress.Ingress) []string {
	anns := ing.Metadata.Annotations
	if anns == nil {
		return nil
	}

	var uris []string
	if target, ok := getAnnotation(anns, "rewrite-target"); ok && hasCombinedRewrite(anns) {
		uris = append(uris, rewriteTargetURIs(ing, target)...)
	}

	if snippet, ok := getAnnotation(anns, "configuration-snippet"); ok {
		uris = append(uris, c.extractRewriteURIs(snippet)...)
	}

	return uris
}

func rewriteTargetURIs(ing ingress.Ingress, target string) []string {
	var uris []string
	seen := make(map[string]bool)
	addPath := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		uris = append(uris, "(?i)"+path, target)
	}

	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, p := range rule.HTTP.Paths {
			addPath(p.Path)
		}
	}
	return uris
}

// ensureTimeSuffix ensures a timeout value has a time unit suffix.
func ensureTimeSuffix(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	last := v[len(v)-1]
	if last < '0' || last > '9' {
		return v
	}
	return v + "s"
}

// parseProxyCookieFlags parses proxy_cookie_flags directives from a
// configuration-snippet into rules suitable for the proxy-cookie-flags Lua plugin.
//
// nginx syntax: proxy_cookie_flags <pattern> [flag1] [flag2] ...;
// Example: proxy_cookie_flags sessionid SameSite=None Secure;
func parseProxyCookieFlags(snippet string) []map[string]interface{} {
	re := regexp.MustCompile(`proxy_cookie_flags\s+([^;]+);`)
	matches := re.FindAllStringSubmatch(snippet, -1)
	if len(matches) == 0 {
		return nil
	}

	var rules []map[string]interface{}
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(m[1]))
		if len(fields) < 2 {
			continue // need at least pattern + 1 flag
		}
		pattern := fields[0]
		flags := fields[1:]
		rules = append(rules, map[string]interface{}{
			"match": pattern,
			"flags": flags,
		})
	}
	return rules
}

// parseProxyCookiePath parses the ingress.kubernetes.io/proxy-cookie-path
// annotation value into path_pairs for the proxy-cookie-path plugin.
//
// Nginx proxy_cookie_path directive format: match replacement
// Examples:
//
//	~(.*) "$1"          → {match: "~(.*)", replacement: "$1"}
//	/path1 /path2       → {match: "/path1", replacement: "/path2"}
//
// When the annotation is present but empty, this also considers the case
// where the annotation is set to the empty string (which nginx ignores).
func parseProxyCookiePath(value string) []map[string]interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	// Try regex pattern first: ~<regex> <replacement>
	// The regex may contain spaces, so we look for the first unquoted space
	// after the regex to split.
	if len(value) > 1 && value[0] == '~' {
		return parseRegexCookiePath(value)
	}

	// Simple string match: /old-path /new-path
	parts := strings.SplitN(value, " ", 2)
	if len(parts) == 2 {
		match := strings.TrimSpace(parts[0])
		replacement := strings.TrimSpace(parts[1])
		if match != "" {
			// Remove surrounding quotes if present
			replacement = unquotePath(replacement)
			return []map[string]interface{}{
				{"match": match, "replacement": replacement},
			}
		}
	}

	return nil
}

// parseRegexCookiePath handles regex-style proxy-cookie-path values.
// Format: ~regex replacement (replacement may be quoted)
func parseRegexCookiePath(value string) []map[string]interface{} {
	// Find the regex pattern: everything after ~ up to the first space
	// that is NOT inside the regex. Regex can contain spaces if escaped,
	// but typically the nginx convention is no spaces in the regex itself.
	// We look for the pattern: ~<regex> <replacement>
	// where replacement may be quoted.
	//
	// Examples:
	//   ~(.*) "$1"
	//   ~^/api/(.*) /$1

	// Strip the leading ~
	rest := value[1:]

	// Find the first space that separates regex from replacement
	// We need to be careful: the regex itself might have spaces if it
	// uses character classes, but in practice proxy-cookie-path patterns
	// are simple and don't contain unescaped spaces.
	// Strategy: split on the last unquoted space, since the regex won't
	// have a trailing space.
	spaceIdx := strings.LastIndex(rest, " ")
	if spaceIdx <= 0 {
		return nil
	}

	regex := rest[:spaceIdx]
	replacement := rest[spaceIdx+1:]

	if regex == "" {
		return nil
	}

	replacement = unquotePath(replacement)

	return []map[string]interface{}{
		{"match": "~" + regex, "replacement": replacement},
	}
}

// unquotePath removes surrounding double quotes from a path string,
// as nginx uses quotes around replacements with special chars.
func unquotePath(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseBodySize parses a nginx proxy-body-size value (e.g. "10m", "1g", "1k", "0")
// into bytes. Suffixes: k/K = 1024, m/M = 1024*1024, g/G = 1024*1024*1024.
func parseBodySize(val string) (int64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, fmt.Errorf("empty value")
	}

	suffix := val[len(val)-1]
	switch {
	case suffix == 'k' || suffix == 'K':
		n, err := strconv.ParseInt(val[:len(val)-1], 10, 64)
		if err != nil {
			return 0, err
		}
		return n * 1024, nil
	case suffix == 'm' || suffix == 'M':
		n, err := strconv.ParseInt(val[:len(val)-1], 10, 64)
		if err != nil {
			return 0, err
		}
		return n * 1024 * 1024, nil
	case suffix == 'g' || suffix == 'G':
		n, err := strconv.ParseInt(val[:len(val)-1], 10, 64)
		if err != nil {
			return 0, err
		}
		return n * 1024 * 1024 * 1024, nil
	default:
		return strconv.ParseInt(val, 10, 64)
	}
}
