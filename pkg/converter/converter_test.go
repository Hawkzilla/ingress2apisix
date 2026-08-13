package converter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ingress2apisix/pkg/apisix"
	"github.com/ingress2apisix/pkg/ingress"
)

// --- ParseIngressYAML Tests ---

func TestParseIngressYAML_SingleIngress(t *testing.T) {
	yaml := []byte(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
  namespace: default
spec:
  ingressClassName: nginx
  rules:
    - host: example.com
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: api-svc
                port:
                  number: 8080
`)
	input, err := ParseIngressYAML(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(input.Ingresses) != 1 {
		t.Fatalf("expected 1 ingress, got %d", len(input.Ingresses))
	}
	if input.Ingresses[0].Metadata.Name != "test-ingress" {
		t.Errorf("expected name 'test-ingress', got '%s'", input.Ingresses[0].Metadata.Name)
	}
	if input.Format != apisix.FormatSingleDoc {
		t.Errorf("expected FormatSingleDoc, got %d", input.Format)
	}
}

func TestParseIngressYAML_MultiDocument(t *testing.T) {
	yaml := []byte(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-1
  namespace: ns1
spec:
  rules:
    - host: a.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: svc-a
                port:
                  number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-2
  namespace: ns2
spec:
  rules:
    - host: b.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: svc-b
                port:
                  number: 80
`)
	input, err := ParseIngressYAML(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(input.Ingresses) != 2 {
		t.Fatalf("expected 2 ingresses, got %d", len(input.Ingresses))
	}
	if input.Format != apisix.FormatMultiDoc {
		t.Errorf("expected FormatMultiDoc, got %d", input.Format)
	}
}

func TestParseIngressYAML_NoIngress(t *testing.T) {
	yaml := []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: test
`)
	_, err := ParseIngressYAML(yaml)
	if err == nil {
		t.Fatal("expected error for non-ingress resource")
	}
}

func TestParseIngressYAML_MixedDocuments(t *testing.T) {
	yaml := []byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: test
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: mixed-ingress
  namespace: test
spec:
  rules:
    - host: mixed.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: mixed-svc
                port:
                  number: 80
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: test
`)
	input, err := ParseIngressYAML(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(input.Ingresses) != 1 {
		t.Fatalf("expected 1 ingress, got %d", len(input.Ingresses))
	}
}

func TestParseIngressYAML_IngressList(t *testing.T) {
	yamlData := []byte(`
apiVersion: networking.k8s.io/v1
kind: IngressList
items:
  - apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: list-ingress-1
      namespace: ns1
    spec:
      rules:
        - host: a.com
          http:
            paths:
              - path: /
                pathType: Prefix
                backend:
                  service:
                    name: svc-a
                    port:
                      number: 80
  - apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: list-ingress-2
      namespace: ns2
    spec:
      rules:
        - host: b.com
          http:
            paths:
              - path: /
                pathType: Prefix
                backend:
                  service:
                    name: svc-b
                    port:
                      number: 80
`)
	input, err := ParseIngressYAML(yamlData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(input.Ingresses) != 2 {
		t.Fatalf("expected 2 ingresses, got %d", len(input.Ingresses))
	}
	if input.Format != apisix.FormatList {
		t.Errorf("expected FormatList, got %d", input.Format)
	}
	if input.Ingresses[0].Metadata.Name != "list-ingress-1" {
		t.Errorf("expected name 'list-ingress-1', got '%s'", input.Ingresses[0].Metadata.Name)
	}
	if input.Ingresses[1].Metadata.Name != "list-ingress-2" {
		t.Errorf("expected name 'list-ingress-2', got '%s'", input.Ingresses[1].Metadata.Name)
	}
}

func TestParseIngressYAML_KindList(t *testing.T) {
	yamlData := []byte(`
apiVersion: v1
kind: List
items:
  - apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: list-ingress-1
      namespace: ns1
    spec:
      rules:
        - host: a.com
          http:
            paths:
              - path: /
                pathType: Prefix
                backend:
                  service:
                    name: svc-a
                    port:
                      number: 80
  - apiVersion: v1
    kind: Service
    metadata:
      name: should-be-skipped
      namespace: ns1
  - apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: list-ingress-2
      namespace: ns2
    spec:
      rules:
        - host: b.com
          http:
            paths:
              - path: /
                pathType: Prefix
                backend:
                  service:
                    name: svc-b
                    port:
                      number: 80
`)
	input, err := ParseIngressYAML(yamlData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(input.Ingresses) != 2 {
		t.Fatalf("expected 2 ingresses, got %d", len(input.Ingresses))
	}
	if input.Format != apisix.FormatList {
		t.Errorf("expected FormatList, got %d", input.Format)
	}
	if input.Ingresses[0].Metadata.Name != "list-ingress-1" {
		t.Errorf("expected name 'list-ingress-1', got '%s'", input.Ingresses[0].Metadata.Name)
	}
	if input.Ingresses[1].Metadata.Name != "list-ingress-2" {
		t.Errorf("expected name 'list-ingress-2', got '%s'", input.Ingresses[1].Metadata.Name)
	}
}

func TestParseIngressYAML_KindList_IngressOnly(t *testing.T) {
	yamlData := []byte(`
apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: Service
    metadata:
      name: svc-only
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: cm-only
`)
	_, err := ParseIngressYAML(yamlData)
	if err == nil {
		t.Fatal("expected error when List contains no Ingress resources")
	}
}

func TestWriteConversionResult_IngressListFormat(t *testing.T) {
	c := newTestConverter()

	// Build an IngressList input
	yamlData := []byte(`
apiVersion: networking.k8s.io/v1
kind: IngressList
items:
  - apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: list-a
      namespace: default
    spec:
      rules:
        - host: a.com
          http:
            paths:
              - path: /
                pathType: Prefix
                backend:
                  service:
                    name: svc-a
                    port:
                      number: 80
  - apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: list-b
      namespace: default
    spec:
      rules:
        - host: b.com
          http:
            paths:
              - path: /
                pathType: Prefix
                backend:
                  service:
                    name: svc-b
                    port:
                      number: 80
`)
	input, err := ParseIngressYAML(yamlData)
	if err != nil {
		t.Fatalf("error parsing: %v", err)
	}

	result := c.ConvertList(input)

	if result.InputFormat != apisix.FormatList {
		t.Fatalf("expected FormatList in result, got %d", result.InputFormat)
	}
	if len(result.Ingresses) != 2 {
		t.Fatalf("expected 2 converted ingresses, got %d", len(result.Ingresses))
	}

	var buf bytes.Buffer
	err = WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("error writing output: %v", err)
	}

	output := buf.String()
	// Output should contain IngressList wrapping
	if !strings.Contains(output, "kind: IngressList") {
		t.Error("output should contain 'kind: IngressList'")
	}
	if !strings.Contains(output, "list-a") {
		t.Error("output should contain first ingress name")
	}
	if !strings.Contains(output, "list-b") {
		t.Error("output should contain second ingress name")
	}
}

func TestWriteConversionResult_SingleIngressFormat(t *testing.T) {
	c := newTestConverter()

	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("single", "default", nil, nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := c.ConvertList(input)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("error writing output: %v", err)
	}

	output := buf.String()
	// Single Ingress output should NOT contain IngressList
	if strings.Contains(output, "kind: IngressList") {
		t.Error("single Ingress output should not contain 'kind: IngressList'")
	}
	if !strings.Contains(output, "kind: Ingress") {
		t.Error("output should contain 'kind: Ingress'")
	}
}

func newTestConverter() *Converter {
	return New(apisix.DefaultConversionOptions())
}

func strPtr(s string) *string {
	return &s
}

// containsNonComment returns true if the data contains the substring on a non-comment line.
// Handles both standalone comment lines and inline comments (# Source: ...).
func containsNonComment(data []byte, substr []byte) bool {
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		// Skip standalone comment lines
		if bytes.HasPrefix(trimmed, []byte("#")) {
			continue
		}
		// Strip inline comments (everything after "  # ")
		if idx := bytes.Index(line, []byte("  # ")); idx >= 0 {
			line = line[:idx]
		}
		if bytes.Contains(line, substr) {
			return true
		}
	}
	return false
}

func makeTestIngress(name, ns string, annotations map[string]string, tls []ingress.IngressTLS, rules []ingress.IngressRule) ingress.Ingress {
	return ingress.Ingress{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "Ingress",
		Metadata: ingress.Metadata{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
		},
		Spec: ingress.IngressSpec{
			IngressClassName: strPtr("nginx"),
			TLS:              tls,
			Rules:            rules,
		},
	}
}

func makeSimpleRule(host, path, svcName string, svcPort int32) ingress.IngressRule {
	return ingress.IngressRule{
		Host: host,
		HTTP: &ingress.HTTPIngressRuleValue{
			Paths: []ingress.HTTPIngressPath{
				{
					Path:     path,
					PathType: strPtr("Prefix"),
					Backend: ingress.IngressBackend{
						Service: &ingress.IngressServiceBackend{
							Name: svcName,
							Port: ingress.ServiceBackendPort{Number: svcPort},
						},
					},
				},
			},
		},
	}
}

func TestConvert_IngressClassNameChanged(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("test", "default", nil, nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)})

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Spec.IngressClassName == nil || *out.Spec.IngressClassName != "apisix" {
		t.Errorf("expected ingressClassName 'apisix', got '%v'", out.Spec.IngressClassName)
	}
}

func TestConvert_CORS(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cors-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// CORS is now fully handled by ApisixPluginConfig, not AIC annotations
	if out.Metadata.Annotations["k8s.apisix.apache.org/enable-cors"] != "" {
		t.Error("cors AIC annotations should NOT be set (handled by ApisixPluginConfig)")
	}
	if _, ok := out.Metadata.Annotations["nginx.ingress.kubernetes.io/enable-cors"]; ok {
		t.Error("original nginx cors annotation should be removed")
	}
	// Should have ApisixPluginConfig with cors plugin
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig for CORS, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["allow_credential"] != true {
		t.Error("expected allow_credential=true (ingress-nginx default)")
	}
}

func TestConvert_SSLRedirect(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("tls-test", "default", nil,
		[]ingress.IngressTLS{
			{Hosts: []string{"secure.com"}, SecretName: "tls-sec"},
		},
		[]ingress.IngressRule{makeSimpleRule("secure.com", "/", "svc", 443)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// SSL redirect should map to http-to-https (per migration doc 3.1)
	if out.Metadata.Annotations["k8s.apisix.apache.org/http-to-https"] != "true" {
		t.Error("expected http-to-https annotation to be set")
	}
	// Should NOT set the old ssl-redirect key
	if _, ok := out.Metadata.Annotations["k8s.apisix.apache.org/ssl-redirect"]; ok {
		t.Error("should not set ssl-redirect annotation, use http-to-https instead")
	}
}

func TestConvert_ForceSSLRedirect(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("force-ssl", "default",
		map[string]string{
			"ingress.kubernetes.io/force-ssl-redirect": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/http-to-https"] != "true" {
		t.Error("expected http-to-https from force-ssl-redirect")
	}
}

func TestConvert_SSLRedirectFalse(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("ssl-false", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/ssl-redirect": "false",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/http-to-https"] != "false" {
		t.Errorf("expected http-to-https=false, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/http-to-https"])
	}
}

func TestConvert_SSLRedirectDisabled(t *testing.T) {
	opts := apisix.DefaultConversionOptions()
	opts.SSLRedirect = false
	c := New(opts)

	ing := makeTestIngress("no-ssl", "default", nil,
		[]ingress.IngressTLS{
			{Hosts: []string{"secure.com"}, SecretName: "tls-sec"},
		},
		[]ingress.IngressRule{makeSimpleRule("secure.com", "/", "svc", 443)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if _, ok := out.Metadata.Annotations["k8s.apisix.apache.org/http-to-https"]; ok {
		t.Error("http-to-https should not be set when SSLRedirect=false")
	}
}

func TestConvert_Timeouts(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("timeout-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/proxy-connect-timeout": "30",
			"nginx.ingress.kubernetes.io/proxy-read-timeout":    "3600",
			"nginx.ingress.kubernetes.io/proxy-send-timeout":    "60",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// Per migration doc: proxy-connect-timeout → upstream-connect-timeout + "s" suffix
	if out.Metadata.Annotations["k8s.apisix.apache.org/upstream-connect-timeout"] != "30s" {
		t.Errorf("expected upstream-connect-timeout=30s, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/upstream-connect-timeout"])
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/upstream-read-timeout"] != "3600s" {
		t.Errorf("expected upstream-read-timeout=3600s, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/upstream-read-timeout"])
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/upstream-send-timeout"] != "60s" {
		t.Errorf("expected upstream-send-timeout=60s, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/upstream-send-timeout"])
	}
}

func TestConvert_Timeouts_WithExistingSuffix(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("timeout-suffix", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/proxy-read-timeout": "3600s",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// Should not double the suffix
	if out.Metadata.Annotations["k8s.apisix.apache.org/upstream-read-timeout"] != "3600s" {
		t.Errorf("expected 3600s (no double suffix), got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/upstream-read-timeout"])
	}
}

func TestConvert_RewriteTarget_Simple(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("rewrite-simple", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/rewrite-target": "/",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/api", "svc", 80)},
	)

	result := c.Convert(ing)

	// Should produce proxy-rewrite plugin in PluginConfig
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	p := result.PluginConfigs[0].Spec.Plugins[0]
	if p.Name != "proxy-rewrite" {
		t.Fatalf("expected proxy-rewrite plugin, got %s", p.Name)
	}
	cfg := p.Config.(map[string]interface{})
	regexURI := cfg["regex_uri"].([]string)
	if len(regexURI) != 2 {
		t.Fatalf("expected 2 regex_uri entries, got %d", len(regexURI))
	}
	if regexURI[0] != "(?i)/api" {
		t.Errorf("expected regex_uri[0]='(?i)/api', got '%s'", regexURI[0])
	}
	if regexURI[1] != "/" {
		t.Errorf("expected regex_uri[1]='/', got '%s'", regexURI[1])
	}
}

func TestConvert_RewriteTarget_Regex(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("rewrite-regex", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/rewrite-target": "/$2",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/api(/|$)(.*)", "svc", 80)},
	)

	result := c.Convert(ing)

	// Should produce proxy-rewrite plugin in PluginConfig
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	p := result.PluginConfigs[0].Spec.Plugins[0]
	if p.Name != "proxy-rewrite" {
		t.Fatalf("expected proxy-rewrite plugin, got %s", p.Name)
	}
	cfg := p.Config.(map[string]interface{})
	regexURI := cfg["regex_uri"].([]string)
	if len(regexURI) != 2 {
		t.Fatalf("expected 2 regex_uri entries, got %d", len(regexURI))
	}
	if regexURI[0] != "(?i)/api(/|$)(.*)" {
		t.Errorf("expected regex_uri[0]='(?i)/api(/|$)(.*)', got '%s'", regexURI[0])
	}
	if regexURI[1] != "/$2" {
		t.Errorf("expected regex_uri[1]='/$2', got '%s'", regexURI[1])
	}
}

func TestConvert_RateLimiting_ProducesPluginConfig(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("rl-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rps": "1000",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	// Per migration doc 4.1.9: limit-rps → ApisixPluginConfig + limit-req
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	pc := result.PluginConfigs[0]
	if len(pc.Spec.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(pc.Spec.Plugins))
	}
	p := pc.Spec.Plugins[0]
	if p.Name != "limit-req" {
		t.Errorf("expected limit-req plugin, got '%s'", p.Name)
	}
	if !p.Enable {
		t.Error("expected limit-req to be enabled")
	}
	cfg, ok := p.Config.(map[string]interface{})
	if !ok {
		t.Fatal("expected plugin config to be a map")
	}
	rate, ok := cfg["rate"].(float64)
	if !ok {
		t.Fatalf("expected rate to be float64, got %T: %v", cfg["rate"], cfg["rate"])
	}
	if rate != 1000 {
		t.Errorf("expected rate=1000, got %v", rate)
	}
	if cfg["burst"] != 5000 {
		t.Errorf("expected burst=5000 (1000*5), got %v", cfg["burst"])
	}
	if cfg["rejected_code"] != "429" {
		t.Errorf("expected rejected_code=429, got '%v'", cfg["rejected_code"])
	}

	// Should NOT set limit-rps as a simple annotation
	out := result.Ingresses[0].(ingress.Ingress)
	if _, ok := out.Metadata.Annotations["k8s.apisix.apache.org/limit-rps"]; ok {
		t.Error("limit-rps should be handled via plugin, not annotation")
	}
}

func TestConvert_BasicAuth_NativeAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("auth-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/auth-type":  "basic",
			"nginx.ingress.kubernetes.io/auth-realm": "401: Authentication Required",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// auth-type: basic → native APISIX auth-type annotation (not PluginConfig)
	if out.Metadata.Annotations["k8s.apisix.apache.org/auth-type"] != "basicAuth" {
		t.Errorf("expected auth-type=basicAuth, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/auth-type"])
	}
	// No PluginConfig needed for basic auth with native annotation
	if len(result.PluginConfigs) != 0 {
		t.Errorf("expected 0 PluginConfigs for basic auth (uses native annotation), got %d",
			len(result.PluginConfigs))
	}
}

func TestConvert_BackendProtocol(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("grpc-ingress", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("grpc.app.com", "/", "svc", 50051)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/upstream-scheme"] != "grpc" {
		t.Errorf("expected upstream-scheme=grpc, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/upstream-scheme"])
	}
}

func TestConvert_Whitelist(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("wl-test", "default",
		map[string]string{
			"ingress.kubernetes.io/whitelist-source-range": "10.20.0.0/24,192.168.10.0/24",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/allowlist-source-range"] != "10.20.0.0/24,192.168.10.0/24" {
		t.Errorf("expected allowlist-source-range, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/allowlist-source-range"])
	}
}

func TestConvert_AuthUrl(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("auth-url-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/auth-url":              "http://oath-gateway.ems.svc.cluster.local/decisions$request_uri",
			"nginx.ingress.kubernetes.io/auth-response-headers": "Authorization",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/auth-uri"] != "http://oath-gateway.ems.svc.cluster.local/decisions$request_uri" {
		t.Errorf("expected auth-uri, got '%s'", out.Metadata.Annotations["k8s.apisix.apache.org/auth-uri"])
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/auth-upstream-headers"] != "Authorization" {
		t.Errorf("expected auth-upstream-headers=Authorization, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/auth-upstream-headers"])
	}
	if _, exists := out.Metadata.Annotations["k8s.apisix.apache.org/auth-request-headers"]; exists {
		t.Errorf("should not auto-set auth-request-headers, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/auth-request-headers"])
	}
}

func TestConvert_AuthRequestHeaders_ConvertsToAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("auth-headers-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/auth-url":             "http://auth-svc/verify",
			"nginx.ingress.kubernetes.io/auth-request-headers": "Authorization,X-Custom-Token",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/auth-uri"] != "http://auth-svc/verify" {
		t.Errorf("expected auth-uri, got '%s'", out.Metadata.Annotations["k8s.apisix.apache.org/auth-uri"])
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/auth-request-headers"] != "Authorization,X-Custom-Token" {
		t.Errorf("expected auth-request-headers=Authorization,X-Custom-Token, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/auth-request-headers"])
	}
}

func TestConvert_PathType_ImplementationSpecific_ToPrefix(t *testing.T) {
	c := newTestConverter()

	// Non-regex path: ImplementationSpecific should become Prefix
	ing := makeTestIngress("path-test", "default", nil, nil,
		[]ingress.IngressRule{
			{
				Host: "app.com",
				HTTP: &ingress.HTTPIngressRuleValue{
					Paths: []ingress.HTTPIngressPath{
						{
							Path:     "/api",
							PathType: strPtr("ImplementationSpecific"),
							Backend: ingress.IngressBackend{
								Service: &ingress.IngressServiceBackend{
									Name: "svc",
									Port: ingress.ServiceBackendPort{Number: 80},
								},
							},
						},
					},
				},
			},
		},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if *out.Spec.Rules[0].HTTP.Paths[0].PathType != "Prefix" {
		t.Errorf("expected ImplementationSpecific → Prefix, got '%s'",
			*out.Spec.Rules[0].HTTP.Paths[0].PathType)
	}
}

func TestConvert_PathType_ImplementationSpecific_WithRegex_Stays(t *testing.T) {
	c := newTestConverter()

	// Regex path: ImplementationSpecific should stay ImplementationSpecific + use-regex
	ing := makeTestIngress("path-regex", "default", nil, nil,
		[]ingress.IngressRule{
			{
				Host: "app.com",
				HTTP: &ingress.HTTPIngressRuleValue{
					Paths: []ingress.HTTPIngressPath{
						{
							Path:     "/api(/|$)(.*)",
							PathType: strPtr("ImplementationSpecific"),
							Backend: ingress.IngressBackend{
								Service: &ingress.IngressServiceBackend{
									Name: "svc",
									Port: ingress.ServiceBackendPort{Number: 80},
								},
							},
						},
					},
				},
			},
		},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if *out.Spec.Rules[0].HTTP.Paths[0].PathType != "ImplementationSpecific" {
		t.Errorf("expected regex path to stay ImplementationSpecific, got '%s'",
			*out.Spec.Rules[0].HTTP.Paths[0].PathType)
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/use-regex"] != "true" {
		t.Errorf("expected use-regex annotation 'true', got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/use-regex"])
	}
}

func TestConvert_PathType_Prefix_WithRegex_BecomesImplementationSpecific(t *testing.T) {
	c := newTestConverter()

	// Regex path with Prefix → should become ImplementationSpecific + use-regex
	ing := makeTestIngress("path-regex-prefix", "default", nil, nil,
		[]ingress.IngressRule{
			{
				Host: "app.com",
				HTTP: &ingress.HTTPIngressRuleValue{
					Paths: []ingress.HTTPIngressPath{
						{
							Path:     "/api(/|$)(.*)",
							PathType: strPtr("Prefix"),
							Backend: ingress.IngressBackend{
								Service: &ingress.IngressServiceBackend{
									Name: "svc",
									Port: ingress.ServiceBackendPort{Number: 80},
								},
							},
						},
					},
				},
			},
		},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if *out.Spec.Rules[0].HTTP.Paths[0].PathType != "ImplementationSpecific" {
		t.Errorf("expected regex+Prefix → ImplementationSpecific, got '%s'",
			*out.Spec.Rules[0].HTTP.Paths[0].PathType)
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/use-regex"] != "true" {
		t.Errorf("expected use-regex annotation 'true', got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/use-regex"])
	}
}

func TestConvert_PathType_Prefix_NoRegex_StaysPrefix(t *testing.T) {
	c := newTestConverter()

	// Non-regex path with Prefix → should stay Prefix, no use-regex
	ing := makeTestIngress("path-prefix-no-regex", "default", nil, nil,
		[]ingress.IngressRule{
			{
				Host: "app.com",
				HTTP: &ingress.HTTPIngressRuleValue{
					Paths: []ingress.HTTPIngressPath{
						{
							Path:     "/api",
							PathType: strPtr("Prefix"),
							Backend: ingress.IngressBackend{
								Service: &ingress.IngressServiceBackend{
									Name: "svc",
									Port: ingress.ServiceBackendPort{Number: 80},
								},
							},
						},
					},
				},
			},
		},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if *out.Spec.Rules[0].HTTP.Paths[0].PathType != "Prefix" {
		t.Errorf("expected non-regex Prefix to stay Prefix, got '%s'",
			*out.Spec.Rules[0].HTTP.Paths[0].PathType)
	}
	if v, ok := out.Metadata.Annotations["k8s.apisix.apache.org/use-regex"]; ok {
		t.Errorf("expected no use-regex annotation, got '%s'", v)
	}
}

func TestConvert_PathType_ImplementationSpecific_NoRegex_BecomesPrefix(t *testing.T) {
	c := newTestConverter()

	// Non-regex path with ImplementationSpecific → Prefix, no use-regex
	ing := makeTestIngress("path-is-no-regex", "default", nil, nil,
		[]ingress.IngressRule{
			{
				Host: "app.com",
				HTTP: &ingress.HTTPIngressRuleValue{
					Paths: []ingress.HTTPIngressPath{
						{
							Path:     "/api",
							PathType: strPtr("ImplementationSpecific"),
							Backend: ingress.IngressBackend{
								Service: &ingress.IngressServiceBackend{
									Name: "svc",
									Port: ingress.ServiceBackendPort{Number: 80},
								},
							},
						},
					},
				},
			},
		},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if *out.Spec.Rules[0].HTTP.Paths[0].PathType != "Prefix" {
		t.Errorf("expected ImplementationSpecific → Prefix, got '%s'",
			*out.Spec.Rules[0].HTTP.Paths[0].PathType)
	}
	if v, ok := out.Metadata.Annotations["k8s.apisix.apache.org/use-regex"]; ok {
		t.Errorf("expected no use-regex annotation, got '%s'", v)
	}
}

func TestConvert_NoAnnotations_ProducesNoPluginConfig(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("plain-test", "default", nil, nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)})

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 0 {
		t.Errorf("expected 0 PluginConfigs, got %d", len(result.PluginConfigs))
	}
}

func TestConvert_PreservesNonNginxAnnotations(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("preserve-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors": "true",
			"custom.io/my-annotation":                 "keep-me",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["custom.io/my-annotation"] != "keep-me" {
		t.Error("expected non-nginx annotation to be preserved")
	}
}

func TestConvert_ManagedByLabel(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("label-test", "default", nil, nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)})

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Labels["managed-by"] != "ingress2apisix" {
		t.Error("expected managed-by label")
	}
}

func TestConvert_DefaultNamespace(t *testing.T) {
	opts := apisix.DefaultConversionOptions()
	opts.DefaultNamespace = "my-namespace"
	c := New(opts)

	ing := ingress.Ingress{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "Ingress",
		Metadata:   ingress.Metadata{Name: "no-ns-ingress"},
		Spec: ingress.IngressSpec{
			Rules: []ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
		},
	}

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Namespace != "my-namespace" {
		t.Errorf("expected namespace 'my-namespace', got '%s'", out.Metadata.Namespace)
	}
}

func TestConvert_TargetIngressClass(t *testing.T) {
	opts := apisix.DefaultConversionOptions()
	opts.TargetIngressClassName = "my-apisix"
	c := New(opts)

	ing := makeTestIngress("class-test", "default", nil, nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)})

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if *out.Spec.IngressClassName != "my-apisix" {
		t.Errorf("expected ingressClassName 'my-apisix', got '%s'", *out.Spec.IngressClassName)
	}
}

// --- Dual-prefix (ingress.kubernetes.io/) Tests ---

func TestConvert_PlainPrefix_CORS(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cors-plain", "default",
		map[string]string{
			"ingress.kubernetes.io/enable-cors": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// CORS is handled by ApisixPluginConfig, not AIC annotations
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig for CORS, got %d", len(result.PluginConfigs))
	}
	if _, ok := out.Metadata.Annotations["ingress.kubernetes.io/enable-cors"]; ok {
		t.Error("original ingress.kubernetes.io annotation should be removed")
	}
}

func TestConvert_PlainPrefix_ForceSSLRedirect(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("force-ssl-plain", "default",
		map[string]string{
			"ingress.kubernetes.io/force-ssl-redirect": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/http-to-https"] != "true" {
		t.Error("expected http-to-https from ingress.kubernetes.io/force-ssl-redirect")
	}
}

func TestConvert_MixedPrefixes_NginxTakesPriority(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("mixed-priority", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rps": "50",
			"ingress.kubernetes.io/limit-rps":       "999",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	rate, ok := cfg["rate"].(float64)
	if !ok {
		t.Fatalf("expected rate to be float64, got %T: %v", cfg["rate"], cfg["rate"])
	}
	if rate != 50 {
		t.Errorf("expected nginx prefix to take priority, got rate=%v", rate)
	}
}

func TestConvert_MixedPrefixes_SameResource(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("mixed-same", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors":      "true",
			"ingress.kubernetes.io/force-ssl-redirect":     "true",
			"ingress.kubernetes.io/backend-protocol":       "HTTPS",
			"ingress.kubernetes.io/whitelist-source-range": "10.0.0.0/8",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/enable-cors"] != "" {
		t.Error("cors AIC annotations should NOT be set (handled by ApisixPluginConfig)")
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/http-to-https"] != "true" {
		t.Error("expected http-to-https from plain prefix")
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/upstream-scheme"] != "https" {
		t.Error("expected upstream-scheme from plain prefix")
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/allowlist-source-range"] != "10.0.0.0/8" {
		t.Error("expected allowlist-source-range from plain prefix")
	}

	// Both prefixes should be stripped from output
	for _, k := range []string{
		"nginx.ingress.kubernetes.io/enable-cors",
		"ingress.kubernetes.io/force-ssl-redirect",
		"ingress.kubernetes.io/backend-protocol",
	} {
		if _, ok := out.Metadata.Annotations[k]; ok {
			t.Errorf("original annotation %q should be removed from output", k)
		}
	}
}

// --- WriteConversionResult Tests ---

func TestWriteConversionResult_RoundTrip(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("rt-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors": "true",
			"nginx.ingress.kubernetes.io/limit-rps":   "100",
		},
		[]ingress.IngressTLS{
			{Hosts: []string{"rt.com"}, SecretName: "rt-sec"},
		},
		[]ingress.IngressRule{makeSimpleRule("rt.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty output")
	}

	if !bytes.Contains(buf.Bytes(), []byte("kind: Ingress")) {
		t.Error("output should contain Ingress kind")
	}
	if !bytes.Contains(buf.Bytes(), []byte("kind: ApisixPluginConfig")) {
		t.Error("output should contain ApisixPluginConfig kind")
	}
	// CORS is now fully handled by ApisixPluginConfig (cors plugin), not AIC annotations
	if !bytes.Contains(buf.Bytes(), []byte("k8s.apisix.apache.org/plugin-config-name")) {
		t.Error("output should contain plugin-config-name annotation linking PluginConfig")
	}
	if containsNonComment(buf.Bytes(), []byte("nginx.ingress.kubernetes.io/")) {
		t.Error("output should not contain any nginx annotations (only in source comments)")
	}
	if containsNonComment(buf.Bytes(), []byte("ingress.kubernetes.io/")) {
		t.Error("output should not contain any ingress.kubernetes.io annotations (only in source comments)")
	}
}

// --- Integration Test ---

func TestIntegration_ExampleFile(t *testing.T) {
	c := newTestConverter()
	input, err := ReadIngressFile("../../examples/ingress.yaml")
	if err != nil {
		t.Fatalf("error reading example file: %v", err)
	}

	if len(input.Ingresses) < 1 {
		t.Fatal("expected at least 1 ingress from example file")
	}

	result := c.ConvertList(input)

	if len(result.Ingresses) != len(input.Ingresses) {
		t.Fatalf("expected %d converted ingresses, got %d", len(input.Ingresses), len(result.Ingresses))
	}

	var buf bytes.Buffer
	err = WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("error writing output: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected non-empty output")
	}
}

// --- Helper tests ---

func TestEnsureTimeSuffix(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"30", "30s"},
		{"3600", "3600s"},
		{"3600s", "3600s"},
		{"500ms", "500ms"},
		{" 60 ", "60s"},
		{"", ""},
	}
	for _, tt := range tests {
		got := ensureTimeSuffix(tt.input)
		if got != tt.want {
			t.Errorf("ensureTimeSuffix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractRewriteURIs(t *testing.T) {
	c := newTestConverter()

	tests := []struct {
		snippet string
		want    int // number of URI pairs
	}{
		{"rewrite ^/shard/0/(.*) /$1 break;", 1},
		{"rewrite ^/iam/(.*) /$1 break;", 1},
		{"rewrite ^/shard/0/(.*) /$1 break;rewrite ^/am/(.*) /$1 break;", 2},
		{"proxy_cookie_flags sessionid SameSite=None Secure;", 0},
		{"", 0},
		{"some random text", 0},
	}
	for _, tt := range tests {
		uris := c.extractRewriteURIs(tt.snippet)
		got := len(uris) / 2
		if got != tt.want {
			t.Errorf("extractRewriteURIs(%q): got %d pairs, want %d", tt.snippet, got, tt.want)
		}
	}
}

func TestPathHasRegex(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api", false},
		{"/", false},
		{"", false},
		{"/api(/|$)(.*)", true},
		{"/api/[^/]+/.*", true},
		{"/static/images", false},
	}
	for _, tt := range tests {
		got := pathHasRegex(tt.path)
		if got != tt.want {
			t.Errorf("pathHasRegex(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// --- Warning Tests ---

func TestWarning_CustomHTTPErrors(t *testing.T) {
	// custom-http-errors is now auto-converted to custom-error-codes annotation,
	// so no warning should be generated.
	c := newTestConverter()
	ing := makeTestIngress("err-ingress", "default",
		map[string]string{
			"ingress.kubernetes.io/custom-http-errors": "404,500",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	for _, w := range result.Warnings {
		if strings.Contains(w, "custom-http-errors") {
			t.Errorf("unexpected warning for custom-http-errors (should be auto-converted): %s", w)
		}
	}

	out := result.Ingresses[0].(ingress.Ingress)
	if out.Metadata.Annotations["k8s.apisix.apache.org/custom-error-codes"] != "404,500" {
		t.Errorf("expected custom-error-codes=404,500, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/custom-error-codes"])
	}
}

func TestWarning_CookieFlags(t *testing.T) {
	// proxy_cookie_flags is now auto-converted to proxy-cookie-flags plugin,
	// so no warning should be generated.
	c := newTestConverter()
	ing := makeTestIngress("cookie-ingress", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": "proxy_cookie_flags sessionid SameSite=None Secure;",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	for _, w := range result.Warnings {
		if strings.Contains(w, "proxy_cookie_flags") {
			t.Errorf("proxy_cookie_flags should no longer generate a warning, got: %v", w)
		}
	}

	// Should produce a PluginConfig with proxy-cookie-flags plugin
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	if result.PluginConfigs[0].Spec.Plugins[0].Name != "proxy-cookie-flags" {
		t.Errorf("expected proxy-cookie-flags plugin, got '%s'",
			result.PluginConfigs[0].Spec.Plugins[0].Name)
	}
}

func TestConvert_SessionAffinity_ProducesBackendTrafficPolicy(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("affinity-ingress", "default",
		map[string]string{
			"ingress.kubernetes.io/affinity":            "cookie",
			"ingress.kubernetes.io/session-cookie-name": "escookie",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.BackendTrafficPolicies) != 1 {
		t.Fatalf("expected 1 BackendTrafficPolicy, got %d", len(result.BackendTrafficPolicies))
	}
	btp := result.BackendTrafficPolicies[0]
	if btp.Kind != "BackendTrafficPolicy" {
		t.Fatalf("expected BackendTrafficPolicy, got %s", btp.Kind)
	}
	if btp.Metadata.Name != "affinity-ingress-cookie-affinity" {
		t.Errorf("expected generated policy name, got %s", btp.Metadata.Name)
	}
	if btp.Spec.LoadBalancer.Type != "chash" {
		t.Errorf("expected loadbalancer type chash, got %s", btp.Spec.LoadBalancer.Type)
	}
	if btp.Spec.LoadBalancer.HashOn != "cookie" {
		t.Errorf("expected hashOn cookie, got %s", btp.Spec.LoadBalancer.HashOn)
	}
	if btp.Spec.LoadBalancer.Key != "escookie" {
		t.Errorf("expected key escookie, got %s", btp.Spec.LoadBalancer.Key)
	}
	if len(btp.Spec.TargetRefs) != 1 || btp.Spec.TargetRefs[0].Name != "svc" {
		t.Fatalf("expected targetRef for service svc, got %#v", btp.Spec.TargetRefs)
	}
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected default session-cookie-hash PluginConfig, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["cookie_name"] != "escookie" {
		t.Errorf("expected cookie_name=escookie, got %v", cfg["cookie_name"])
	}
	if cfg["algorithm"] != "sha1" {
		t.Errorf("expected default algorithm=sha1, got %v", cfg["algorithm"])
	}

	hashWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "affinity") && strings.Contains(w, "无法自动转换") {
			t.Fatalf("session affinity should be converted without manual warning, got: %s", w)
		}
		if strings.Contains(w, "session-cookie-hash") && strings.Contains(w, "默认使用 sha1") {
			hashWarn = true
		}
	}
	if !hashWarn {
		t.Fatalf("expected warning about default sha1, got: %v", result.Warnings)
	}
}

func TestConvert_CookieAffinityOnly_WarnsAboutAmbiguousCookie(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("affinity-only", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/affinity": "cookie",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.BackendTrafficPolicies) != 1 {
		t.Fatalf("expected 1 BackendTrafficPolicy, got %d", len(result.BackendTrafficPolicies))
	}
	if result.BackendTrafficPolicies[0].Spec.LoadBalancer.Key != "INGRESSCOOKIE" {
		t.Fatalf("expected default key INGRESSCOOKIE, got %s", result.BackendTrafficPolicies[0].Spec.LoadBalancer.Key)
	}
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected default session-cookie-hash PluginConfig, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["cookie_name"] != "INGRESSCOOKIE" {
		t.Errorf("expected default cookie_name=INGRESSCOOKIE, got %v", cfg["cookie_name"])
	}
	if cfg["algorithm"] != "sha1" {
		t.Errorf("expected default algorithm=sha1, got %v", cfg["algorithm"])
	}

	nameWarn := false
	hashWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "session-cookie-name") && strings.Contains(w, "INGRESSCOOKIE") {
			nameWarn = true
		}
		if strings.Contains(w, "session-cookie-hash") && strings.Contains(w, "默认使用 sha1") {
			hashWarn = true
		}
	}
	if !nameWarn {
		t.Fatalf("expected warning about missing session-cookie-name, got: %v", result.Warnings)
	}
	if !hashWarn {
		t.Fatalf("expected warning about missing session-cookie-hash, got: %v", result.Warnings)
	}
}

func TestConvert_SessionCookieHash_ProducesCustomPlugin(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("session-hash", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/session-cookie-hash": "sha1",
			"nginx.ingress.kubernetes.io/session-cookie-name": "escookie",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	pc := result.PluginConfigs[0]
	if len(pc.Spec.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(pc.Spec.Plugins))
	}
	p := pc.Spec.Plugins[0]
	if p.Name != "session-cookie-hash" {
		t.Fatalf("expected session-cookie-hash plugin, got %s", p.Name)
	}
	cfg, ok := p.Config.(map[string]interface{})
	if !ok {
		t.Fatal("expected plugin config to be a map")
	}
	if cfg["cookie_name"] != "escookie" {
		t.Errorf("expected cookie_name=escookie, got %v", cfg["cookie_name"])
	}
	if cfg["algorithm"] != "sha1" {
		t.Errorf("expected algorithm=sha1, got %v", cfg["algorithm"])
	}
	if cfg["header_name"] != "X-Session-Hash" {
		t.Errorf("expected header_name=X-Session-Hash, got %v", cfg["header_name"])
	}
	if cfg["fallback"] != "pass" {
		t.Errorf("expected fallback=pass, got %v", cfg["fallback"])
	}
	if cfg["generate_cookie"] != true {
		t.Errorf("expected generate_cookie=true, got %v", cfg["generate_cookie"])
	}
	if _, ok := cfg["cookie_path"]; ok {
		t.Errorf("cookie_path should be omitted by default, got %v", cfg["cookie_path"])
	}
	if cfg["cookie_httponly"] != false {
		t.Errorf("expected cookie_httponly=false, got %v", cfg["cookie_httponly"])
	}

	out := result.Ingresses[0].(ingress.Ingress)
	if out.Metadata.Annotations["k8s.apisix.apache.org/plugin-config-name"] != pc.Metadata.Name {
		t.Error("session-cookie-hash PluginConfig should be linked via annotation")
	}
}

func TestConvert_CookieAffinityWithSessionCookieHash_DoesNotInventUpstreamHashAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cookie-affinity", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/affinity":            "cookie",
			"nginx.ingress.kubernetes.io/session-cookie-name": "route",
			"nginx.ingress.kubernetes.io/session-cookie-hash": "sha1",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	p := result.PluginConfigs[0].Spec.Plugins[0]
	if p.Name != "session-cookie-hash" {
		t.Fatalf("expected session-cookie-hash plugin, got %s", p.Name)
	}
	cfg, ok := p.Config.(map[string]interface{})
	if !ok {
		t.Fatal("expected plugin config to be a map")
	}
	if cfg["cookie_name"] != "route" {
		t.Errorf("expected cookie_name=route, got %v", cfg["cookie_name"])
	}
	if cfg["algorithm"] != "sha1" {
		t.Errorf("expected algorithm=sha1, got %v", cfg["algorithm"])
	}

	out := result.Ingresses[0].(ingress.Ingress)
	if _, ok := out.Metadata.Annotations["k8s.apisix.apache.org/upstream-hash"]; ok {
		t.Fatal("must not emit nonexistent k8s.apisix.apache.org/upstream-hash annotation")
	}
	if len(result.BackendTrafficPolicies) != 1 {
		t.Fatalf("expected 1 BackendTrafficPolicy, got %d", len(result.BackendTrafficPolicies))
	}
	if result.BackendTrafficPolicies[0].Spec.LoadBalancer.Key != "route" {
		t.Fatalf("expected BackendTrafficPolicy key=route, got %s", result.BackendTrafficPolicies[0].Spec.LoadBalancer.Key)
	}
	for _, w := range result.Warnings {
		if strings.Contains(w, "affinity") && strings.Contains(w, "无法自动转换") {
			t.Fatalf("affinity should be converted to BackendTrafficPolicy, got: %s", w)
		}
		if strings.Contains(w, "session-cookie-name") && strings.Contains(w, "无法自动转换") {
			t.Fatalf("session-cookie-name should be consumed by session-cookie-hash plugin config, got: %s", w)
		}
	}
}

func TestConvert_UpstreamHashBy_DoesNotInventAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("upstream-hash-by", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/upstream-hash-by": "$request_uri",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)
	if _, ok := out.Metadata.Annotations["k8s.apisix.apache.org/upstream-hash"]; ok {
		t.Fatal("must not emit nonexistent k8s.apisix.apache.org/upstream-hash annotation")
	}

	// upstream-hash-by is now auto-converted to BackendTrafficPolicy
	if len(result.BackendTrafficPolicies) != 1 {
		t.Fatalf("expected 1 BackendTrafficPolicy for upstream-hash-by, got %d", len(result.BackendTrafficPolicies))
	}
	btp := result.BackendTrafficPolicies[0]
	if btp.Spec.LoadBalancer.Type != "chash" {
		t.Errorf("expected chash, got %s", btp.Spec.LoadBalancer.Type)
	}

	// Should NOT have a manual warning for upstream-hash-by
	for _, w := range result.Warnings {
		if strings.Contains(w, "upstream-hash-by") && strings.Contains(w, "无法自动转换") {
			t.Fatalf("upstream-hash-by should now be auto-converted, got: %s", w)
		}
	}
}

func TestConvert_SessionCookieHash_DefaultCookieNameWarning(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("session-hash-default-cookie", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/session-cookie-hash": "sha256",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	p := result.PluginConfigs[0].Spec.Plugins[0]
	cfg, ok := p.Config.(map[string]interface{})
	if !ok {
		t.Fatal("expected plugin config to be a map")
	}
	if cfg["cookie_name"] != "INGRESSCOOKIE" {
		t.Errorf("expected default cookie_name=INGRESSCOOKIE, got %v", cfg["cookie_name"])
	}
	if cfg["algorithm"] != "sha256" {
		t.Errorf("expected algorithm=sha256, got %v", cfg["algorithm"])
	}

	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "session-cookie-name") && strings.Contains(w, "INGRESSCOOKIE") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected warning about missing session-cookie-name, got: %v", result.Warnings)
	}
}

func TestConvert_SessionCookieHash_UnsupportedAlgoWarning(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("session-hash-unsupported", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/session-cookie-hash": "crc32",
			"nginx.ingress.kubernetes.io/session-cookie-name": "escookie",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	if len(result.PluginConfigs) != 0 {
		t.Fatalf("expected 0 PluginConfig for unsupported algorithm, got %d", len(result.PluginConfigs))
	}

	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "session-cookie-hash") && strings.Contains(w, "sha1/md5/sha256") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected warning about unsupported session-cookie-hash value, got: %v", result.Warnings)
	}
}

func TestConvert_SessionCookieName_NoWarningWhenHashExists(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("session-cookie-name-no-manual-warning", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/session-cookie-name": "escookie",
			"nginx.ingress.kubernetes.io/session-cookie-hash": "md5",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	for _, w := range result.Warnings {
		if strings.Contains(w, "session-cookie-name") && strings.Contains(w, "无法自动转换") {
			t.Fatalf("session-cookie-name should be consumed by session-cookie-hash conversion, got warning: %s", w)
		}
	}
}

func TestWarning_ProxyBodySize(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("body-ingress", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/proxy-body-size": "0",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	// proxy-body-size is now auto-converted to client-control plugin, no warning expected
	for _, w := range result.Warnings {
		if strings.Contains(w, "proxy-body-size") {
			t.Errorf("proxy-body-size should no longer generate a warning, got: %v", w)
		}
	}

	// Should produce a PluginConfig with client-control plugin (0 bytes)
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	p := result.PluginConfigs[0].Spec.Plugins[0]
	if p.Name != "client-control" {
		t.Errorf("expected client-control plugin, got '%s'", p.Name)
	}
	cfg := p.Config.(map[string]interface{})
	if cfg["max_body_size"] != int64(0) {
		t.Errorf("expected max_body_size=0, got %v", cfg["max_body_size"])
	}
}

func TestWarning_MTLS(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("mtls-ingress", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/auth-tls-secret":        "openstack/client-ca",
			"nginx.ingress.kubernetes.io/auth-tls-verify-client": "on",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	secretWarn := false
	verifyWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "auth-tls-secret") {
			secretWarn = true
		}
		if strings.Contains(w, "auth-tls-verify-client") {
			verifyWarn = true
		}
	}
	if !secretWarn {
		t.Errorf("expected warning for auth-tls-secret, got warnings: %v", result.Warnings)
	}
	if !verifyWarn {
		t.Errorf("expected warning for auth-tls-verify-client, got warnings: %v", result.Warnings)
	}
}

func TestWarning_UnrecognizedAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("unknown-ingress", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/some-future-annotation": "some-value",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "some-future-annotation") && strings.Contains(w, "未被识别") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for unrecognized annotation, got warnings: %v", result.Warnings)
	}
}

func TestWarning_NoWarningsForHandledAnnotations(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("clean-ingress", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors":        "true",
			"nginx.ingress.kubernetes.io/proxy-read-timeout": "300",
			"nginx.ingress.kubernetes.io/rewrite-target":     "/",
			"ingress.kubernetes.io/whitelist-source-range":   "10.0.0.0/8",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings for fully handled annotations, got: %v", result.Warnings)
	}
}

func TestWarning_MoreSetHeaders(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("headers-ingress", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": `more_set_headers "X-Forwarded-For: $http_x_forwarded_for";`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	// more_set_headers is now auto-converted to response-rewrite plugin (no warning expected)
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	pc := result.PluginConfigs[0]
	var rwPlugin *apisix.Plugin
	for i := range pc.Spec.Plugins {
		if pc.Spec.Plugins[i].Name == "response-rewrite" {
			rwPlugin = &pc.Spec.Plugins[i]
			break
		}
	}
	if rwPlugin == nil {
		t.Fatal("expected response-rewrite plugin for more_set_headers")
	}

	cfg := rwPlugin.Config.(map[string]interface{})
	headers := cfg["headers"].(map[string]interface{})
	setHeaders := headers["set"].(map[string]string)
	if setHeaders["X-Forwarded-For"] != "$http_x_forwarded_for" {
		t.Errorf("expected X-Forwarded-For=$http_x_forwarded_for, got '%s'", setHeaders["X-Forwarded-For"])
	}

	// No warning about more_set_headers
	for _, w := range result.Warnings {
		if strings.Contains(w, "more_set_headers") {
			t.Errorf("should not warn about more_set_headers, got: %v", w)
		}
	}
}

func TestConvert_AuthSecret_ProducesWarning(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("auth-secret-ingress", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/auth-type":   "basic",
			"nginx.ingress.kubernetes.io/auth-secret": "my-basic-auth",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// auth-secret is NOT supported by AIC, should NOT be passed through
	if out.Metadata.Annotations["k8s.apisix.apache.org/auth-secret"] != "" {
		t.Errorf("auth-secret should NOT be passed through (AIC doesn't support it), got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/auth-secret"])
	}

	// Should produce a warning about auth-secret
	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "auth-secret") {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Error("expected a warning about auth-secret not being supported")
	}
}

// --- PluginConfig Linkage Tests ---

func TestConvert_PluginConfigLinked(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("linked-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rps": "100",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	out := result.Ingresses[0].(ingress.Ingress)
	expected := result.PluginConfigs[0].Metadata.Name
	if out.Metadata.Annotations["k8s.apisix.apache.org/plugin-config-name"] != expected {
		t.Errorf("expected plugin-config-name=%q, got %q",
			expected,
			out.Metadata.Annotations["k8s.apisix.apache.org/plugin-config-name"])
	}
}

func TestConvert_CORS_AlwaysCreatesPluginConfig(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("no-link", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig for CORS, got %d", len(result.PluginConfigs))
	}

	out := result.Ingresses[0].(ingress.Ingress)
	expected := result.PluginConfigs[0].Metadata.Name
	if out.Metadata.Annotations["k8s.apisix.apache.org/plugin-config-name"] != expected {
		t.Errorf("expected plugin-config-name=%q, got %q",
			expected,
			out.Metadata.Annotations["k8s.apisix.apache.org/plugin-config-name"])
	}

	// Verify CORS plugin config has ingress-nginx defaults
	corsPlugin := result.PluginConfigs[0].Spec.Plugins[0]
	if corsPlugin.Name != "cors" {
		t.Fatalf("expected cors plugin, got %s", corsPlugin.Name)
	}
	cfg := corsPlugin.Config.(map[string]interface{})
	if cfg["allow_credential"] != true {
		t.Errorf("expected allow_credential=true, got %v", cfg["allow_credential"])
	}
	if cfg["max_age"] != 1728000 {
		t.Errorf("expected max_age=1728000, got %v", cfg["max_age"])
	}
	if cfg["allow_methods"] != "GET, PUT, POST, DELETE, PATCH, OPTIONS" {
		t.Errorf("expected ingress-nginx default methods, got %v", cfg["allow_methods"])
	}
	if cfg["allow_headers"] != "DNT,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Range,Authorization" {
		t.Errorf("expected ingress-nginx default headers, got %v", cfg["allow_headers"])
	}
	// With credentials + wildcard origin, should use allow_origins_by_regex
	if _, ok := cfg["allow_origins_by_regex"]; !ok {
		t.Error("expected allow_origins_by_regex with credentials + wildcard origin")
	}
	if _, ok := cfg["allow_origins"]; ok {
		t.Error("should not set allow_origins when using allow_origins_by_regex")
	}
}

// --- Proxy Redirect Tests ---

func TestConvert_ProxyRedirect_SSL(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("redirect-ssl", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/proxy-redirect-from": "http://",
			"nginx.ingress.kubernetes.io/proxy-redirect-to":   "https://",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/http-to-https"] != "true" {
		t.Error("expected http-to-https from proxy-redirect http://→https://")
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings for SSL redirect, got: %v", result.Warnings)
	}
}

func TestConvert_ProxyRedirect_NonSSL_Warning(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("redirect-custom", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/proxy-redirect-from": "http://old.example.com",
			"nginx.ingress.kubernetes.io/proxy-redirect-to":   "https://new.example.com",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "proxy-redirect-from") && strings.Contains(w, "无法自动转换") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for non-SSL proxy-redirect, got warnings: %v", result.Warnings)
	}
}

// --- CORS max-age Removal Test ---

func TestConvert_CORS_DefaultMaxAge(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cors-nomax", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// cors-max-age is NOT a native APISIX annotation, should not be set
	if _, ok := out.Metadata.Annotations["k8s.apisix.apache.org/cors-max-age"]; ok {
		t.Error("cors-max-age is not a native APISIX annotation and should not be set")
	}

	// PluginConfig should be created with ingress-nginx default max_age
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig for CORS, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["max_age"] != 1728000 {
		t.Errorf("expected default max_age=1728000, got %v", cfg["max_age"])
	}
}

// --- Additional CORS Tests ---

func TestConvert_CORS_ExplicitCredentialsFalse(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cors-creds-false", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors":            "true",
			"nginx.ingress.kubernetes.io/cors-allow-credentials": "false",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig for CORS, got %d", len(result.PluginConfigs))
	}

	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["allow_credential"] != false {
		t.Errorf("expected allow_credential=false, got %v", cfg["allow_credential"])
	}
	// Without credentials, allow_origins="*" is valid in APISIX
	if v, ok := cfg["allow_origins"]; !ok || v != "*" {
		t.Errorf("expected allow_origins=\"*\", got %v", v)
	}
	if _, ok := cfg["allow_origins_by_regex"]; ok {
		t.Error("should not use allow_origins_by_regex when credentials are disabled")
	}
}

func TestConvert_CORS_ExplicitMaxAge(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cors-explicit-max", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors":  "true",
			"nginx.ingress.kubernetes.io/cors-max-age": "600",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["max_age"] != 600 {
		t.Errorf("expected max_age=600, got %v", cfg["max_age"])
	}
}

func TestConvert_CORS_ExposeHeaders(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cors-expose", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors":         "true",
			"nginx.ingress.kubernetes.io/cors-expose-headers": "X-Custom-Header,X-Another",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["expose_headers"] != "X-Custom-Header,X-Another" {
		t.Errorf("expected expose_headers=\"X-Custom-Header,X-Another\", got %v", cfg["expose_headers"])
	}
}

func TestConvert_CORS_ExplicitOrigin_NoRegex(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cors-explicit-origin", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors":       "true",
			"nginx.ingress.kubernetes.io/cors-allow-origin": "https://example.com",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	// With explicit origin + default credentials=true, should use allow_origins (not regex)
	if v, ok := cfg["allow_origins"]; !ok || v != "https://example.com" {
		t.Errorf("expected allow_origins=\"https://example.com\", got %v", v)
	}
	if _, ok := cfg["allow_origins_by_regex"]; ok {
		t.Error("should not use allow_origins_by_regex when origin is explicitly set")
	}
}

func TestConvert_CORS_FullCustom(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cors-full", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors":            "true",
			"nginx.ingress.kubernetes.io/cors-allow-origin":      "https://myapp.com",
			"nginx.ingress.kubernetes.io/cors-allow-methods":     "GET,POST",
			"nginx.ingress.kubernetes.io/cors-allow-headers":     "Authorization",
			"nginx.ingress.kubernetes.io/cors-allow-credentials": "true",
			"nginx.ingress.kubernetes.io/cors-max-age":           "3600",
			"nginx.ingress.kubernetes.io/cors-expose-headers":    "X-Total-Count",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["allow_origins"] != "https://myapp.com" {
		t.Errorf("expected allow_origins=\"https://myapp.com\", got %v", cfg["allow_origins"])
	}
	if cfg["allow_methods"] != "GET,POST" {
		t.Errorf("expected allow_methods=\"GET,POST\", got %v", cfg["allow_methods"])
	}
	if cfg["allow_headers"] != "Authorization" {
		t.Errorf("expected allow_headers=\"Authorization\", got %v", cfg["allow_headers"])
	}
	if cfg["allow_credential"] != true {
		t.Errorf("expected allow_credential=true, got %v", cfg["allow_credential"])
	}
	if cfg["max_age"] != 3600 {
		t.Errorf("expected max_age=3600, got %v", cfg["max_age"])
	}
	if cfg["expose_headers"] != "X-Total-Count" {
		t.Errorf("expected expose_headers=\"X-Total-Count\", got %v", cfg["expose_headers"])
	}
}

// --- Single Rewrite from configuration-snippet → proxy-rewrite plugin ---

func TestConvert_SingleRewrite_Snippet_NativeAnnotations(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("snippet-single", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": `rewrite ^/api/(.*) /$1 break;`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	// Single rewrite → proxy-rewrite plugin in PluginConfig
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	p := result.PluginConfigs[0].Spec.Plugins[0]
	if p.Name != "proxy-rewrite" {
		t.Fatalf("expected proxy-rewrite plugin, got %s", p.Name)
	}
	cfg := p.Config.(map[string]interface{})
	regexURI := cfg["regex_uri"].([]string)
	if len(regexURI) != 2 {
		t.Fatalf("expected 2 regex_uri entries, got %d", len(regexURI))
	}
	if regexURI[0] != "^/api/(.*)" {
		t.Errorf("expected regex_uri[0]='^/api/(.*)', got '%s'", regexURI[0])
	}
	if regexURI[1] != "/$1" {
		t.Errorf("expected regex_uri[1]='/$1', got '%s'", regexURI[1])
	}
}

func TestConvert_MultipleRewrites_Snippet_PluginConfig(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("snippet-multi", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": `rewrite ^/shard/0/(.*) /$1 break;rewrite ^/am/(.*) /$1 break;`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	// Multiple rewrites → PluginConfig with proxy-rewrite
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig for multi-rewrite, got %d", len(result.PluginConfigs))
	}
	if result.PluginConfigs[0].Spec.Plugins[0].Name != "proxy-rewrite" {
		t.Errorf("expected proxy-rewrite plugin, got '%s'",
			result.PluginConfigs[0].Spec.Plugins[0].Name)
	}

	// PluginConfig should be linked
	out := result.Ingresses[0].(ingress.Ingress)
	if out.Metadata.Annotations["k8s.apisix.apache.org/plugin-config-name"] != result.PluginConfigs[0].Metadata.Name {
		t.Error("multi-rewrite PluginConfig should be linked via annotation")
	}
}

func TestConvert_RewriteTargetAndSnippet_PluginConfig(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("rewrite-target-and-snippet", "openstack",
		map[string]string{
			"ingress.kubernetes.io/rewrite-target":        "/",
			"ingress.kubernetes.io/configuration-snippet": `rewrite ^/cinder_dashboard_api/(.*) /$1 break;`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/cinder_dashboard_api", "cinder-golem", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	p := result.PluginConfigs[0].Spec.Plugins[0]
	if p.Name != "proxy-rewrite" {
		t.Fatalf("expected proxy-rewrite plugin, got %s", p.Name)
	}
	cfg := p.Config.(map[string]interface{})
	regexURI := cfg["regex_uri"].([]string)
	expected := []string{
		"(?i)/cinder_dashboard_api", "/",
		"^/cinder_dashboard_api/(.*)", "/$1",
	}
	if len(regexURI) != len(expected) {
		t.Fatalf("expected regex_uri %v, got %v", expected, regexURI)
	}
	for i := range expected {
		if regexURI[i] != expected[i] {
			t.Fatalf("expected regex_uri %v, got %v", expected, regexURI)
		}
	}

	out := result.Ingresses[0].(ingress.Ingress)
	if _, ok := out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target"]; ok {
		t.Fatal("rewrite-target annotation should not be emitted when proxy-rewrite handles combined rewrites")
	}
	if _, ok := out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target-regex"]; ok {
		t.Fatal("rewrite-target-regex annotation should not be emitted when proxy-rewrite handles combined rewrites")
	}
}

// --- Auth-secret without auth-type (no double warning) ---

func TestConvert_AuthSecretWithoutAuthType_ProducesWarning(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("auth-secret-only", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/auth-secret": "my-secret",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// auth-secret is NOT supported by AIC
	if out.Metadata.Annotations["k8s.apisix.apache.org/auth-secret"] != "" {
		t.Errorf("auth-secret should NOT be passed through, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/auth-secret"])
	}

	// Should produce a warning about auth-secret
	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "auth-secret") {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Error("expected a warning about auth-secret not being supported")
	}
}

// --- kubernetes.io/ingress.class Removal ---

func TestConvert_IngressClassAnnotation_Nginx_Removed(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("class-annotation", "default",
		map[string]string{
			"kubernetes.io/ingress.class":             "nginx",
			"nginx.ingress.kubernetes.io/enable-cors": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if _, ok := out.Metadata.Annotations["kubernetes.io/ingress.class"]; ok {
		t.Error("kubernetes.io/ingress.class: nginx should be removed")
	}
	// CORS should still be converted via ApisixPluginConfig
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig for CORS, got %d", len(result.PluginConfigs))
	}
}

func TestConvert_IngressClassAnnotation_Nginx_CaseInsensitive(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("class-case", "default",
		map[string]string{
			"kubernetes.io/ingress.class": "Nginx",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if _, ok := out.Metadata.Annotations["kubernetes.io/ingress.class"]; ok {
		t.Error("kubernetes.io/ingress.class: Nginx (case insensitive) should be removed")
	}
}

func TestConvert_IngressClassAnnotation_Apisix_Preserved(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("class-apisix", "default",
		map[string]string{
			"kubernetes.io/ingress.class": "apisix",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// Non-nginx values should be preserved (user may have manually set this)
	if out.Metadata.Annotations["kubernetes.io/ingress.class"] != "apisix" {
		t.Error("kubernetes.io/ingress.class: apisix should be preserved")
	}
}

// --- proxy-cookie-flags Plugin Tests ---

func TestConvert_ProxyCookieFlags_SingleRule(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cookie-flags", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": "proxy_cookie_flags sessionid SameSite=None Secure;",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	pc := result.PluginConfigs[0]
	if len(pc.Spec.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(pc.Spec.Plugins))
	}
	p := pc.Spec.Plugins[0]
	if p.Name != "proxy-cookie-flags" {
		t.Errorf("expected proxy-cookie-flags plugin, got '%s'", p.Name)
	}
	if !p.Enable {
		t.Error("expected proxy-cookie-flags to be enabled")
	}

	cfg, ok := p.Config.(map[string]interface{})
	if !ok {
		t.Fatal("expected plugin config to be a map")
	}
	rules, ok := cfg["rules"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected rules to be []map[string]interface{}, got %T", cfg["rules"])
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0]["match"] != "sessionid" {
		t.Errorf("expected match='sessionid', got '%s'", rules[0]["match"])
	}
	flags, ok := rules[0]["flags"].([]string)
	if !ok {
		t.Fatalf("expected flags to be []string, got %T", rules[0]["flags"])
	}
	if len(flags) != 2 || flags[0] != "SameSite=None" || flags[1] != "Secure" {
		t.Errorf("expected flags=[SameSite=None, Secure], got %v", flags)
	}

	// PluginConfig should be linked to Ingress
	out := result.Ingresses[0].(ingress.Ingress)
	if out.Metadata.Annotations["k8s.apisix.apache.org/plugin-config-name"] != pc.Metadata.Name {
		t.Error("proxy-cookie-flags PluginConfig should be linked via annotation")
	}

	// No warnings
	for _, w := range result.Warnings {
		if strings.Contains(w, "proxy_cookie_flags") {
			t.Errorf("should not warn about proxy_cookie_flags, got: %v", w)
		}
	}
}

func TestConvert_ProxyCookieFlags_MultipleRules(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cookie-flags-multi", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": "proxy_cookie_flags sessionid SameSite=None Secure;\nproxy_cookie_flags * HttpOnly;",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	rules := cfg["rules"].([]map[string]interface{})
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}

	if rules[0]["match"] != "sessionid" {
		t.Errorf("expected first rule match='sessionid', got '%s'", rules[0]["match"])
	}
	if rules[1]["match"] != "*" {
		t.Errorf("expected second rule match='*', got '%s'", rules[1]["match"])
	}

	flags1 := rules[0]["flags"].([]string)
	if len(flags1) != 2 {
		t.Errorf("expected 2 flags for first rule, got %d", len(flags1))
	}
	flags2 := rules[1]["flags"].([]string)
	if len(flags2) != 1 || flags2[0] != "HttpOnly" {
		t.Errorf("expected flags=[HttpOnly] for second rule, got %v", flags2)
	}
}

func TestConvert_ProxyCookieFlags_WithRewrite(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cookie-flags-rewrite", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": "rewrite ^/api/(.*) /$1 break;\nproxy_cookie_flags sessionid SameSite=None Secure;",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	// Both rewrite and proxy-cookie-flags → 1 PluginConfig with 2 plugins
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	plugins := result.PluginConfigs[0].Spec.Plugins
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}

	// First plugin should be proxy-rewrite
	if plugins[0].Name != "proxy-rewrite" {
		t.Errorf("expected first plugin to be proxy-rewrite, got '%s'", plugins[0].Name)
	}

	// Second plugin should be proxy-cookie-flags
	if plugins[1].Name != "proxy-cookie-flags" {
		t.Errorf("expected second plugin to be proxy-cookie-flags, got '%s'", plugins[1].Name)
	}
}

func TestParseProxyCookieFlags(t *testing.T) {
	tests := []struct {
		name    string
		snippet string
		wantLen int
		want0M  string
		want0F  []string
	}{
		{
			"single",
			"proxy_cookie_flags sessionid SameSite=None Secure;",
			1, "sessionid", []string{"SameSite=None", "Secure"},
		},
		{
			"wildcard",
			"proxy_cookie_flags * HttpOnly;",
			1, "*", []string{"HttpOnly"},
		},
		{
			"multiple",
			"proxy_cookie_flags sessionid SameSite=None Secure;\nproxy_cookie_flags * HttpOnly;",
			2, "sessionid", []string{"SameSite=None", "Secure"},
		},
		{
			"no_flags",
			"proxy_cookie_flags sessionid;",
			0, "", nil,
		},
		{
			"none",
			"some other text",
			0, "", nil,
		},
		{
			"empty",
			"",
			0, "", nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseProxyCookieFlags(tt.snippet)
			if len(got) != tt.wantLen {
				t.Fatalf("parseProxyCookieFlags(%q): got len %d, want %d", tt.snippet, len(got), tt.wantLen)
			}
			if tt.wantLen > 0 {
				if got[0]["match"] != tt.want0M {
					t.Errorf("got match=%q, want %q", got[0]["match"], tt.want0M)
				}
				flags := got[0]["flags"].([]string)
				if len(flags) != len(tt.want0F) {
					t.Errorf("got %d flags, want %d", len(flags), len(tt.want0F))
				} else {
					for i, f := range flags {
						if f != tt.want0F[i] {
							t.Errorf("got flag[%d]=%q, want %q", i, f, tt.want0F[i])
						}
					}
				}
			}
		})
	}
}

func TestParseProxyCookiePath(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantLen int
		want0M  string
		want0R  string
	}{
		{
			name:    "regex with quoted replacement",
			value:   `~(.*) "$1"`,
			wantLen: 1,
			want0M:  `~(.*)`,
			want0R:  "$1",
		},
		{
			name:    "regex without quotes",
			value:   `~^/api/(.*) /$1`,
			wantLen: 1,
			want0M:  `~^/api/(.*)`,
			want0R:  "/$1",
		},
		{
			name:    "literal match",
			value:   "/old-path /new-path",
			wantLen: 1,
			want0M:  "/old-path",
			want0R:  "/new-path",
		},
		{
			name:    "empty value",
			value:   "",
			wantLen: 0,
		},
		{
			name:    "whitespace only",
			value:   "   ",
			wantLen: 0,
		},
		{
			name:    "regex with complex pattern",
			value:   `~(?i)/cinder_dashboard_api/(.*) "/$1"`,
			wantLen: 1,
			want0M:  `~(?i)/cinder_dashboard_api/(.*)`,
			want0R:  "/$1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseProxyCookiePath(tt.value)
			if len(got) != tt.wantLen {
				t.Fatalf("parseProxyCookiePath(%q): got len %d, want %d", tt.value, len(got), tt.wantLen)
			}
			if tt.wantLen > 0 {
				if got[0]["match"] != tt.want0M {
					t.Errorf("got match=%q, want %q", got[0]["match"], tt.want0M)
				}
				if got[0]["replacement"] != tt.want0R {
					t.Errorf("got replacement=%q, want %q", got[0]["replacement"], tt.want0R)
				}
			}
		})
	}
}

func TestConvert_ProxyCookiePath_ProducesPluginConfig(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cookie-path", "default",
		map[string]string{
			`ingress.kubernetes.io/proxy-cookie-path`: `~(.*) "$1"`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	pc := result.PluginConfigs[0]
	if len(pc.Spec.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(pc.Spec.Plugins))
	}

	p := pc.Spec.Plugins[0]
	if p.Name != "proxy-cookie-path" {
		t.Errorf("expected plugin name proxy-cookie-path, got %s", p.Name)
	}
	if !p.Enable {
		t.Error("expected plugin to be enabled")
	}

	cfg := p.Config.(map[string]interface{})
	pathPairs, ok := cfg["path_pairs"].([]map[string]interface{})
	if !ok {
		t.Fatalf("path_pairs is not []map[string]interface{}: %T", cfg["path_pairs"])
	}
	if len(pathPairs) != 1 {
		t.Fatalf("expected 1 path_pair, got %d", len(pathPairs))
	}
	pair := pathPairs[0]
	if pair["match"] != "~(.*)" {
		t.Errorf("expected match=~(.*), got %q", pair["match"])
	}
	if pair["replacement"] != "$1" {
		t.Errorf("expected replacement=$1, got %q", pair["replacement"])
	}
}

func TestConvert_ProxyCookiePath_NoManualWarning(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cookie-path-no-warn", "default",
		map[string]string{
			`ingress.kubernetes.io/proxy-cookie-path`: `~(.*) "$1"`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	for _, w := range result.Warnings {
		if strings.Contains(w, "proxy-cookie-path") {
			t.Errorf("should not warn about proxy-cookie-path: %s", w)
		}
	}
}

func TestConvert_ProxyCookiePath_InvalidValue_Warns(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cookie-path-invalid", "default",
		map[string]string{
			`ingress.kubernetes.io/proxy-cookie-path`: `invalid_value`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "无法解析") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a warning about unparseable proxy-cookie-path value")
	}
	if len(result.PluginConfigs) != 0 {
		t.Errorf("expected no PluginConfigs for invalid value, got %d", len(result.PluginConfigs))
	}
}

func TestConvert_ProxyBodySize_ProducesPluginConfig(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("body-size", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/proxy-body-size": "10m",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	pc := result.PluginConfigs[0]
	if len(pc.Spec.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(pc.Spec.Plugins))
	}
	p := pc.Spec.Plugins[0]
	if p.Name != "client-control" {
		t.Errorf("expected client-control plugin, got '%s'", p.Name)
	}
	if !p.Enable {
		t.Error("expected client-control to be enabled")
	}
	cfg, ok := p.Config.(map[string]interface{})
	if !ok {
		t.Fatal("expected plugin config to be a map")
	}
	if cfg["max_body_size"] != int64(10485760) {
		t.Errorf("expected max_body_size=10485760, got %v", cfg["max_body_size"])
	}

	// Should NOT generate a warning
	for _, w := range result.Warnings {
		if strings.Contains(w, "proxy-body-size") {
			t.Errorf("should not warn about proxy-body-size, got: %v", w)
		}
	}

	// PluginConfig should be linked
	out := result.Ingresses[0].(ingress.Ingress)
	if out.Metadata.Annotations["k8s.apisix.apache.org/plugin-config-name"] != pc.Metadata.Name {
		t.Error("client-control PluginConfig should be linked via annotation")
	}
}

// --- New auto-conversion tests ---

func TestConvert_LimitMultiplier_AppliesToRPS(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("limit-multi", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rps":        "10",
			"nginx.ingress.kubernetes.io/limit-multiplier": "5",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	rate, ok := cfg["rate"].(float64)
	if !ok {
		t.Fatalf("expected rate to be float64, got %T: %v", cfg["rate"], cfg["rate"])
	}
	if rate != 50 {
		t.Errorf("expected rate=50 (10*5), got %v", rate)
	}
	if cfg["burst"] != 250 {
		t.Errorf("expected burst=250 (50*5), got %v", cfg["burst"])
	}
}

func TestConvert_LimitMultiplier_AppliesToRPM(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("limit-multi-rpm", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rpm":        "100",
			"nginx.ingress.kubernetes.io/limit-multiplier": "2",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	// 100 rpm * 2 = 200 rpm, divided by 60 = ~3.333 rps
	rate, ok := cfg["rate"].(float64)
	if !ok {
		t.Fatalf("expected rate to be a float64, got %T: %v", cfg["rate"], cfg["rate"])
	}
	if rate < 3.33 || rate > 3.34 {
		t.Errorf("expected rate≈3.33 (200rpm/60), got %v", rate)
	}
}

func TestConvert_LimitMultiplier_InvalidIgnored(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("limit-multi-invalid", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rps":        "10",
			"nginx.ingress.kubernetes.io/limit-multiplier": "abc",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	rate, ok := cfg["rate"].(float64)
	if !ok {
		t.Fatalf("expected rate to be float64, got %T: %v", cfg["rate"], cfg["rate"])
	}
	if rate != 10 {
		t.Errorf("expected rate=10 (multiplier ignored), got %v", rate)
	}
	if cfg["burst"] != 50 {
		t.Errorf("expected burst=50 (10*5), got %v", cfg["burst"])
	}
}

func TestConvert_BothRPSAndRPM_TakesMoreRestrictive(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("both-rate", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rps": "10",
			"nginx.ingress.kubernetes.io/limit-rpm": "300", // 300/60 = 5 rps, more restrictive
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	rate, ok := cfg["rate"].(float64)
	if !ok {
		t.Fatalf("expected rate to be float64, got %T: %v", cfg["rate"], cfg["rate"])
	}
	if rate != 5 {
		t.Errorf("expected rate=5 (rpm=300/60 is more restrictive than rps=10), got %v", rate)
	}
	if cfg["burst"] != 25 {
		t.Errorf("expected burst=25 (5*5), got %v", cfg["burst"])
	}

	// Should warn about both being set
	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "limit-rps 和 limit-rpm 同时设置") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Error("expected warning about both limit-rps and limit-rpm being set")
	}
}

func TestConvert_BothRPSAndRPM_RPSStricter(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("both-rate-rps", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/limit-rps": "3",
			"nginx.ingress.kubernetes.io/limit-rpm": "300", // 300/60 = 5 rps, rps is more restrictive
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	rate, ok := cfg["rate"].(float64)
	if !ok {
		t.Fatalf("expected rate to be float64, got %T: %v", cfg["rate"], cfg["rate"])
	}
	if rate != 3 {
		t.Errorf("expected rate=3 (rps=3 is more restrictive than rpm=300/60=5), got %v", rate)
	}
}

func TestConvert_EnableAccessLog_Annotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("access-log", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-access-log": "false",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/enable-access-log"] != "false" {
		t.Errorf("expected enable-access-log=false, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/enable-access-log"])
	}

	// No warning for enable-access-log
	for _, w := range result.Warnings {
		if strings.Contains(w, "enable-access-log") {
			t.Errorf("should not warn about enable-access-log: %s", w)
		}
	}
}

func TestConvert_EnableAccessLog_True(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("access-log-true", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-access-log": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/enable-access-log"] != "true" {
		t.Errorf("expected enable-access-log=true, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/enable-access-log"])
	}
}

func TestConvert_RealIP_ProducesPluginConfig(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("real-ip", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-real-ip": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	p := result.PluginConfigs[0].Spec.Plugins[0]
	if p.Name != "real-ip" {
		t.Errorf("expected real-ip plugin, got '%s'", p.Name)
	}
	if !p.Enable {
		t.Error("expected real-ip to be enabled")
	}
	cfg := p.Config.(map[string]interface{})
	if cfg["source"] != "http_x_forwarded_for" {
		t.Errorf("expected source=http_x_forwarded_for, got '%v'", cfg["source"])
	}

	// No warning for enable-real-ip
	for _, w := range result.Warnings {
		if strings.Contains(w, "enable-real-ip") {
			t.Errorf("should not warn about enable-real-ip: %s", w)
		}
	}
}

func TestConvert_RealIP_WithForwardedForHeader(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("real-ip-header", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-real-ip":       "true",
			"nginx.ingress.kubernetes.io/forwarded-for-header": "X-Real-IP",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["source"] != "http_x_real_ip" {
		t.Errorf("expected source=http_x_real_ip, got '%v'", cfg["source"])
	}
}

func TestConvert_UpstreamHashBy_ProducesBackendTrafficPolicy(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("hash-by", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/upstream-hash-by": "$remote_addr",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.BackendTrafficPolicies) != 1 {
		t.Fatalf("expected 1 BackendTrafficPolicy, got %d", len(result.BackendTrafficPolicies))
	}
	btp := result.BackendTrafficPolicies[0]
	if btp.Spec.LoadBalancer.Type != "chash" {
		t.Errorf("expected chash, got %s", btp.Spec.LoadBalancer.Type)
	}
	if btp.Spec.LoadBalancer.HashOn != "vars" {
		t.Errorf("expected hashOn=vars, got %s", btp.Spec.LoadBalancer.HashOn)
	}
	if btp.Spec.LoadBalancer.Key != "remote_addr" {
		t.Errorf("expected key=remote_addr, got %s", btp.Spec.LoadBalancer.Key)
	}
}

func TestConvert_UpstreamHashBy_ArgProducesQueryArg(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("hash-by-arg", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/upstream-hash-by": "$arg_user_id",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.BackendTrafficPolicies) != 1 {
		t.Fatalf("expected 1 BackendTrafficPolicy, got %d", len(result.BackendTrafficPolicies))
	}
	btp := result.BackendTrafficPolicies[0]
	if btp.Spec.LoadBalancer.HashOn != "vars" {
		t.Errorf("expected hashOn=vars, got %s", btp.Spec.LoadBalancer.HashOn)
	}
	if btp.Spec.LoadBalancer.Key != "arg_user_id" {
		t.Errorf("expected key=arg_user_id, got %s", btp.Spec.LoadBalancer.Key)
	}

	// upstream-hash-by should NOT produce a warning
	for _, w := range result.Warnings {
		if strings.Contains(w, "upstream-hash-by") && strings.Contains(w, "无法自动转换") {
			t.Fatalf("upstream-hash-by should now be auto-converted, got: %s", w)
		}
	}
}

func TestConvert_HealthCheck_ProducesApisixUpstream(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("health-check", "default",
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

	// Health checks now go into ApisixUpstream, not BackendTrafficPolicy
	if len(result.ApisixUpstreams) != 1 {
		t.Fatalf("expected 1 ApisixUpstream, got %d", len(result.ApisixUpstreams))
	}
	au := result.ApisixUpstreams[0]
	if au.Spec.HealthCheck == nil {
		t.Fatal("expected healthCheck to be set")
	}
	if au.Spec.HealthCheck.Active == nil {
		t.Fatal("expected healthCheck.active to be set")
	}
	active := au.Spec.HealthCheck.Active
	if active.Type != "http" {
		t.Errorf("expected type=http, got %s", active.Type)
	}
	if active.HTTPPath != "/healthz" {
		t.Errorf("expected httpPath=/healthz, got %s", active.HTTPPath)
	}
	if active.Timeout != "5s" {
		t.Errorf("expected timeout=5s, got %s", active.Timeout)
	}
	if active.Healthy == nil || active.Healthy.Successes != 3 {
		t.Errorf("expected healthy.successes=3, got %v", active.Healthy)
	}
	if active.Healthy != nil && active.Healthy.Interval != "10s" {
		t.Errorf("expected healthy.interval=10s, got %s", active.Healthy.Interval)
	}
	if active.Unhealthy == nil {
		t.Fatal("expected unhealthy to be set")
	}

	// No warning for health-check
	for _, w := range result.Warnings {
		if strings.Contains(w, "health-check") {
			t.Errorf("should not warn about health-check: %s", w)
		}
	}
}

func TestConvert_SessionCookieExpires_ExtendsPlugin(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cookie-expires", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/session-cookie-hash":    "sha1",
			"nginx.ingress.kubernetes.io/session-cookie-name":    "escookie",
			"nginx.ingress.kubernetes.io/session-cookie-expires": "3600",
			"nginx.ingress.kubernetes.io/session-cookie-path":    "/app",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["max_age"] != 3600 {
		t.Errorf("expected max_age=3600, got %v", cfg["max_age"])
	}
	if cfg["cookie_path"] != "/app" {
		t.Errorf("expected cookie_path=/app, got %v", cfg["cookie_path"])
	}
}

func TestConvert_SessionCookieMaxAge_TakesPriorityOverExpires(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cookie-maxage", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/session-cookie-hash":    "sha1",
			"nginx.ingress.kubernetes.io/session-cookie-name":    "escookie",
			"nginx.ingress.kubernetes.io/session-cookie-expires": "3600",
			"nginx.ingress.kubernetes.io/session-cookie-max-age": "7200",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["max_age"] != 7200 {
		t.Errorf("expected max_age=7200 (max-age takes priority), got %v", cfg["max_age"])
	}
}

func TestConvert_SessionCookieConditionalSameSiteNone_Warns(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("cookie-samesite", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/session-cookie-hash":                      "sha1",
			"nginx.ingress.kubernetes.io/session-cookie-name":                      "escookie",
			"nginx.ingress.kubernetes.io/session-cookie-conditional-samesite-none": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "session-cookie-conditional-samesite-none") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected warning for session-cookie-conditional-samesite-none, got: %v", result.Warnings)
	}
}

func TestConvert_SSLVerify_False_NoPlugin(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("ssl-verify", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/ssl-verify": "false",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	// ssl-verify=false produces a warning (not a PluginConfig)
	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "ssl-verify") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected warning for ssl-verify, got: %v", result.Warnings)
	}
}

func TestConvert_AuthRealm_NoExtraPluginConfig(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("auth-realm-test", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/auth-type":  "basic",
			"nginx.ingress.kubernetes.io/auth-realm": "My Site",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// auth-type=basic → native APISIX auth-type annotation
	if out.Metadata.Annotations["k8s.apisix.apache.org/auth-type"] != "basicAuth" {
		t.Errorf("expected auth-type=basicAuth, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/auth-type"])
	}

	// auth-realm → now auto-converted to native AIC annotation
	if out.Metadata.Annotations["k8s.apisix.apache.org/auth-realm"] != "My Site" {
		t.Errorf("expected auth-realm=My Site, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/auth-realm"])
	}
}

func TestConvert_ComputeFullForwardedFor_Warns(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("compute-ff", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-real-ip":             "true",
			"nginx.ingress.kubernetes.io/compute-full-forwarded-for": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	// Should produce real-ip plugin
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	if result.PluginConfigs[0].Spec.Plugins[0].Name != "real-ip" {
		t.Errorf("expected real-ip plugin, got '%s'",
			result.PluginConfigs[0].Spec.Plugins[0].Name)
	}
	cfg := result.PluginConfigs[0].Spec.Plugins[0].Config.(map[string]interface{})
	if cfg["trusted_addresses"] == nil {
		t.Error("expected trusted_addresses to be set")
	}

	// Should produce a warning about compute-full-forwarded-for
	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "compute-full-forwarded-for") {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Error("expected a warning about compute-full-forwarded-for")
	}
}

func TestConvert_AuthRealm_ProducesAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("auth-realm-annot", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/auth-realm": "401: Authentication Required",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	// auth-realm is now auto-converted → should produce native APISIX annotation
	if out.Metadata.Annotations["k8s.apisix.apache.org/auth-realm"] != "401: Authentication Required" {
		t.Errorf("expected auth-realm annotation, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/auth-realm"])
	}

	// Should NOT produce a warning
	for _, w := range result.Warnings {
		if strings.Contains(w, "auth-realm") && strings.Contains(w, "无法自动转换") {
			t.Errorf("should not warn about auth-realm anymore: %s", w)
		}
	}
}

func TestConvert_DenylistSourceRange_ProducesAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("denylist", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/denylist-source-range": "10.0.0.0/8,172.16.0.0/12",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/blocklist-source-range"] != "10.0.0.0/8,172.16.0.0/12" {
		t.Errorf("expected blocklist-source-range, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/blocklist-source-range"])
	}
}

func TestConvert_PermanentRedirect_ProducesAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("perm-redirect", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/permanent-redirect": "https://new.example.com",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/http-redirect"] != "https://new.example.com" {
		t.Errorf("expected http-redirect, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/http-redirect"])
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/http-redirect-code"] != "308" {
		t.Errorf("expected http-redirect-code=308, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/http-redirect-code"])
	}
}

func TestConvert_TemporalRedirect_ProducesAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("temp-redirect", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/temporal-redirect": "https://maintenance.example.com",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/http-redirect"] != "https://maintenance.example.com" {
		t.Errorf("expected http-redirect, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/http-redirect"])
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/http-redirect-code"] != "302" {
		t.Errorf("expected http-redirect-code=302, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/http-redirect-code"])
	}
}

func TestConvert_AppRoot_ProducesAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("app-root", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/app-root": "/app",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/http-redirect"] != "/app" {
		t.Errorf("expected http-redirect=/app, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/http-redirect"])
	}
}

func TestConvert_CustomHttpErrors_ProducesAnnotation(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("custom-errs", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/custom-http-errors": "404,500,503",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/custom-error-codes"] != "404,500,503" {
		t.Errorf("expected custom-error-codes=404,500,503, got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/custom-error-codes"])
	}

	// Should not produce any warnings about this annotation
	for _, w := range result.Warnings {
		if strings.Contains(w, "custom-http-errors") {
			t.Errorf("unexpected warning about custom-http-errors: %s", w)
		}
	}
}

func TestConvert_KeepaliveAnnotations_ProducesWarning(t *testing.T) {
	yamlData := []byte(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: keepalive-ingress
  namespace: default
  annotations:
    nginx.ingress.kubernetes.io/upstream-keepalive-connections: "64"
    nginx.ingress.kubernetes.io/upstream-keepalive-requests: "1000"
    nginx.ingress.kubernetes.io/upstream-keepalive-timeout: "60"
spec:
  rules:
    - host: keepalive.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: keepalive-svc
                port:
                  number: 80
`)
	input, err := ParseIngressYAML(yamlData)
	if err != nil {
		t.Fatal(err)
	}
	c := newTestConverter()
	result := c.ConvertList(input)

	// Keepalive annotations are NOT supported by AIC ApisixUpstream CRD
	// So no ApisixUpstream should be produced
	if len(result.ApisixUpstreams) != 0 {
		t.Fatalf("expected 0 ApisixUpstreams (keepalive not supported by AIC), got %d", len(result.ApisixUpstreams))
	}

	// Should produce a warning about keepalive not being supported
	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "upstream-keepalive") && strings.Contains(w, "不支持") {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Error("expected a warning about upstream-keepalive not being supported by AIC")
	}
}

func TestConvert_NoKeepaliveAnnotations_NoApisixUpstream(t *testing.T) {
	yamlData := []byte(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: no-keepalive-ingress
  namespace: default
spec:
  rules:
    - host: nokeepalive.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: svc1
                port:
                  number: 80
`)
	input, err := ParseIngressYAML(yamlData)
	if err != nil {
		t.Fatal(err)
	}
	c := newTestConverter()
	result := c.ConvertList(input)

	if len(result.ApisixUpstreams) != 0 {
		t.Errorf("expected 0 ApisixUpstreams when no keepalive annotations, got %d", len(result.ApisixUpstreams))
	}
}

func TestConvert_TlsSection_ProducesApisixTls(t *testing.T) {
	yamlData := []byte(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: tls-ingress
  namespace: default
spec:
  tls:
    - hosts:
        - secure.example.com
        - alt.example.com
      secretName: my-tls-secret
  rules:
    - host: secure.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: tls-svc
                port:
                  number: 443
`)
	input, err := ParseIngressYAML(yamlData)
	if err != nil {
		t.Fatal(err)
	}
	c := newTestConverter()
	result := c.ConvertList(input)

	if len(result.ApisixTls) != 1 {
		t.Fatalf("expected 1 ApisixTls, got %d", len(result.ApisixTls))
	}

	atls := result.ApisixTls[0]
	if atls.Kind != "ApisixTls" {
		t.Errorf("expected Kind=ApisixTls, got %s", atls.Kind)
	}
	if len(atls.Spec.Hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(atls.Spec.Hosts))
	}
	if atls.Spec.Secret.Name != "my-tls-secret" {
		t.Errorf("expected secret name my-tls-secret, got %s", atls.Spec.Secret.Name)
	}
	if atls.Spec.Secret.Namespace != "default" {
		t.Errorf("expected secret namespace default, got %s", atls.Spec.Secret.Namespace)
	}
	if atls.Spec.IngressClassName == "" {
		t.Error("expected IngressClassName to be set")
	}
}

func TestConvert_NoTlsSection_NoApisixTls(t *testing.T) {
	yamlData := []byte(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: notls-ingress
  namespace: default
spec:
  rules:
    - host: plain.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: svc1
                port:
                  number: 80
`)
	input, err := ParseIngressYAML(yamlData)
	if err != nil {
		t.Fatal(err)
	}
	c := newTestConverter()
	result := c.ConvertList(input)

	if len(result.ApisixTls) != 0 {
		t.Errorf("expected 0 ApisixTls when no TLS section, got %d", len(result.ApisixTls))
	}
}

func TestConvert_MultipleTlsEntries_ProducesMultipleApisixTls(t *testing.T) {
	yamlData := []byte(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: multi-tls-ingress
  namespace: default
spec:
  tls:
    - hosts:
        - a.example.com
      secretName: secret-a
    - hosts:
        - b.example.com
        - c.example.com
      secretName: secret-b
  rules:
    - host: a.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: svc1
                port:
                  number: 80
`)
	input, err := ParseIngressYAML(yamlData)
	if err != nil {
		t.Fatal(err)
	}
	c := newTestConverter()
	result := c.ConvertList(input)

	if len(result.ApisixTls) != 2 {
		t.Fatalf("expected 2 ApisixTls, got %d", len(result.ApisixTls))
	}

	if result.ApisixTls[0].Spec.Secret.Name != "secret-a" {
		t.Errorf("first TLS: expected secret name secret-a, got %s", result.ApisixTls[0].Spec.Secret.Name)
	}
	if result.ApisixTls[1].Spec.Secret.Name != "secret-b" {
		t.Errorf("second TLS: expected secret name secret-b, got %s", result.ApisixTls[1].Spec.Secret.Name)
	}
	if len(result.ApisixTls[1].Spec.Hosts) != 2 {
		t.Errorf("second TLS: expected 2 hosts, got %d", len(result.ApisixTls[1].Spec.Hosts))
	}
}

func TestConvert_AddHeader_Single(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("add-header", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": `add_header X-Frame-Options "SAMEORIGIN";`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	pc := result.PluginConfigs[0]
	var rwPlugin *apisix.Plugin
	for i := range pc.Spec.Plugins {
		if pc.Spec.Plugins[i].Name == "response-rewrite" {
			rwPlugin = &pc.Spec.Plugins[i]
			break
		}
	}
	if rwPlugin == nil {
		t.Fatal("expected response-rewrite plugin in PluginConfig")
	}
	if !rwPlugin.Enable {
		t.Error("expected response-rewrite to be enabled")
	}

	cfg, ok := rwPlugin.Config.(map[string]interface{})
	if !ok {
		t.Fatal("expected plugin config to be a map")
	}
	headers, ok := cfg["headers"].(map[string]interface{})
	if !ok {
		t.Fatal("expected headers to be a map")
	}
	setHeaders, ok := headers["set"].(map[string]string)
	if !ok {
		t.Fatalf("expected headers.set to be map[string]string, got %T", headers["set"])
	}
	if setHeaders["X-Frame-Options"] != "SAMEORIGIN" {
		t.Errorf("expected X-Frame-Options=SAMEORIGIN, got '%s'", setHeaders["X-Frame-Options"])
	}

	// No warnings about add_header
	for _, w := range result.Warnings {
		if strings.Contains(w, "add_header") {
			t.Errorf("should not warn about add_header, got: %v", w)
		}
	}
}

func TestConvert_AddHeader_Multiple(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("add-header-multi", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": `add_header Content-Security-Policy "default-src 'self';" always;
add_header X-Frame-Options "SAMEORIGIN";
add_header X-Content-Type-Options "nosniff";`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	pc := result.PluginConfigs[0]
	var rwPlugin *apisix.Plugin
	for i := range pc.Spec.Plugins {
		if pc.Spec.Plugins[i].Name == "response-rewrite" {
			rwPlugin = &pc.Spec.Plugins[i]
			break
		}
	}
	if rwPlugin == nil {
		t.Fatal("expected response-rewrite plugin in PluginConfig")
	}

	cfg := rwPlugin.Config.(map[string]interface{})
	headers := cfg["headers"].(map[string]interface{})
	setHeaders := headers["set"].(map[string]string)

	if len(setHeaders) != 3 {
		t.Fatalf("expected 3 headers, got %d: %v", len(setHeaders), setHeaders)
	}
	if setHeaders["Content-Security-Policy"] != "default-src 'self';" {
		t.Errorf("CSP header wrong: '%s'", setHeaders["Content-Security-Policy"])
	}
	if setHeaders["X-Frame-Options"] != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options wrong: '%s'", setHeaders["X-Frame-Options"])
	}
	if setHeaders["X-Content-Type-Options"] != "nosniff" {
		t.Errorf("X-Content-Type-Options wrong: '%s'", setHeaders["X-Content-Type-Options"])
	}
}

func TestConvert_AddHeader_WithAlways(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("add-header-always", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": `add_header Strict-Transport-Security "max-age=31536000" always;`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	pc := result.PluginConfigs[0]
	var rwPlugin *apisix.Plugin
	for i := range pc.Spec.Plugins {
		if pc.Spec.Plugins[i].Name == "response-rewrite" {
			rwPlugin = &pc.Spec.Plugins[i]
			break
		}
	}
	if rwPlugin == nil {
		t.Fatal("expected response-rewrite plugin")
	}

	cfg := rwPlugin.Config.(map[string]interface{})
	headers := cfg["headers"].(map[string]interface{})
	setHeaders := headers["set"].(map[string]string)

	if setHeaders["Strict-Transport-Security"] != "max-age=31536000" {
		t.Errorf("HSTS header wrong: '%s'", setHeaders["Strict-Transport-Security"])
	}
}

func TestConvert_AddHeader_WithRewrite(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("add-header-rewrite", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/rewrite-target":        "/",
			"nginx.ingress.kubernetes.io/configuration-snippet": `add_header X-Custom "value";`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/api", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	pc := result.PluginConfigs[0]
	var hasRewrite, hasResponseRewrite bool
	for _, p := range pc.Spec.Plugins {
		switch p.Name {
		case "proxy-rewrite":
			hasRewrite = true
		case "response-rewrite":
			hasResponseRewrite = true
			cfg := p.Config.(map[string]interface{})
			headers := cfg["headers"].(map[string]interface{})
			setHeaders := headers["set"].(map[string]string)
			if setHeaders["X-Custom"] != "value" {
				t.Errorf("X-Custom header wrong: '%s'", setHeaders["X-Custom"])
			}
		}
	}
	if !hasRewrite {
		t.Error("expected proxy-rewrite plugin")
	}
	if !hasResponseRewrite {
		t.Error("expected response-rewrite plugin")
	}
}

func TestConvert_MoreSetHeaders(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("more-headers", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/configuration-snippet": `more_set_headers "X-Test: hello";
more_set_headers "X-Another: world";`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}

	pc := result.PluginConfigs[0]
	var rwPlugin *apisix.Plugin
	for i := range pc.Spec.Plugins {
		if pc.Spec.Plugins[i].Name == "response-rewrite" {
			rwPlugin = &pc.Spec.Plugins[i]
			break
		}
	}
	if rwPlugin == nil {
		t.Fatal("expected response-rewrite plugin")
	}

	cfg := rwPlugin.Config.(map[string]interface{})
	headers := cfg["headers"].(map[string]interface{})
	setHeaders := headers["set"].(map[string]string)

	if setHeaders["X-Test"] != "hello" {
		t.Errorf("X-Test header wrong: '%s'", setHeaders["X-Test"])
	}
	if setHeaders["X-Another"] != "world" {
		t.Errorf("X-Another header wrong: '%s'", setHeaders["X-Another"])
	}
}

func TestConvert_AddHeader_NoSnippet_NoPlugin(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("no-snippet", "default",
		map[string]string{},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 0 {
		t.Fatalf("expected 0 PluginConfigs, got %d", len(result.PluginConfigs))
	}
}
