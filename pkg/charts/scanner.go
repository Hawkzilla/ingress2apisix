package charts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ingress2apisix/pkg/converter"
)

// AnnotationStatus describes the migration status of an annotation.
type AnnotationStatus int

const (
	// StatusConverted means the annotation is automatically converted to APISIX native annotation.
	StatusConverted AnnotationStatus = iota
	// StatusPluginConfig means the annotation produces an ApisixPluginConfig.
	StatusPluginConfig
	// StatusCustomPlugin means the annotation is handled by a custom APISIX Lua plugin.
	StatusCustomPlugin
	// StatusManual means the annotation requires manual intervention.
	StatusManual
	// StatusUnknown means the annotation is not recognized.
	StatusUnknown
)

func (s AnnotationStatus) String() string {
	switch s {
	case StatusConverted:
		return "CONVERTED"
	case StatusPluginConfig:
		return "PLUGIN_CONFIG"
	case StatusCustomPlugin:
		return "CUSTOM_PLUGIN"
	case StatusManual:
		return "MANUAL"
	case StatusUnknown:
		return "UNKNOWN"
	default:
		return "UNKNOWN"
	}
}

// AnnotationFinding represents a single annotation found in a chart file.
type AnnotationFinding struct {
	Annotation string           // Full annotation key
	Value      string           // Annotation value (may be empty for Helm templates)
	Status     AnnotationStatus // Migration status
	Detail     string           // Migration guidance or APISIX equivalent
}

// FileReport is the check result for a single file.
type FileReport struct {
	Path       string              // Relative path to the file
	IsIngress  bool                // Whether the file contains Ingress resources
	Findings   []AnnotationFinding // Annotations found
	HasHelmTpl bool                // Whether the file contains Helm Go template syntax
}

// CheckReport is the overall result of checking a charts directory.
type CheckReport struct {
	Files        []FileReport
	TotalFiles   int
	IngressFiles int
	Converted    int
	PluginConfig int
	CustomPlugin int
	Manual       int
	Unknown      int
}

// annotation regex patterns for both prefix types
var (
	reNginxAnnotation = regexp.MustCompile(`nginx\.ingress\.kubernetes\.io/([\w-]+)`)
	rePlainAnnotation = regexp.MustCompile(`ingress\.kubernetes\.io/([\w-]+)`)
	reIngressKind     = regexp.MustCompile(`(?i)kind:\s*["']?Ingress["']?`)
	reHelmTemplate    = regexp.MustCompile(`\{\{[^}]*\}\}`)
	reYAMLFile        = regexp.MustCompile(`\.(ya?ml)(\.tpl)?$`)
)

// CheckChartsDir walks a directory and checks all Ingress YAML/template files
// for nginx annotations, producing a migration report.
func CheckChartsDir(dir string) (*CheckReport, error) {
	report := &CheckReport{}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable files
		}
		if info.IsDir() {
			return nil
		}
		if !reYAMLFile.MatchString(info.Name()) {
			return nil
		}

		relPath, _ := filepath.Rel(dir, path)
		fr, err := checkFile(path, relPath)
		if err != nil {
			return nil // skip files that can't be read
		}
		if fr.IsIngress {
			report.Files = append(report.Files, *fr)
			report.IngressFiles++
		}
		report.TotalFiles++
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Aggregate counts
	for _, f := range report.Files {
		for _, finding := range f.Findings {
			switch finding.Status {
			case StatusConverted:
				report.Converted++
			case StatusPluginConfig:
				report.PluginConfig++
			case StatusCustomPlugin:
				report.CustomPlugin++
			case StatusManual:
				report.Manual++
			case StatusUnknown:
				report.Unknown++
			}
		}
	}

	return report, nil
}

// checkFile examines a single file for Ingress annotations.
func checkFile(path, relPath string) (*FileReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)

	fr := &FileReport{
		Path:       relPath,
		IsIngress:  reIngressKind.MatchString(content),
		HasHelmTpl: reHelmTemplate.MatchString(content),
	}

	if !fr.IsIngress {
		return fr, nil
	}

	// Extract annotations
	findings := extractAnnotations(content)

	// Enrich configuration-snippet findings with content analysis
	for i := range findings {
		if findings[i].Annotation != "configuration-snippet" &&
			findings[i].Annotation != "nginx.ingress.kubernetes.io/configuration-snippet" &&
			findings[i].Annotation != "ingress.kubernetes.io/configuration-snippet" {
			continue
		}
		// Try to extract snippet value from the file for deeper analysis
		snippetVal := extractSnippetValue(content)
		if snippetVal != "" {
			findings[i].Detail = analyzeSnippetContent(snippetVal)
		}
	}

	fr.Findings = findings

	return fr, nil
}

// extractSnippetValue extracts the multi-line value of configuration-snippet
// from the raw file content.
func extractSnippetValue(content string) string {
	// Match: configuration-snippet: | (or >) followed by indented lines
	re := regexp.MustCompile(`(?:nginx\.ingress\.kubernetes\.io/|ingress\.kubernetes\.io/)configuration-snippet:\s*[|>][\r\n]((?:(?:[ \t]+).*(?:[\r\n]|\$))+)`)
	m := re.FindStringSubmatch(content)
	if len(m) >= 2 {
		return m[1]
	}
	// Also try single-line quoted value
	re2 := regexp.MustCompile(`(?:nginx\.ingress\.kubernetes\.io/|ingress\.kubernetes\.io/)configuration-snippet:\s*["']?([^"'\n]+)`)
	m2 := re2.FindStringSubmatch(content)
	if len(m2) >= 2 {
		return m2[1]
	}
	return ""
}

// analyzeSnippetContent performs deeper analysis on a configuration-snippet
// and returns a detailed classification.
func analyzeSnippetContent(snippet string) string {
	var parts []string

	// Check for rewrite directives
	rewriteRe := regexp.MustCompile(`rewrite\s+\S+\s+\S+\s+(break|last|redirect|permanent)`)
	rewriteMatches := rewriteRe.FindAllString(snippet, -1)
	rewriteCount := len(rewriteMatches)
	if rewriteCount == 1 {
		parts = append(parts, "单条 rewrite → 可用原生注解 rewrite-target-regex + rewrite-target-regex-template")
	} else if rewriteCount > 1 {
		parts = append(parts, fmt.Sprintf("%d 条 rewrite → 需 ApisixPluginConfig + proxy-rewrite 插件", rewriteCount))
	}

	// Check for proxy_cookie_flags
	cookieRe := regexp.MustCompile(`proxy_cookie_flags\s+`)
	if cookieRe.MatchString(snippet) {
		parts = append(parts, "proxy_cookie_flags → 需自定义 Lua 插件 proxy-cookie-flags (需部署 plugins/proxy-cookie-flags.lua)")
	}

	// Check for more_set_headers
	moreHeadersRe := regexp.MustCompile(`more_set_headers\s+`)
	if moreHeadersRe.MatchString(snippet) {
		parts = append(parts, "more_set_headers → 需 ApisixPluginConfig + proxy-rewrite 插件 (headers.set)，参见迁移文档 4.1.5")
	}

	// Check for limit_req_status
	limitReqRe := regexp.MustCompile(`limit_req_status\s+(\d{3})`)
	if m := limitReqRe.FindStringSubmatch(snippet); len(m) > 0 {
		parts = append(parts, fmt.Sprintf("limit_req_status %s → 配合 limit-req 插件的 rejected_code 参数", m[1]))
	}

	// Check for add_header
	addHeaderRe := regexp.MustCompile(`add_header\s+`)
	if addHeaderRe.MatchString(snippet) {
		parts = append(parts, "add_header → 需 proxy-rewrite 或 response-rewrite 插件")
	}

	// Check for proxy_set_header
	proxySetHeaderRe := regexp.MustCompile(`proxy_set_header\s+`)
	if proxySetHeaderRe.MatchString(snippet) {
		parts = append(parts, "proxy_set_header → 需 ApisixPluginConfig + proxy-rewrite 插件 (headers.set)")
	}

	// Check for if directives (complex conditional logic)
	ifRe := regexp.MustCompile(`(?m)^\s*if\s+`)
	if ifRe.MatchString(snippet) {
		parts = append(parts, "条件逻辑 (if) → 需手动分析，可能需拆分路由或使用 APISIX 变量匹配")
	}

	if len(parts) == 0 {
		return "→ 单条 rewrite 用原生注解，多条用 ApisixPluginConfig + proxy-rewrite"
	}
	return strings.Join(parts, "; ")
}

// extractAnnotations scans file content for nginx/ingress annotations
// and classifies each one.
func extractAnnotations(content string) []AnnotationFinding {
	var findings []AnnotationFinding
	seen := make(map[string]bool)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// Skip pure comment lines
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Find nginx.ingress.kubernetes.io annotations
		for _, m := range reNginxAnnotation.FindAllStringSubmatch(line, -1) {
			if len(m) >= 2 {
				suffix := m[1]
				if !seen[suffix] {
					seen[suffix] = true
					finding := classifyAnnotation("nginx.ingress.kubernetes.io/"+suffix, suffix)
					findings = append(findings, finding)
				}
			}
		}

		// Find ingress.kubernetes.io annotations
		for _, m := range rePlainAnnotation.FindAllStringSubmatch(line, -1) {
			if len(m) >= 2 {
				suffix := m[1]
				if !seen[suffix] {
					seen[suffix] = true
					finding := classifyAnnotation("ingress.kubernetes.io/"+suffix, suffix)
					findings = append(findings, finding)
				}
			}
		}
	}

	// Sort for deterministic output
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].Annotation < findings[j].Annotation
	})

	return findings
}

// knownManualInfo mirrors converter.knownManualAnnotations for check reporting.
var knownManualInfo = map[string]string{
	"custom-http-errors":     "需自定义 Lua 插件 (custom-error-page)，参见迁移文档 4.1.3",
	"affinity":               "需 BackendTrafficPolicy CRD 实现会话亲和，参见迁移文档 4.1.1",
	"session-cookie-name":    "需 BackendTrafficPolicy CRD 实现会话亲和，参见迁移文档 4.1.1",
	"upstream-hash-by":       "APISIX Ingress 没有等价原生注解；如需上游一致性哈希，请使用 BackendTrafficPolicy 或 ApisixRoute/ApisixUpstream 能力",
	"auth-tls-secret":        "需 ApisixTls CRD 实现 mTLS，参见迁移文档 4.1.6",
	"auth-tls-verify-client": "需 ApisixTls CRD 实现 mTLS，参见迁移文档 4.1.6",
	"auth-secret":            "需 ApisixConsumer CRD 配合 auth-type 注解，参见迁移文档 4.1.8",
	"proxy-body-size":        "需全局配置 nginx_config.http.client_max_body_size，参见迁移文档 3.1",
	"proxy-buffer-size":      "需全局配置 nginx_config.http_configuration_snippet.proxy_buffer_size，参见迁移文档 3.1",
	"proxy-buffers-number":   "需全局配置 nginx_config.http_configuration_snippet.proxy_buffers，参见迁移文档 3.1",
}

// convertedMapping describes the APISIX equivalent for auto-converted annotations.
var convertedMapping = map[string]string{
	"enable-cors":            "→ k8s.apisix.apache.org/enable-cors",
	"cors-allow-origin":      "→ k8s.apisix.apache.org/cors-allow-origin",
	"cors-allow-methods":     "→ k8s.apisix.apache.org/cors-allow-methods",
	"cors-allow-headers":     "→ k8s.apisix.apache.org/cors-allow-headers",
	"ssl-redirect":           "→ k8s.apisix.apache.org/http-to-https",
	"force-ssl-redirect":     "→ k8s.apisix.apache.org/http-to-https",
	"rewrite-target":         "→ k8s.apisix.apache.org/rewrite-target 或 rewrite-target-regex",
	"proxy-connect-timeout":  "→ k8s.apisix.apache.org/upstream-connect-timeout",
	"proxy-send-timeout":     "→ k8s.apisix.apache.org/upstream-send-timeout",
	"proxy-read-timeout":     "→ k8s.apisix.apache.org/upstream-read-timeout",
	"backend-protocol":       "→ k8s.apisix.apache.org/upstream-scheme",
	"whitelist-source-range": "→ k8s.apisix.apache.org/allowlist-source-range",
	"auth-url":               "→ k8s.apisix.apache.org/auth-uri",
	"auth-response-headers":  "→ k8s.apisix.apache.org/auth-upstream-headers",
	"auth-type":              "→ k8s.apisix.apache.org/auth-type (basicAuth/keyAuth)",
	"auth-realm":             "→ k8s.apisix.apache.org/auth-realm",
	"websocket-services":     "→ k8s.apisix.apache.org/enable-websocket",
	"use-regex":              "→ k8s.apisix.apache.org/use-regex",
}

// knownManualExtraInfo adds more known annotations that require manual handling,
// supplementing the converter's knownManualAnnotations map.
var knownManualExtraInfo = map[string]string{
	// Canary deployments → APISIX has no direct equivalent; needs traffic-split plugin or ApisixRoute
	"canary":                 "需 ApisixRoute + traffic-split 插件或 header/cookie 条件路由",
	"canary-weight":          "需 ApisixRoute + traffic-split 插件实现权重分流",
	"canary-by-header":       "需 ApisixRoute 条件路由（基于 header）",
	"canary-by-header-value": "需配合 canary-by-header 使用",
	"canary-by-cookie":       "需 ApisixRoute 条件路由（基于 cookie）",
	// SSL/TLS 配置 → ApisixTls CRD 或全局配置
	"ssl-passthrough": "需 APISIX stream_proxy 配置 + ApisixTls CRD",
	"ssl-protocols":   "需全局配置 apisix.ssl.protocols",
	"ssl-ciphers":     "需全局配置 apisix.ssl.ciphers",
	// HSTS → 需 response-rewrite 插件或全局配置
	"hsts":                    "需 response-rewrite 插件或全局配置注入 Strict-Transport-Security 头",
	"hsts-max-age":            "需配合 hsts 使用",
	"hsts-include-subdomains": "需配合 hsts 使用",
	"hsts-preload":            "需配合 hsts 使用",
	// Server/Location snippet → 需分析内容确定迁移方案
	"server-snippet": "服务级别 NGINX 配置片段，需逐条分析迁移方案",
	// HTTP redirects
	"permanent-redirect": "→ k8s.apisix.apache.org/http-redirect + http-redirect-code: 308",
	"temporal-redirect":  "→ k8s.apisix.apache.org/http-redirect + http-redirect-code: 302",
	// App root redirect
	"app-root": "→ k8s.apisix.apache.org/http-redirect 重定向根路径",
	// Proxy HTTP version
	"proxy-http-version": "需全局配置上游 HTTP 版本",
	// Service upstream (use ClusterIP)
	"service-upstream": "需 APISIX 全局 DNS 解析模式配置",
	// Upstream keepalive
	"upstream-keepalive-connections": "需全局配置 apisix.upstream.keepalive",
	"upstream-keepalive-requests":    "需全局配置 apisix.upstream.keepalive_requests",
	"upstream-keepalive-timeout":     "需全局配置 apisix.upstream.keepalive_timeout",
	// Access log
	"disable-access-log": "需全局配置 nginx_config.http.access_log",
	// Denylist
	"denylist-source-range": "→ k8s.apisix.apache.org/blocklist-source-range",
	// Connection proxy header
	"connection-proxy-header": "需 proxy-rewrite 插件设置 Connection 头",
	// X-Forwarded-Prefix
	"x-forwarded-prefix": "需 proxy-rewrite 插件添加 X-Forwarded-Prefix 头",
	// Preserve trailing slash
	"preserve-trailing-slash": "需 proxy-rewrite 插件保留尾斜杠",
	// Retry
	"retry-non-idempotent": "需 APISIX 全局重试配置",
	// Satisfy (auth)
	"satisfy": "需 APISIX 多认证插件组合配置",
	// OpenTracing / InfluxDB
	"enable-opentracing": "需 APISIX 全局 OpenTracing 配置",
	"enable-influxdb":    "需 APISIX 全局 InfluxDB 配置",
	// Mirror
	"mirror-target": "需 APISIX plugin_attr.mirror 配置实现流量镜像",
	"mirror-path":   "需配合 mirror-target 使用",
	// ModSecurity / WAF
	"enable-owasp-modsecurity-crc": "需 APISIX waf 插件配置",
	"modsecurity-transaction-id":   "需配合 ModSecurity 使用",
	"modsecurity-snippet":          "需配合 ModSecurity 使用",
}

// classifyAnnotation determines the migration status of a single annotation suffix.
func classifyAnnotation(fullKey, suffix string) AnnotationFinding {
	f := AnnotationFinding{Annotation: fullKey}

	// Check handled → converted to native annotation
	if detail, ok := convertedMapping[suffix]; ok {
		f.Status = StatusConverted
		f.Detail = detail
		return f
	}

	// Check handled → PluginConfig
	switch suffix {
	case "limit-rps":
		f.Status = StatusPluginConfig
		f.Detail = "→ ApisixPluginConfig + limit-req 插件"
		return f
	case "limit-rpm":
		f.Status = StatusPluginConfig
		f.Detail = "→ ApisixPluginConfig + limit-req 插件 (rate: N/min)"
		return f
	case "limit-connections":
		f.Status = StatusPluginConfig
		f.Detail = "→ ApisixPluginConfig + limit-conn 插件"
		return f
	case "configuration-snippet":
		f.Status = StatusPluginConfig
		f.Detail = "→ 单条 rewrite 用原生注解，多条用 ApisixPluginConfig + proxy-rewrite"
		return f
	case "proxy-cookie-path":
		f.Status = StatusManual
		f.Detail = "→ 无等价 APISIX 插件，需手动评估 Cookie Path 改写需求"
		return f
	case "session-cookie-hash":
		f.Status = StatusCustomPlugin
		f.Detail = "→ ApisixPluginConfig + 自定义 session-cookie-hash 插件；会话亲和仍需 BackendTrafficPolicy 承载"
		return f
	case "session-cookie-name":
		f.Status = StatusCustomPlugin
		f.Detail = "→ 作为 session-cookie-hash 插件的 cookie_name 输入；affinity=cookie 仍需 BackendTrafficPolicy"
		return f
	case "proxy-redirect-from", "proxy-redirect-to":
		f.Status = StatusConverted
		f.Detail = "→ k8s.apisix.apache.org/http-to-https (仅 HTTP→HTTPS 场景可自动迁移，其他场景需手动处理)"
		return f
	}

	// Check known manual annotations (from converter)
	if hint, ok := knownManualInfo[suffix]; ok {
		f.Status = StatusManual
		f.Detail = hint
		return f
	}

	// Check additional known manual annotations
	if hint, ok := knownManualExtraInfo[suffix]; ok {
		// If it starts with "→ ", it's actually convertible
		if strings.HasPrefix(hint, "→ ") {
			f.Status = StatusConverted
			f.Detail = hint
			return f
		}
		f.Status = StatusManual
		f.Detail = hint
		return f
	}

	// Unknown
	f.Status = StatusUnknown
	f.Detail = "未识别的注解，请手动确认迁移方案"
	return f
}

// FormatCheckReport returns a human-readable report string.
func FormatCheckReport(report *CheckReport, verbose bool) string {
	var sb strings.Builder

	sb.WriteString("\n=== Ingress Annotation Migration Check ===\n\n")
	sb.WriteString(fmt.Sprintf("Scanned files:    %d\n", report.TotalFiles))
	sb.WriteString(fmt.Sprintf("Ingress files:    %d\n\n", report.IngressFiles))

	if report.IngressFiles == 0 {
		sb.WriteString("No Ingress resources found.\n")
		return sb.String()
	}

	// Summary
	sb.WriteString("Summary:\n")
	sb.WriteString(fmt.Sprintf("  CONVERTED      (auto): %d\n", report.Converted))
	sb.WriteString(fmt.Sprintf("  PLUGIN_CONFIG  (auto): %d\n", report.PluginConfig))
	sb.WriteString(fmt.Sprintf("  CUSTOM_PLUGIN  (auto): %d\n", report.CustomPlugin))
	sb.WriteString(fmt.Sprintf("  MANUAL     (requires): %d\n", report.Manual))
	sb.WriteString(fmt.Sprintf("  UNKNOWN    (check it): %d\n\n", report.Unknown))

	// Issues
	if report.Manual > 0 || report.Unknown > 0 {
		sb.WriteString("--- Issues requiring attention ---\n\n")
		for _, f := range report.Files {
			hasIssue := false
			for _, finding := range f.Findings {
				if finding.Status == StatusManual || finding.Status == StatusUnknown {
					if !hasIssue {
						sb.WriteString(fmt.Sprintf("  %s:\n", f.Path))
						if f.HasHelmTpl {
							sb.WriteString("    (Helm template - annotation values may contain Go template expressions)\n")
						}
						hasIssue = true
					}
					sb.WriteString(fmt.Sprintf("    [%s] %s\n", finding.Status, finding.Annotation))
					sb.WriteString(fmt.Sprintf("      %s\n", finding.Detail))
				}
			}
		}
	}

	// Verbose: show all annotations per file
	if verbose {
		sb.WriteString("\n--- Full annotation listing ---\n\n")
		for _, f := range report.Files {
			sb.WriteString(fmt.Sprintf("  %s:\n", f.Path))
			if f.HasHelmTpl {
				sb.WriteString("    (Helm template)\n")
			}
			if len(f.Findings) == 0 {
				sb.WriteString("    (no nginx/ingress annotations found)\n")
			}
			for _, finding := range f.Findings {
				sb.WriteString(fmt.Sprintf("    [%s] %s\n", finding.Status, finding.Annotation))
				sb.WriteString(fmt.Sprintf("      %s\n", finding.Detail))
			}
		}
	}

	return sb.String()
}

// FormatCheckReportMarkdown returns a markdown report (for writing to file).
func FormatCheckReportMarkdown(report *CheckReport) string {
	var sb strings.Builder

	sb.WriteString("# Ingress Annotation Migration Check\n\n")
	sb.WriteString(fmt.Sprintf("- Scanned files: %d\n", report.TotalFiles))
	sb.WriteString(fmt.Sprintf("- Ingress files: %d\n", report.IngressFiles))
	sb.WriteString(fmt.Sprintf("- Auto-converted: %d\n", report.Converted+report.PluginConfig+report.CustomPlugin))
	sb.WriteString(fmt.Sprintf("- Manual required: %d\n", report.Manual))
	sb.WriteString(fmt.Sprintf("- Unknown: %d\n\n", report.Unknown))

	if report.IngressFiles == 0 {
		sb.WriteString("No Ingress resources found.\n")
		return sb.String()
	}

	// Files with issues
	sb.WriteString("## Files with Issues\n\n")
	sb.WriteString("| File | Annotation | Status | Migration Guide |\n")
	sb.WriteString("|---|---|---|---|\n")

	for _, f := range report.Files {
		for _, finding := range f.Findings {
			if finding.Status == StatusManual || finding.Status == StatusUnknown {
				helmTag := ""
				if f.HasHelmTpl {
					helmTag = " (Helm)"
				}
				sb.WriteString(fmt.Sprintf("| `%s`%s | `%s` | %s | %s |\n",
					f.Path, helmTag, finding.Annotation, finding.Status, finding.Detail))
			}
		}
	}

	// All findings
	sb.WriteString("\n## Full Report\n\n")
	sb.WriteString("| File | Annotation | Status | Migration |\n")
	sb.WriteString("|---|---|---|---|\n")

	for _, f := range report.Files {
		helmTag := ""
		if f.HasHelmTpl {
			helmTag = " (Helm)"
		}
		for _, finding := range f.Findings {
			sb.WriteString(fmt.Sprintf("| `%s`%s | `%s` | %s | %s |\n",
				f.Path, helmTag, finding.Annotation, finding.Status, finding.Detail))
		}
	}

	return sb.String()
}

// ScanIngressesFromCharts is a convenience function that scans a charts directory
// and returns parsed Ingress resources compatible with the converter.
// It uses the converter's ParseIngressYAML for raw files and falls back to
// regex-based extraction for Helm templates.
func ScanIngressesFromCharts(dir string) (converter.ParsedInput, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if reYAMLFile.MatchString(info.Name()) && !strings.HasSuffix(info.Name(), ".tpl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return converter.ParsedInput{}, err
	}

	// Collect all raw YAML files that contain Ingress resources
	var allData []byte
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if reIngressKind.MatchString(string(data)) && !reHelmTemplate.MatchString(string(data)) {
			allData = append(allData, []byte("---\n")...)
			allData = append(allData, data...)
		}
	}

	if len(allData) == 0 {
		return converter.ParsedInput{}, fmt.Errorf("no non-templated Ingress resources found in %s", dir)
	}

	return converter.ParseIngressYAML(allData)
}
