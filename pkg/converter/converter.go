package converter

import (
	"fmt"
	"regexp"
	"sort"
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
	"auth-response-headers":  true,
	"websocket-services":     true,
	"use-regex":              true,
	// Auth via native APISIX annotation
	"auth-type":  true,
	"auth-realm": true,
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
}

// knownManualAnnotations are recognized but require manual intervention.
// The value is the recommended migration approach.
var knownManualAnnotations = map[string]string{
	"custom-http-errors":     "需自定义 Lua 插件 (custom-error-page)，参见迁移文档 4.1.3",
	"affinity":               "需 BackendTrafficPolicy CRD 实现会话亲和，参见迁移文档 4.1.1",
	"session-cookie-name":    "需 BackendTrafficPolicy CRD 实现会话亲和，参见迁移文档 4.1.1",
	"upstream-hash-by":       "APISIX Ingress 没有等价原生注解；如需基于 cookie/header 做上游一致性哈希，请使用 BackendTrafficPolicy 或 ApisixRoute/ApisixUpstream 能力",
	"auth-tls-secret":        "需 ApisixTls CRD 实现 mTLS，参见迁移文档 4.1.6",
	"auth-tls-verify-client": "需 ApisixTls CRD 实现 mTLS，参见迁移文档 4.1.6",
	"auth-secret":            "需 ApisixConsumer CRD 配合 auth-type 注解，参见迁移文档 4.1.8",
	"proxy-body-size":        "需全局配置 nginx_config.http.client_max_body_size，参见迁移文档 3.1",
	"proxy-buffer-size":      "需全局配置 nginx_config.http_configuration_snippet.proxy_buffer_size，参见迁移文档 3.1",
	"proxy-buffers-number":   "需全局配置 nginx_config.http_configuration_snippet.proxy_buffers，参见迁移文档 3.1",
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

	// Convert nginx annotations to APISIX annotations
	c.convertAnnotations(ing.Metadata.Annotations, newAnnotations)

	// Default SSL redirect when TLS is configured and option is enabled
	if c.opts.SSLRedirect && len(ing.Spec.TLS) > 0 {
		if _, exists := newAnnotations["k8s.apisix.apache.org/http-to-https"]; !exists {
			newAnnotations["k8s.apisix.apache.org/http-to-https"] = "true"
		}
	}

	out.Metadata.Annotations = newAnnotations

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

// convertAnnotations maps nginx/ingress annotations to APISIX annotations.
func (c *Converter) convertAnnotations(src, out map[string]string) {
	if src == nil {
		return
	}

	// --- CORS ---
	if v, ok := getAnnotation(src, "enable-cors"); ok && v == "true" {
		out["k8s.apisix.apache.org/enable-cors"] = "true"
		out["k8s.apisix.apache.org/cors-allow-origin"] = "*"
		out["k8s.apisix.apache.org/cors-allow-methods"] = "GET,POST,PUT,DELETE,PATCH,OPTIONS"
		out["k8s.apisix.apache.org/cors-allow-headers"] = "*"
	}
	if v, ok := getAnnotation(src, "cors-allow-origin"); ok {
		out["k8s.apisix.apache.org/cors-allow-origin"] = v
	}
	if v, ok := getAnnotation(src, "cors-allow-methods"); ok {
		out["k8s.apisix.apache.org/cors-allow-methods"] = v
	}
	if v, ok := getAnnotation(src, "cors-allow-headers"); ok {
		out["k8s.apisix.apache.org/cors-allow-headers"] = v
	}

	// --- SSL redirect → http-to-https ---
	if v, ok := getAnnotation(src, "ssl-redirect"); ok && v == "true" {
		out["k8s.apisix.apache.org/http-to-https"] = "true"
	}
	if hasAnnotation(src, "force-ssl-redirect") {
		out["k8s.apisix.apache.org/http-to-https"] = "true"
	}

	// --- Proxy redirect: http:// → https:// maps to http-to-https annotation ---
	if from, ok := getAnnotation(src, "proxy-redirect-from"); ok {
		if to, ok := getAnnotation(src, "proxy-redirect-to"); ok {
			if isSSLRedirect(from, to) {
				out["k8s.apisix.apache.org/http-to-https"] = "true"
			}
		}
	}

	combinedRewrite := hasCombinedRewrite(src)

	// --- Rewrite target ---
	if v, ok := getAnnotation(src, "rewrite-target"); ok && !combinedRewrite {
		if regexCapturePattern.MatchString(v) {
			out["k8s.apisix.apache.org/rewrite-target-regex"] = v
		} else {
			out["k8s.apisix.apache.org/rewrite-target"] = v
		}
	}

	// --- Proxy timeouts → upstream timeouts with 's' suffix ---
	if v, ok := getAnnotation(src, "proxy-connect-timeout"); ok {
		out["k8s.apisix.apache.org/upstream-connect-timeout"] = ensureTimeSuffix(v)
	}
	if v, ok := getAnnotation(src, "proxy-send-timeout"); ok {
		out["k8s.apisix.apache.org/upstream-send-timeout"] = ensureTimeSuffix(v)
	}
	if v, ok := getAnnotation(src, "proxy-read-timeout"); ok {
		out["k8s.apisix.apache.org/upstream-read-timeout"] = ensureTimeSuffix(v)
	}

	// --- Backend protocol → upstream-scheme ---
	if v, ok := getAnnotation(src, "backend-protocol"); ok {
		out["k8s.apisix.apache.org/upstream-scheme"] = strings.ToLower(v)
	}

	// --- Whitelist → allowlist-source-range ---
	if v, ok := getAnnotation(src, "whitelist-source-range"); ok {
		out["k8s.apisix.apache.org/allowlist-source-range"] = v
	}

	// --- External auth: auth-url → auth-uri ---
	if v, ok := getAnnotation(src, "auth-url"); ok {
		out["k8s.apisix.apache.org/auth-uri"] = v
		if _, exists := out["k8s.apisix.apache.org/auth-request-headers"]; !exists {
			out["k8s.apisix.apache.org/auth-request-headers"] = "User-Agent,cookie"
		}
	}
	if v, ok := getAnnotation(src, "auth-response-headers"); ok {
		out["k8s.apisix.apache.org/auth-upstream-headers"] = v
	}

	// --- Auth type: basic/key → native APISIX auth-type annotation ---
	if v, ok := getAnnotation(src, "auth-type"); ok {
		switch strings.ToLower(v) {
		case "basic":
			out["k8s.apisix.apache.org/auth-type"] = "basicAuth"
		case "digest":
			out["k8s.apisix.apache.org/auth-type"] = "keyAuth"
		default:
			out["k8s.apisix.apache.org/auth-type"] = v
		}
	}

	// --- WebSocket ---
	if v, ok := getAnnotation(src, "websocket-services"); ok && v != "" {
		out["k8s.apisix.apache.org/enable-websocket"] = "true"
	}

	// --- use-regex ---
	if v, ok := getAnnotation(src, "use-regex"); ok && v == "true" {
		out["k8s.apisix.apache.org/use-regex"] = "true"
	}

	// --- configuration-snippet: single rewrite → native rewrite-target-regex annotations ---
	if snippet, ok := getAnnotation(src, "configuration-snippet"); ok && !combinedRewrite {
		rewriteURIs := c.extractRewriteURIs(snippet)
		if len(rewriteURIs) == 2 {
			// Single rewrite directive → use native APISIX annotations
			out["k8s.apisix.apache.org/rewrite-target-regex"] = rewriteURIs[0]
			out["k8s.apisix.apache.org/rewrite-target-regex-template"] = rewriteURIs[1]
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
		plugins = append(plugins, apisix.Plugin{
			Name:   "limit-req",
			Enable: true,
			Config: map[string]interface{}{
				"rate":          v,
				"burst":         0,
				"key":           "remote_addr",
				"rejected_code": rejectedCode,
			},
		})
	} else if v, ok := getAnnotation(anns, "limit-rpm"); ok && v != "" {
		rejectedCode := "429"
		plugins = append(plugins, apisix.Plugin{
			Name:   "limit-req",
			Enable: true,
			Config: map[string]interface{}{
				"rate":          v + "/min",
				"burst":         0,
				"key":           "remote_addr",
				"rejected_code": rejectedCode,
			},
		})
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
	}

	// --- Auth-secret warning (needs ApisixConsumer CRD) ---
	if _, ok := getAnnotation(anns, "auth-secret"); ok {
		if v, ok := getAnnotation(anns, "auth-type"); ok && strings.ToLower(v) == "basic" {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] auth-secret 需配合 ApisixConsumer CRD 使用，参见迁移文档 4.1.8",
					ing.Metadata.Namespace, ing.Metadata.Name))
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
		}
	}

	// --- Session cookie hash → session-cookie-hash custom plugin ---
	hashAlgo, hasHashAlgo := getAnnotation(anns, "session-cookie-hash")
	if !hasHashAlgo && hasCookieAffinity(anns) {
		hashAlgo = "sha1"
		hasHashAlgo = true
		warnings = append(warnings,
			fmt.Sprintf("[%s/%s] affinity=cookie 未配置 session-cookie-hash，默认使用 sha1 生成 session-cookie-hash 插件；如需其他算法请显式配置 sha1/md5/sha256",
				ing.Metadata.Namespace, ing.Metadata.Name))
	}
	if hasHashAlgo {
		algo := strings.ToLower(strings.TrimSpace(hashAlgo))
		if supportedSessionCookieHash(algo) {
			if !hasAnnotation(anns, "session-cookie-name") {
				warnings = append(warnings,
					fmt.Sprintf("[%s/%s] 未配置 session-cookie-name，session-cookie-hash 插件将使用默认 cookie_name=INGRESSCOOKIE；请确认与实际会话 Cookie 一致。注意：APISIX Ingress 没有 upstream-hash 注解，会话亲和仍需 BackendTrafficPolicy 等资源承载",
						ing.Metadata.Namespace, ing.Metadata.Name))
			}
			plugins = append(plugins, apisix.Plugin{
				Name:   "session-cookie-hash",
				Enable: true,
				Config: map[string]interface{}{
					"cookie_name":     sessionCookieName(anns),
					"algorithm":       algo,
					"header_name":     "X-Session-Hash",
					"fallback":        "pass",
					"generate_cookie": true,
					"cookie_httponly": false,
				},
			})
		} else {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] session-cookie-hash=%q 不支持自动转换，仅支持 sha1/md5/sha256；请改用支持值并配合 session-cookie-name。注意：APISIX Ingress 没有 upstream-hash 注解，会话亲和仍需 BackendTrafficPolicy 等资源承载",
					ing.Metadata.Namespace, ing.Metadata.Name, hashAlgo))
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
			Plugins: plugins,
		},
	}

	// Sort warnings for deterministic output
	sort.Strings(warnings)
	return pc, warnings
}

func (c *Converter) buildBackendTrafficPolicies(ing ingress.Ingress, ns string) ([]apisix.BackendTrafficPolicy, []string) {
	anns := ing.Metadata.Annotations
	if !hasCookieAffinity(anns) {
		return nil, nil
	}

	services := collectBackendServiceNames(ing)
	if len(services) == 0 {
		return nil, []string{
			fmt.Sprintf("[%s/%s] affinity=cookie 已识别，但未找到可关联的 Service，无法生成 BackendTrafficPolicy",
				ing.Metadata.Namespace, ing.Metadata.Name),
		}
	}

	var warnings []string
	cookieName := sessionCookieName(anns)
	if cookieName == "INGRESSCOOKIE" && !hasAnnotation(anns, "session-cookie-name") {
		warnings = append(warnings,
			fmt.Sprintf("[%s/%s] affinity=cookie 未配置 session-cookie-name，BackendTrafficPolicy 将使用默认 key=INGRESSCOOKIE；请确认客户端实际会携带该 Cookie",
				ing.Metadata.Namespace, ing.Metadata.Name))
	}

	name := truncateName(sanitizeK8sName(fmt.Sprintf("%s-cookie-affinity", ing.Metadata.Name)), 63)
	return []apisix.BackendTrafficPolicy{
		{
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
		},
	}, warnings
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
