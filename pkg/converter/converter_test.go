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

	if out.Metadata.Annotations["k8s.apisix.apache.org/enable-cors"] != "true" {
		t.Error("expected cors annotation to be set")
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/cors-allow-origin"] != "*" {
		t.Error("expected cors-allow-origin to be '*'")
	}
	if _, ok := out.Metadata.Annotations["nginx.ingress.kubernetes.io/enable-cors"]; ok {
		t.Error("original nginx cors annotation should be removed")
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
	out := result.Ingresses[0].(ingress.Ingress)

	if out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target"] != "/" {
		t.Error("expected simple rewrite-target")
	}
	if _, ok := out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target-regex"]; ok {
		t.Error("should not set rewrite-target-regex for simple target")
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
	out := result.Ingresses[0].(ingress.Ingress)

	// Per migration doc: rewrite-target with $1/$2 → rewrite-target-regex
	if _, ok := out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target"]; ok {
		t.Error("should not set simple rewrite-target for regex captures")
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target-regex"] != "/$2" {
		t.Errorf("expected rewrite-target-regex='/$2', got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target-regex"])
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
	if cfg["rate"] != "1000" {
		t.Errorf("expected rate=1000, got '%v'", cfg["rate"])
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
	if out.Metadata.Annotations["k8s.apisix.apache.org/auth-request-headers"] != "User-Agent,cookie" {
		t.Errorf("expected default auth-request-headers, got '%s'",
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

	if out.Metadata.Annotations["k8s.apisix.apache.org/enable-cors"] != "true" {
		t.Error("expected cors annotation from ingress.kubernetes.io prefix")
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
	if cfg["rate"] != "50" {
		t.Errorf("expected nginx prefix to take priority, got rate=%v", cfg["rate"])
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

	if out.Metadata.Annotations["k8s.apisix.apache.org/enable-cors"] != "true" {
		t.Error("expected cors from nginx prefix")
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
	if !bytes.Contains(buf.Bytes(), []byte("k8s.apisix.apache.org/enable-cors")) {
		t.Error("output should contain APISIX CORS annotation")
	}
	// PluginConfig must be linked via annotation
	if !bytes.Contains(buf.Bytes(), []byte("k8s.apisix.apache.org/plugin-config-name")) {
		t.Error("output should contain plugin-config-name annotation linking PluginConfig")
	}
	if bytes.Contains(buf.Bytes(), []byte("nginx.ingress.kubernetes.io/")) {
		t.Error("output should not contain any nginx annotations")
	}
	if bytes.Contains(buf.Bytes(), []byte("ingress.kubernetes.io/")) {
		t.Error("output should not contain any ingress.kubernetes.io annotations")
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
	c := newTestConverter()
	ing := makeTestIngress("err-ingress", "default",
		map[string]string{
			"ingress.kubernetes.io/custom-http-errors": "404,500",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "custom-http-errors") && strings.Contains(w, "4.1.3") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for custom-http-errors, got warnings: %v", result.Warnings)
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
	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "upstream-hash-by") && strings.Contains(w, "没有等价原生注解") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Fatalf("expected warning for upstream-hash-by, got: %v", result.Warnings)
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

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "proxy-body-size") && strings.Contains(w, "client_max_body_size") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for proxy-body-size, got warnings: %v", result.Warnings)
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
			"nginx.ingress.kubernetes.io/configuration-snippet": `more_set_headers "X-Forwarded-For $http_x_forwarded_for";`,
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "more_set_headers") && strings.Contains(w, "4.1.5") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for more_set_headers, got warnings: %v", result.Warnings)
	}
}

func TestWarning_AuthSecretInPluginConfig(t *testing.T) {
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

	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "auth-secret") && strings.Contains(w, "ApisixConsumer") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for auth-secret needing ApisixConsumer, got warnings: %v", result.Warnings)
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

func TestConvert_NoPluginConfig_NoLink(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("no-link", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors": "true",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	if len(result.PluginConfigs) != 0 {
		t.Fatalf("expected 0 PluginConfigs, got %d", len(result.PluginConfigs))
	}

	out := result.Ingresses[0].(ingress.Ingress)
	if _, ok := out.Metadata.Annotations["k8s.apisix.apache.org/plugin-config-name"]; ok {
		t.Error("should not have plugin-config-name annotation when no PluginConfig is generated")
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

func TestConvert_CORS_NoMaxAge(t *testing.T) {
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
}

// --- Single Rewrite from configuration-snippet → native annotations ---

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
	out := result.Ingresses[0].(ingress.Ingress)

	// Single rewrite → native annotations (not PluginConfig)
	if out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target-regex"] != "^/api/(.*)" {
		t.Errorf("expected rewrite-target-regex='^/api/(.*)', got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target-regex"])
	}
	if out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target-regex-template"] != "/$1" {
		t.Errorf("expected rewrite-target-regex-template='/$1', got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target-regex-template"])
	}
	if len(result.PluginConfigs) != 0 {
		t.Errorf("single rewrite should use native annotations, not PluginConfig, got %d PluginConfigs",
			len(result.PluginConfigs))
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

func TestWarning_AuthSecretWithoutAuthType(t *testing.T) {
	c := newTestConverter()
	ing := makeTestIngress("auth-secret-only", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/auth-secret": "my-secret",
		},
		nil,
		[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)},
	)

	result := c.Convert(ing)

	// auth-secret alone without auth-type → should warn about auth-secret needing ApisixConsumer
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "auth-secret") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about auth-secret, got: %v", result.Warnings)
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
	// CORS should still be converted
	if out.Metadata.Annotations["k8s.apisix.apache.org/enable-cors"] != "true" {
		t.Error("expected CORS annotation to be set")
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

	// Single rewrite → native annotation, not PluginConfig for rewrite
	out := result.Ingresses[0].(ingress.Ingress)
	if out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target-regex"] != "^/api/(.*)" {
		t.Errorf("expected rewrite-target-regex='^/api/(.*)', got '%s'",
			out.Metadata.Annotations["k8s.apisix.apache.org/rewrite-target-regex"])
	}

	// proxy-cookie-flags → PluginConfig
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig for cookie-flags, got %d", len(result.PluginConfigs))
	}
	if result.PluginConfigs[0].Spec.Plugins[0].Name != "proxy-cookie-flags" {
		t.Errorf("expected proxy-cookie-flags plugin, got '%s'",
			result.PluginConfigs[0].Spec.Plugins[0].Name)
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
