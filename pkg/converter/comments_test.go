package converter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ingress2apisix/pkg/apisix"
	"github.com/ingress2apisix/pkg/ingress"
)

// --- Unit tests for YAML comment injection ---

func TestAddAnnotationSourceComments(t *testing.T) {
	yamlStr := `metadata:
  name: test
  annotations:
    k8s.apisix.apache.org/enable-cors: "true"
    k8s.apisix.apache.org/cors-allow-origin: "*"
    k8s.apisix.apache.org/http-to-https: "true"`
	sources := map[string]string{
		"k8s.apisix.apache.org/enable-cors":       "nginx.ingress.kubernetes.io/enable-cors",
		"k8s.apisix.apache.org/cors-allow-origin": "nginx.ingress.kubernetes.io/cors-allow-origin",
		"k8s.apisix.apache.org/http-to-https":     "nginx.ingress.kubernetes.io/force-ssl-redirect",
	}

	result := addAnnotationSourceComments(yamlStr, sources)

	if !strings.Contains(result, `k8s.apisix.apache.org/enable-cors: "true"  # Source: nginx.ingress.kubernetes.io/enable-cors`) {
		t.Error("expected source comment for enable-cors annotation")
	}
	if !strings.Contains(result, `k8s.apisix.apache.org/cors-allow-origin: "*"  # Source: nginx.ingress.kubernetes.io/cors-allow-origin`) {
		t.Error("expected source comment for cors-allow-origin annotation")
	}
	if !strings.Contains(result, `# Source: nginx.ingress.kubernetes.io/force-ssl-redirect`) {
		t.Error("expected source comment for http-to-https annotation")
	}
}

func TestAddAnnotationSourceComments_EmptySources(t *testing.T) {
	yamlStr := "metadata:\n  name: test"
	result := addAnnotationSourceComments(yamlStr, nil)
	if result != yamlStr {
		t.Error("expected no change when sources is nil")
	}
}

func TestAddPluginSourceCommentsV2(t *testing.T) {
	yamlStr := `spec:
  plugins:
    - name: limit-req
      enable: true
      config:
        rate: "100"
        burst: 0
    - name: cors
      enable: true
      config:
        allow_credential: true`
	sources := map[string]string{
		"limit-req": "nginx.ingress.kubernetes.io/limit-rps",
		"cors":      "nginx.ingress.kubernetes.io/cors-allow-credentials",
	}

	result := addPluginSourceCommentsV2(yamlStr, sources)

	lines := strings.Split(result, "\n")
	var foundLimitReq, foundCors bool
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# Source: nginx.ingress.kubernetes.io/limit-rps" {
			foundLimitReq = true
		}
		if trimmed == "# Source: nginx.ingress.kubernetes.io/cors-allow-credentials" {
			foundCors = true
		}
	}
	if !foundLimitReq {
		t.Error("expected source comment for limit-req plugin")
	}
	if !foundCors {
		t.Error("expected source comment for cors plugin")
	}
}

func TestAddPluginSourceCommentsV2_EmptySources(t *testing.T) {
	yamlStr := "spec:\n  plugins:\n    - name: limit-req"
	result := addPluginSourceCommentsV2(yamlStr, nil)
	if result != yamlStr {
		t.Error("expected no change when sources is nil")
	}
}

func TestAddFieldSourceComments(t *testing.T) {
	yamlStr := `spec:
  hosts:
    - secure.com
  secret:
    name: tls-sec
    namespace: default`
	sources := map[string]string{
		"hosts":  "spec.tls[0].hosts",
		"secret": "spec.tls[0].secretName",
	}

	result := addFieldSourceComments(yamlStr, sources)

	if !strings.Contains(result, "hosts:  # Source: spec.tls[0].hosts") {
		t.Error("expected source comment for hosts field")
	}
}

func TestAddSourceCommentsForResource_Combined(t *testing.T) {
	annotationSources := map[string]string{
		"k8s.apisix.apache.org/enable-cors": "nginx.ingress.kubernetes.io/enable-cors",
	}
	pluginSources := map[string]string{
		"limit-req": "nginx.ingress.kubernetes.io/limit-rps",
	}
	fieldSources := map[string]string{}

	yamlStr := `apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: test-plugins
  annotations:
    k8s.apisix.apache.org/enable-cors: "true"
spec:
  plugins:
    - name: limit-req
      enable: true`

	result := addSourceCommentsForResource(yamlStr, annotationSources, pluginSources, fieldSources, nil)

	if !strings.Contains(result, "# Source: nginx.ingress.kubernetes.io/enable-cors") {
		t.Error("expected source comment for annotation")
	}
	if !strings.Contains(result, "# Source: nginx.ingress.kubernetes.io/limit-rps") {
		t.Error("expected source comment for plugin")
	}
}

// --- Integration tests: verify source comments in full YAML output ---

func TestSourceComments_CORSAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cors-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Source: nginx.ingress.kubernetes.io/enable-cors") {
		t.Errorf("expected source comment for enable-cors in output:\n%s", output)
	}
}

func TestSourceComments_RateLimitPlugin(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("rl-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rps": "1000",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Source: nginx.ingress.kubernetes.io/limit-rps") {
		t.Errorf("expected source comment for limit-rps in output:\n%s", output)
	}
}

func TestSourceComments_SSLRedirect(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("ssl-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/force-ssl-redirect": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Source: nginx.ingress.kubernetes.io/force-ssl-redirect") {
		t.Errorf("expected source comment for force-ssl-redirect in output:\n%s", output)
	}
}

func TestSourceComments_RewriteTarget(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("rewrite-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/rewrite-target": "/",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Source: nginx.ingress.kubernetes.io/rewrite-target") {
		t.Errorf("expected source comment for rewrite-target in output:\n%s", output)
	}
}

func TestSourceComments_HealthCheck(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("hc-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/health-check-path":     "/healthz",
			"nginx.ingress.kubernetes.io/health-check-interval": "10",
			"nginx.ingress.kubernetes.io/health-check-timeout":  "5",
			"nginx.ingress.kubernetes.io/health-check-retries":  "3",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Source: nginx.ingress.kubernetes.io/health-check-path") {
		t.Errorf("expected source comment for health-check-path in output:\n%s", output)
	}
	if !strings.Contains(output, "# Source: nginx.ingress.kubernetes.io/health-check-interval") {
		t.Errorf("expected source comment for health-check-interval in output:\n%s", output)
	}
}

func TestSourceComments_SessionAffinity(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("affinity-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/affinity":            "cookie",
			"nginx.ingress.kubernetes.io/session-cookie-name": "MYCOOKIE",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Source: nginx.ingress.kubernetes.io/affinity") {
		t.Errorf("expected source comment for affinity in output:\n%s", output)
	}
}

func TestSourceComments_NoAnnotationsProducesNoComments(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("no-anns", "default", nil, nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "# Source:") {
		t.Errorf("expected no source comments for ingress without annotations:\n%s", output)
	}
}

func TestSourceComments_MultipleAnnotations(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("multi-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/proxy-connect-timeout": "10",
			"nginx.ingress.kubernetes.io/proxy-send-timeout":    "20",
			"nginx.ingress.kubernetes.io/proxy-read-timeout":    "30",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	for _, suffix := range []string{"proxy-connect-timeout", "proxy-send-timeout", "proxy-read-timeout"} {
		key := "nginx.ingress.kubernetes.io/" + suffix
		if !strings.Contains(output, "# Source: "+key) {
			t.Errorf("expected source comment for %s in output:\n%s", suffix, output)
		}
	}
}

func TestSourceComments_AuthTypeAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("auth-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/auth-type": "basic",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Source: nginx.ingress.kubernetes.io/auth-type") {
		t.Errorf("expected source comment for auth-type in output:\n%s", output)
	}
}

func TestSourceComments_PlainPrefixAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("plain-test", "default",
		map[string]string{
			"ingress.kubernetes.io/enable-cors": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Source: ingress.kubernetes.io/enable-cors") {
		t.Errorf("expected source comment for plain prefix enable-cors in output:\n%s", output)
	}
}

func TestSourceComments_TlsSection(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("tls-test", "default", nil,
		[]ingress.IngressTLS{
			{Hosts: []string{"secure.com"}, SecretName: "tls-sec"},
		},
		[]ingress.IngressRule{makeSimpleRule("secure.com", "/", "svc", 443)},
	)

	result := c.Convert(ing)

	if len(result.ApisixTls) == 0 {
		t.Fatal("expected ApisixTls to be produced")
	}

	// Verify the SourceAnnotations are set on the ApisixTls
	if result.ApisixTls[0].SourceAnnotations == nil {
		t.Fatal("expected SourceAnnotations to be set on ApisixTls")
	}
}

func TestSourceComments_SessionCookieHashPlugin(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("sch-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/session-cookie-hash":    "sha256",
			"nginx.ingress.kubernetes.io/session-cookie-name":    "MYSESSION",
			"nginx.ingress.kubernetes.io/session-cookie-max-age": "3600",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "session-cookie-hash") {
		t.Errorf("expected source comment for session-cookie-hash in output:\n%s", output)
	}
	if !strings.Contains(output, "session-cookie-max-age") {
		t.Errorf("expected source comment for session-cookie-max-age in output:\n%s", output)
	}
}

func TestSourceComments_UpstreamHashBy(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("hash-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/upstream-hash-by": "$remote_addr",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.BackendTrafficPolicies) == 0 {
		t.Fatal("expected BackendTrafficPolicy to be produced")
	}

	btp := result.BackendTrafficPolicies[0]
	if btp.SourceAnnotations == nil {
		t.Fatal("expected SourceAnnotations on BackendTrafficPolicy")
	}
	if _, ok := btp.SourceAnnotations["loadbalancer"]; !ok {
		t.Error("expected loadbalancer source annotation on BackendTrafficPolicy")
	}
}

// Verify that the source comment format is consistent and well-formed
func TestSourceCommentFormat(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("format-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors":    "true",
			"nginx.ingress.kubernetes.io/limit-rps":      "100",
			"nginx.ingress.kubernetes.io/rewrite-target": "/",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "# Source:") {
			// Verify format: should be "  # Source: nginx.ingress.kubernetes.io/..."
			if !strings.Contains(line, "# Source: ") {
				t.Errorf("malformed source comment: %q", line)
			}
		}
	}
}

// Benchmark to ensure comment injection doesn't significantly impact performance
func BenchmarkWriteConversionResult_WithComments(b *testing.B) {
	c := New(apisix.DefaultConversionOptions())
	ing := makeTestIngress("bench-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors":           "true",
			"nginx.ingress.kubernetes.io/limit-rps":             "1000",
			"nginx.ingress.kubernetes.io/rewrite-target":        "/",
			"nginx.ingress.kubernetes.io/proxy-connect-timeout": "10",
			"nginx.ingress.kubernetes.io/auth-type":             "basic",
		},
		[]ingress.IngressTLS{
			{Hosts: []string{"app.com"}, SecretName: "tls-sec"},
		},
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_ = WriteConversionResult(&buf, result)
	}
}

// --- Default hint comment tests ---

func TestFormatSourceComment_Explicit(t *testing.T) {
	got := formatSourceComment("nginx.ingress.kubernetes.io/enable-cors")
	want := "# Source: nginx.ingress.kubernetes.io/enable-cors"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatSourceComment_Default(t *testing.T) {
	got := formatSourceComment("DEFAULT:nginx.ingress.kubernetes.io/session-cookie-name")
	want := "# Default — set nginx.ingress.kubernetes.io/session-cookie-name to customize"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatSourceComment_DefaultHint(t *testing.T) {
	got := formatSourceComment("DEFAULT_HINT:auto-enabled because spec.tls is configured")
	want := "# Default — auto-enabled because spec.tls is configured"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Test that CORS defaults produce "Default" comments, not "Source" comments
func TestDefaultHints_CORSDefaults(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cors-defaults", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// enable-cors itself should be a "Source" comment (appears on the plugin name line)
	if !strings.Contains(output, "# Source: nginx.ingress.kubernetes.io/enable-cors") {
		t.Errorf("expected source comment for enable-cors:\n%s", output)
	}

	// cors-allow-methods should be a "Default" comment (not explicitly set)
	if !strings.Contains(output, "# Default — set nginx.ingress.kubernetes.io/cors-allow-methods to customize") {
		t.Errorf("expected default hint for cors-allow-methods:\n%s", output)
	}

	// cors-allow-headers should be a "Default" comment
	if !strings.Contains(output, "# Default — set nginx.ingress.kubernetes.io/cors-allow-headers to customize") {
		t.Errorf("expected default hint for cors-allow-headers:\n%s", output)
	}

	// allow_credential defaults to true (ingress-nginx default)
	if !strings.Contains(output, "Default — ingress-nginx default is true") {
		t.Errorf("expected default hint for allow_credential:\n%s", output)
	}

	// max_age defaults to 1728000
	if !strings.Contains(output, "Default — ingress-nginx default is 1728000") {
		t.Errorf("expected default hint for max_age:\n%s", output)
	}

	// Origin is * with credentials, so allow_origins_by_regex is used
	if !strings.Contains(output, "Default — ingress-nginx default origin is *") {
		t.Errorf("expected default hint for allow_origins_by_regex:\n%s", output)
	}
}

// Test that explicit CORS annotations override defaults with "Source" comments
func TestDefaultHints_CORSExplicitOverrides(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cors-explicit", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors":       "true",
			"nginx.ingress.kubernetes.io/cors-allow-origin": "https://example.com",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// cors-allow-origin should appear in the plugin source comment (explicitly set)
	if !strings.Contains(output, "nginx.ingress.kubernetes.io/cors-allow-origin") {
		t.Errorf("expected source reference for explicit cors-allow-origin:\n%s", output)
	}
	if strings.Contains(output, "Default — set nginx.ingress.kubernetes.io/cors-allow-origin to customize") {
		t.Errorf("should NOT have default hint for explicitly set cors-allow-origin:\n%s", output)
	}

	// cors-allow-methods should still be a "Default" comment (not explicitly set)
	if !strings.Contains(output, "# Default — set nginx.ingress.kubernetes.io/cors-allow-methods to customize") {
		t.Errorf("expected default hint for cors-allow-methods:\n%s", output)
	}
}

// Test that SSL redirect from spec.tls produces a default hint
func TestDefaultHints_SSLRedirectFromSpecTLS(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("ssl-default", "default", nil,
		[]ingress.IngressTLS{
			{Hosts: []string{"secure.com"}, SecretName: "tls-sec"},
		},
		[]ingress.IngressRule{makeSimpleRule("secure.com", "/", "svc", 443)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "# Default — auto-enabled because spec.tls is configured") {
		t.Errorf("expected default hint for SSL redirect from spec.tls:\n%s", output)
	}
}

// Test that session-cookie-hash with default cookie_name gets a default hint on the plugin config field
func TestDefaultHints_SessionCookieHashDefaultCookieName(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("sch-default", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/session-cookie-hash": "sha256",
			// session-cookie-name NOT set — defaults to INGRESSCOOKIE
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) == 0 {
		t.Fatal("expected PluginConfig to be produced")
	}

	pc := result.PluginConfigs[0]
	if pc.PluginFieldDefaults == nil {
		t.Fatal("expected PluginFieldDefaults to be set")
	}

	key := "session-cookie-hash.cookie_name"
	val, ok := pc.PluginFieldDefaults[key]
	if !ok {
		t.Fatalf("expected PluginFieldDefaults to have key %q, got: %v", key, pc.PluginFieldDefaults)
	}
	if !strings.HasPrefix(val, "DEFAULT:") {
		t.Errorf("expected DEFAULT: prefix, got: %q", val)
	}

	// Verify the default hint appears in the YAML output
	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Default — set nginx.ingress.kubernetes.io/session-cookie-name to customize") {
		t.Errorf("expected default hint for session-cookie-name in plugin config:\n%s", output)
	}
}

// Test that session-cookie-hash with explicit cookie_name does NOT get a default hint
func TestDefaultHints_SessionCookieHashExplicitCookieName(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("sch-explicit", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/session-cookie-hash": "sha256",
			"nginx.ingress.kubernetes.io/session-cookie-name": "MYSESSION",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) == 0 {
		t.Fatal("expected PluginConfig to be produced")
	}

	pc := result.PluginConfigs[0]
	if pc.PluginFieldDefaults != nil {
		if _, ok := pc.PluginFieldDefaults["session-cookie-hash.cookie_name"]; ok {
			t.Error("should NOT have default hint for explicitly set session-cookie-name")
		}
	}

	// Verify NO default hint in the output
	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "Default — set nginx.ingress.kubernetes.io/session-cookie-name to customize") {
		t.Errorf("should NOT have default hint for explicitly set cookie name:\n%s", output)
	}
	// Should have a plugin-level Source comment
	if !strings.Contains(output, "# Source: nginx.ingress.kubernetes.io/session-cookie-hash") {
		t.Errorf("expected plugin-level source comment for session-cookie-hash:\n%s", output)
	}
}

// Test BackendTrafficPolicy default cookie key
func TestDefaultHints_BackendTrafficPolicyCookieDefault(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("btp-default", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/affinity": "cookie",
			// session-cookie-name NOT set — defaults to INGRESSCOOKIE
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.BackendTrafficPolicies) == 0 {
		t.Fatal("expected BackendTrafficPolicy to be produced")
	}

	btp := result.BackendTrafficPolicies[0]
	if btp.SourceAnnotations == nil {
		t.Fatal("expected SourceAnnotations on BackendTrafficPolicy")
	}

	loadbalancerSrc, ok := btp.SourceAnnotations["loadbalancer"]
	if !ok {
		t.Fatal("expected loadbalancer source annotation")
	}
	if !strings.HasPrefix(loadbalancerSrc, "DEFAULT:") {
		t.Errorf("expected DEFAULT: prefix for default cookie name, got: %q", loadbalancerSrc)
	}
}

// Test ApisixUpstream health-check-path default hint
func TestDefaultHints_HealthCheckPathDefault(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("hc-default", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/health-check-interval": "10",
			// health-check-path NOT set — defaults to "/"
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.ApisixUpstreams) == 0 {
		t.Fatal("expected ApisixUpstream to be produced")
	}

	au := result.ApisixUpstreams[0]
	if au.SourceAnnotations == nil {
		t.Fatal("expected SourceAnnotations on ApisixUpstream")
	}

	hcPathSrc, ok := au.SourceAnnotations["healthCheck.active.httpPath"]
	if !ok {
		t.Fatal("expected healthCheck.active.httpPath source annotation")
	}
	if !strings.HasPrefix(hcPathSrc, "DEFAULT:") {
		t.Errorf("expected DEFAULT: prefix for default health-check-path, got: %q", hcPathSrc)
	}

	// Verify the default hint appears in YAML output
	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Default — set nginx.ingress.kubernetes.io/health-check-path to customize") {
		t.Errorf("expected default hint for health-check-path:\n%s", output)
	}
}

// Test that no-default scenarios don't produce any default hints
func TestDefaultHints_NoDefaultsForExplicitValues(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("no-defaults", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/proxy-connect-timeout": "10",
			"nginx.ingress.kubernetes.io/proxy-send-timeout":    "20",
			"nginx.ingress.kubernetes.io/rewrite-target":        "/api",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "# Default") {
		t.Errorf("should not have any default hints when all values are explicit:\n%s", output)
	}
}

// Test addPluginFieldDefaultComments unit function
func TestAddPluginFieldDefaultComments(t *testing.T) {
	yamlStr := `spec:
  plugins:
    - name: session-cookie-hash
      enable: true
      config:
        cookie_name: INGRESSCOOKIE
        algorithm: sha256
    - name: limit-req
      enable: true
      config:
        rate: "100"`
	defaults := map[string]string{
		"session-cookie-hash.cookie_name": "DEFAULT:nginx.ingress.kubernetes.io/session-cookie-name",
	}

	result := addPluginFieldDefaultComments(yamlStr, defaults)

	if !strings.Contains(result, "cookie_name: INGRESSCOOKIE  # Default — set nginx.ingress.kubernetes.io/session-cookie-name to customize") {
		t.Errorf("expected default hint on cookie_name field:\n%s", result)
	}
	// limit-req's rate should NOT have a default hint
	if strings.Contains(result, "rate: \"100\"  # Default") {
		t.Errorf("should NOT have default hint on rate field:\n%s", result)
	}
}

// Test addPluginFieldDefaultComments with empty defaults
func TestAddPluginFieldDefaultComments_Empty(t *testing.T) {
	yamlStr := "spec:\n  plugins:\n    - name: test"
	result := addPluginFieldDefaultComments(yamlStr, nil)
	if result != yamlStr {
		t.Error("expected no change when defaults is nil")
	}
}

// --- Comprehensive default hints tests ---

func TestDefaultHints_LimitReqPlugin(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("lr-defaults", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rps": "100",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) == 0 {
		t.Fatal("expected PluginConfig to be produced")
	}

	pc := result.PluginConfigs[0]
	if pc.PluginFieldDefaults == nil {
		t.Fatal("expected PluginFieldDefaults to be set")
	}

	expected := map[string]bool{
		"limit-req.burst":         true,
		"limit-req.key":           true,
		"limit-req.rejected_code": true,
	}
	for key := range expected {
		val, ok := pc.PluginFieldDefaults[key]
		if !ok {
			t.Errorf("expected PluginFieldDefaults to have key %q", key)
			continue
		}
		if !strings.HasPrefix(val, "DEFAULT_HINT:") {
			t.Errorf("expected DEFAULT_HINT: prefix for %q, got: %q", key, val)
		}
	}

	// Verify in YAML output
	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `burst: 0  # Default — hardcoded, same as nginx default`) {
		t.Errorf("expected default hint for burst:\n%s", output)
	}
	if !strings.Contains(output, `key: remote_addr  # Default — hardcoded, same as nginx default`) {
		t.Errorf("expected default hint for key:\n%s", output)
	}
	if !strings.Contains(output, `rejected_code: "429"  # Default — hardcoded, same as nginx default`) {
		t.Errorf("expected default hint for rejected_code:\n%s", output)
	}
}

func TestDefaultHints_LimitReqFromSnippet_NoRejectedCodeDefault(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("lr-snippet", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rps":             "100",
			"nginx.ingress.kubernetes.io/configuration-snippet": "limit_req_status 503;",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) == 0 {
		t.Fatal("expected PluginConfig to be produced")
	}

	pc := result.PluginConfigs[0]
	if pc.PluginFieldDefaults == nil {
		t.Fatal("expected PluginFieldDefaults to be set")
	}

	// rejected_code should NOT have DEFAULT hint since it came from snippet
	if val, ok := pc.PluginFieldDefaults["limit-req.rejected_code"]; ok && strings.HasPrefix(val, "DEFAULT") {
		t.Errorf("should NOT have DEFAULT hint for rejected_code from snippet, got: %q", val)
	}

	// burst and key should still have DEFAULT hints
	if val, ok := pc.PluginFieldDefaults["limit-req.burst"]; !ok || !strings.HasPrefix(val, "DEFAULT") {
		t.Errorf("expected DEFAULT hint for burst, got: %q", val)
	}
	if val, ok := pc.PluginFieldDefaults["limit-req.key"]; !ok || !strings.HasPrefix(val, "DEFAULT") {
		t.Errorf("expected DEFAULT hint for key, got: %q", val)
	}
}

func TestDefaultHints_LimitRpmPlugin(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("lr-rpm", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rpm": "6000",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) == 0 {
		t.Fatal("expected PluginConfig to be produced")
	}

	pc := result.PluginConfigs[0]
	if pc.PluginFieldDefaults == nil {
		t.Fatal("expected PluginFieldDefaults to be set")
	}

	// All three should have DEFAULT hints
	for _, key := range []string{"limit-req.burst", "limit-req.key", "limit-req.rejected_code"} {
		val, ok := pc.PluginFieldDefaults[key]
		if !ok {
			t.Errorf("expected PluginFieldDefaults to have key %q", key)
			continue
		}
		if !strings.HasPrefix(val, "DEFAULT_HINT:") {
			t.Errorf("expected DEFAULT_HINT: prefix for %q, got: %q", key, val)
		}
	}
}

func TestDefaultHints_LimitConnPlugin(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("lc-defaults", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-connections": "10",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) == 0 {
		t.Fatal("expected PluginConfig to be produced")
	}

	pc := result.PluginConfigs[0]
	if pc.PluginFieldDefaults == nil {
		t.Fatal("expected PluginFieldDefaults to be set")
	}

	for _, key := range []string{"limit-conn.burst", "limit-conn.key", "limit-conn.rejected_code"} {
		val, ok := pc.PluginFieldDefaults[key]
		if !ok {
			t.Errorf("expected PluginFieldDefaults to have key %q", key)
			continue
		}
		if !strings.HasPrefix(val, "DEFAULT_HINT:") {
			t.Errorf("expected DEFAULT_HINT: prefix for %q, got: %q", key, val)
		}
	}

	// Verify in YAML output
	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "burst: 0  # Default — hardcoded, same as nginx default") {
		t.Errorf("expected default hint for burst in limit-conn:\n%s", output)
	}
	if !strings.Contains(output, `key: remote_addr  # Default — hardcoded, same as nginx default`) {
		t.Errorf("expected default hint for key in limit-conn:\n%s", output)
	}
	if !strings.Contains(output, `rejected_code: "503"  # Default — hardcoded, same as nginx default`) {
		t.Errorf("expected default hint for rejected_code in limit-conn:\n%s", output)
	}
}

func TestDefaultHints_SessionCookieHashHardcodedFields(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("sch-hardcoded", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/session-cookie-hash": "sha256",
			"nginx.ingress.kubernetes.io/session-cookie-name": "MYSESSION",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) == 0 {
		t.Fatal("expected PluginConfig to be produced")
	}

	pc := result.PluginConfigs[0]
	if pc.PluginFieldDefaults == nil {
		t.Fatal("expected PluginFieldDefaults to be set")
	}

	// Hardcoded fields should have DEFAULT_HINT
	for _, key := range []string{
		"session-cookie-hash.header_name",
		"session-cookie-hash.fallback",
		"session-cookie-hash.generate_cookie",
		"session-cookie-hash.cookie_httponly",
	} {
		val, ok := pc.PluginFieldDefaults[key]
		if !ok {
			t.Errorf("expected PluginFieldDefaults to have key %q", key)
			continue
		}
		if !strings.HasPrefix(val, "DEFAULT_HINT:") {
			t.Errorf("expected DEFAULT_HINT: prefix for %q, got: %q", key, val)
		}
	}

	// cookie_name should NOT have a default since it was explicitly set
	if val, ok := pc.PluginFieldDefaults["session-cookie-hash.cookie_name"]; ok && strings.HasPrefix(val, "DEFAULT") {
		t.Errorf("should NOT have DEFAULT hint for explicitly set cookie_name, got: %q", val)
	}

	// algorithm should NOT have a default since it was explicitly set
	if val, ok := pc.PluginFieldDefaults["session-cookie-hash.algorithm"]; ok {
		t.Errorf("should NOT have algorithm default hint for explicit hash, got: %q", val)
	}

	// Verify in YAML output
	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `header_name: X-Session-Hash  # Default — hardcoded default`) {
		t.Errorf("expected default hint for header_name:\n%s", output)
	}
	if !strings.Contains(output, `fallback: pass  # Default — hardcoded default`) {
		t.Errorf("expected default hint for fallback:\n%s", output)
	}
	if !strings.Contains(output, `generate_cookie: true  # Default — hardcoded default`) {
		t.Errorf("expected default hint for generate_cookie:\n%s", output)
	}
	if !strings.Contains(output, `cookie_httponly: false  # Default — hardcoded default`) {
		t.Errorf("expected default hint for cookie_httponly:\n%s", output)
	}
}

func TestDefaultHints_SessionCookieHashAlgorithmDefault(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("sch-algo-default", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/affinity": "cookie",
			// session-cookie-hash NOT set — defaults to sha1
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) == 0 {
		t.Fatal("expected PluginConfig to be produced")
	}

	pc := result.PluginConfigs[0]
	if pc.PluginFieldDefaults == nil {
		t.Fatal("expected PluginFieldDefaults to be set")
	}

	// algorithm should have DEFAULT since it was inferred from affinity=cookie
	val, ok := pc.PluginFieldDefaults["session-cookie-hash.algorithm"]
	if !ok {
		t.Fatal("expected PluginFieldDefaults to have session-cookie-hash.algorithm")
	}
	if !strings.HasPrefix(val, "DEFAULT:") {
		t.Errorf("expected DEFAULT: prefix for algorithm default, got: %q", val)
	}
}

func TestDefaultHints_RealIPPlugin(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("real-ip-defaults", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-real-ip": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) == 0 {
		t.Fatal("expected PluginConfig to be produced")
	}

	pc := result.PluginConfigs[0]
	if pc.PluginFieldDefaults == nil {
		t.Fatal("expected PluginFieldDefaults to be set")
	}

	// All three fields should have DEFAULT_HINT
	for _, key := range []string{
		"real-ip.source",
		"real-ip.trusted_addresses",
		"real-ip.recursive",
	} {
		val, ok := pc.PluginFieldDefaults[key]
		if !ok {
			t.Errorf("expected PluginFieldDefaults to have key %q", key)
			continue
		}
		if !strings.HasPrefix(val, "DEFAULT_HINT:") {
			t.Errorf("expected DEFAULT_HINT: prefix for %q, got: %q", key, val)
		}
	}

	// Verify in YAML output
	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `source: http_x_forwarded_for  # Default — hardcoded, same as nginx default`) {
		t.Errorf("expected default hint for source:\n%s", output)
	}
}

func TestDefaultHints_RealIPPlugin_ExplicitSource(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("real-ip-explicit", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-real-ip":       "true",
			"nginx.ingress.kubernetes.io/forwarded-for-header": "X-Custom-IP",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) == 0 {
		t.Fatal("expected PluginConfig to be produced")
	}

	pc := result.PluginConfigs[0]

	// source should NOT have default since it was explicitly configured
	if val, ok := pc.PluginFieldDefaults["real-ip.source"]; ok && strings.HasPrefix(val, "DEFAULT") {
		t.Errorf("should NOT have DEFAULT hint for explicitly configured source, got: %q", val)
	}

	// trusted_addresses and recursive should still have defaults
	if val, ok := pc.PluginFieldDefaults["real-ip.trusted_addresses"]; !ok || !strings.HasPrefix(val, "DEFAULT_HINT") {
		t.Errorf("expected DEFAULT_HINT for trusted_addresses, got: %q", val)
	}
	if val, ok := pc.PluginFieldDefaults["real-ip.recursive"]; !ok || !strings.HasPrefix(val, "DEFAULT_HINT") {
		t.Errorf("expected DEFAULT_HINT for recursive, got: %q", val)
	}
}

func TestDefaultHints_RealIPPlugin_ExplicitRecursive(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("real-ip-recursive", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-real-ip":        "true",
			"nginx.ingress.kubernetes.io/use-forwarded-headers": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) == 0 {
		t.Fatal("expected PluginConfig to be produced")
	}

	pc := result.PluginConfigs[0]

	// recursive should NOT have default since use-forwarded-headers sets it explicitly
	if val, ok := pc.PluginFieldDefaults["real-ip.recursive"]; ok && strings.HasPrefix(val, "DEFAULT") {
		t.Errorf("should NOT have DEFAULT hint for explicitly configured recursive, got: %q", val)
	}

	// source and trusted_addresses should still have defaults
	if val, ok := pc.PluginFieldDefaults["real-ip.source"]; !ok || !strings.HasPrefix(val, "DEFAULT_HINT") {
		t.Errorf("expected DEFAULT_HINT for source, got: %q", val)
	}
	if val, ok := pc.PluginFieldDefaults["real-ip.trusted_addresses"]; !ok || !strings.HasPrefix(val, "DEFAULT_HINT") {
		t.Errorf("expected DEFAULT_HINT for trusted_addresses, got: %q", val)
	}
}

func TestDefaultHints_HealthCheckHardcodedDefaults(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("hc-hardcoded", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/health-check-path":     "/healthz",
			"nginx.ingress.kubernetes.io/health-check-interval": "10",
			"nginx.ingress.kubernetes.io/health-check-timeout":  "5",
			"nginx.ingress.kubernetes.io/health-check-retries":  "3",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.ApisixUpstreams) == 0 {
		t.Fatal("expected ApisixUpstream to be produced")
	}

	au := result.ApisixUpstreams[0]
	if au.SourceAnnotations == nil {
		t.Fatal("expected SourceAnnotations on ApisixUpstream")
	}

	// type should have DEFAULT_HINT
	typeSrc, ok := au.SourceAnnotations["healthCheck.active.type"]
	if !ok {
		t.Fatal("expected healthCheck.active.type source annotation")
	}
	if !strings.HasPrefix(typeSrc, "DEFAULT_HINT:") {
		t.Errorf("expected DEFAULT_HINT: prefix for type, got: %q", typeSrc)
	}

	// unhealthy.httpCodes should have DEFAULT_HINT
	httpCodesSrc, ok := au.SourceAnnotations["healthCheck.active.unhealthy.httpCodes"]
	if !ok {
		t.Fatal("expected healthCheck.active.unhealthy.httpCodes source annotation")
	}
	if !strings.HasPrefix(httpCodesSrc, "DEFAULT_HINT:") {
		t.Errorf("expected DEFAULT_HINT: prefix for httpCodes, got: %q", httpCodesSrc)
	}

	// unhealthy.tcpFailures should have DEFAULT_HINT
	tcpSrc, ok := au.SourceAnnotations["healthCheck.active.unhealthy.tcpFailures"]
	if !ok {
		t.Fatal("expected healthCheck.active.unhealthy.tcpFailures source annotation")
	}
	if !strings.HasPrefix(tcpSrc, "DEFAULT_HINT:") {
		t.Errorf("expected DEFAULT_HINT: prefix for tcpFailures, got: %q", tcpSrc)
	}

	// Verify in YAML output
	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Default — hardcoded to http") {
		t.Errorf("expected default hint for type:\n%s", output)
	}
	if !strings.Contains(output, "Default — hardcoded, same as nginx default") {
		t.Errorf("expected default hint for httpCodes:\n%s", output)
	}
	if !strings.Contains(output, "Default — hardcoded default") {
		t.Errorf("expected default hint for tcpFailures:\n%s", output)
	}
}
