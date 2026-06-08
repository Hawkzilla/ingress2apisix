package converter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ingress2apisix/pkg/apisix"
	"github.com/ingress2apisix/pkg/ingress"
)

func newTestGatewayConverter() *GatewayConverter {
	return NewGatewayConverter(apisix.DefaultConversionOptions())
}

// --- Basic conversion tests ---

func TestGatewayConvert_BasicIngress(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("basic-ingress", "default", nil, nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	// Should produce 1 GatewayClass
	if len(result.GatewayClasses) != 1 {
		t.Fatalf("expected 1 GatewayClass, got %d", len(result.GatewayClasses))
	}
	gcRes := result.GatewayClasses[0]
	if gcRes.Kind != "GatewayClass" {
		t.Errorf("expected kind GatewayClass, got %s", gcRes.Kind)
	}
	if gcRes.Spec.ControllerName != "apisix.apache.org/gateway-controller" {
		t.Errorf("expected controller name apisix.apache.org/gateway-controller, got %s", gcRes.Spec.ControllerName)
	}

	// Should produce 1 Gateway (for host app.com)
	if len(result.Gateways) != 1 {
		t.Fatalf("expected 1 Gateway, got %d", len(result.Gateways))
	}
	gw := result.Gateways[0]
	if gw.Spec.GatewayClassName != "apisix" {
		t.Errorf("expected gatewayClassName=apisix, got %s", gw.Spec.GatewayClassName)
	}
	// Should have HTTP listener
	if len(gw.Spec.Listeners) < 1 {
		t.Fatal("expected at least 1 listener")
	}
	if gw.Spec.Listeners[0].Protocol != "HTTP" {
		t.Errorf("expected HTTP listener, got %s", gw.Spec.Listeners[0].Protocol)
	}
	if gw.Spec.Listeners[0].Port != 80 {
		t.Errorf("expected port 80, got %d", gw.Spec.Listeners[0].Port)
	}

	// Should produce 1 HTTPRoute
	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}
	route := result.HTTPRoutes[0]
	if route.Kind != "HTTPRoute" {
		t.Errorf("expected kind HTTPRoute, got %s", route.Kind)
	}
	if len(route.Spec.Hostnames) != 1 || route.Spec.Hostnames[0] != "app.com" {
		t.Errorf("expected hostname app.com, got %v", route.Spec.Hostnames)
	}
	if len(route.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(route.Spec.Rules))
	}

	// Check match
	match := route.Spec.Rules[0].Matches[0]
	if match.Path == nil {
		t.Fatal("expected path match")
	}
	if match.Path.Type != "PathPrefix" {
		t.Errorf("expected PathPrefix, got %s", match.Path.Type)
	}
	if match.Path.Value != "/" {
		t.Errorf("expected path /, got %s", match.Path.Value)
	}

	// Check backend ref
	if len(route.Spec.Rules[0].BackendRefs) != 1 {
		t.Fatalf("expected 1 backend ref, got %d", len(route.Spec.Rules[0].BackendRefs))
	}
	br := route.Spec.Rules[0].BackendRefs[0]
	if br.Name != "svc" || br.Port != 80 {
		t.Errorf("expected backend svc:80, got %s:%d", br.Name, br.Port)
	}

	// Check parent ref
	if len(route.Spec.ParentRefs) != 1 {
		t.Fatalf("expected 1 parent ref, got %d", len(route.Spec.ParentRefs))
	}
	if route.Spec.ParentRefs[0].Name != gw.Metadata.Name {
		t.Errorf("expected parentRef=%s, got %s", gw.Metadata.Name, route.Spec.ParentRefs[0].Name)
	}
}

func TestGatewayConvert_TLSIngress(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("tls-ingress", "default", nil,
				[]ingress.IngressTLS{
					{Hosts: []string{"secure.com"}, SecretName: "tls-sec"},
				},
				[]ingress.IngressRule{makeSimpleRule("secure.com", "/", "svc", 443)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	if len(result.Gateways) != 1 {
		t.Fatalf("expected 1 Gateway, got %d", len(result.Gateways))
	}
	gw := result.Gateways[0]

	// Should have both HTTP and HTTPS listeners
	if len(gw.Spec.Listeners) != 2 {
		t.Fatalf("expected 2 listeners (HTTP + HTTPS), got %d", len(gw.Spec.Listeners))
	}

	httpListener := gw.Spec.Listeners[0]
	if httpListener.Protocol != "HTTP" {
		t.Errorf("expected HTTP, got %s", httpListener.Protocol)
	}

	httpsListener := gw.Spec.Listeners[1]
	if httpsListener.Protocol != "HTTPS" {
		t.Errorf("expected HTTPS, got %s", httpsListener.Protocol)
	}
	if httpsListener.Port != 443 {
		t.Errorf("expected port 443, got %d", httpsListener.Port)
	}
	if httpsListener.TLS == nil {
		t.Fatal("expected TLS config")
	}
	if httpsListener.TLS.Mode != "Terminate" {
		t.Errorf("expected TLS mode Terminate, got %s", httpsListener.TLS.Mode)
	}
	if len(httpsListener.TLS.CertRefs) != 1 {
		t.Fatalf("expected 1 cert ref, got %d", len(httpsListener.TLS.CertRefs))
	}
	if httpsListener.TLS.CertRefs[0].Name != "tls-sec" {
		t.Errorf("expected cert ref tls-sec, got %s", httpsListener.TLS.CertRefs[0].Name)
	}
	if httpsListener.TLS.CertRefs[0].Kind != "Secret" {
		t.Errorf("expected kind Secret, got %s", httpsListener.TLS.CertRefs[0].Kind)
	}
}

func TestGatewayConvert_PathTypes(t *testing.T) {
	gc := newTestGatewayConverter()

	tests := []struct {
		pathType string
		path     string
		wantType string
	}{
		{"Prefix", "/", "PathPrefix"},
		{"Prefix", "/api", "PathPrefix"},
		{"Exact", "/api/v1", "Exact"},
		{"ImplementationSpecific", "/api", "PathPrefix"},
	}

	for _, tt := range tests {
		input := ParsedInput{
			Ingresses: []ingress.Ingress{
				makeTestIngress("test", "default", nil, nil,
					[]ingress.IngressRule{{
						Host: "app.com",
						HTTP: &ingress.HTTPIngressRuleValue{
							Paths: []ingress.HTTPIngressPath{
								{
									Path:     tt.path,
									PathType: strPtr(tt.pathType),
									Backend: ingress.IngressBackend{
										Service: &ingress.IngressServiceBackend{
											Name: "svc",
											Port: ingress.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					}}),
			},
			Format: apisix.FormatSingleDoc,
		}

		result := gc.ConvertList(input)
		if len(result.HTTPRoutes) != 1 {
			t.Fatalf("pathType=%s: expected 1 HTTPRoute, got %d", tt.pathType, len(result.HTTPRoutes))
		}
		match := result.HTTPRoutes[0].Spec.Rules[0].Matches[0]
		if match.Path == nil {
			t.Fatalf("pathType=%s: expected path match", tt.pathType)
		}
		if match.Path.Type != tt.wantType {
			t.Errorf("pathType=%s path=%s: expected %s, got %s", tt.pathType, tt.path, tt.wantType, match.Path.Type)
		}
	}
}

func TestGatewayConvert_RegexPath(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("regex-ingress", "default", nil, nil,
				[]ingress.IngressRule{{
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
				}}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)
	match := result.HTTPRoutes[0].Spec.Rules[0].Matches[0]
	if match.Path == nil {
		t.Fatal("expected path match")
	}
	if match.Path.Type != "RegularExpression" {
		t.Errorf("expected RegularExpression, got %s", match.Path.Type)
	}
}

// --- Annotation filter tests ---

func TestGatewayConvert_RewriteTarget_Simple(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("rewrite-simple", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/rewrite-target": "/",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/api", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)
	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}

	rule := result.HTTPRoutes[0].Spec.Rules[0]
	if len(rule.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(rule.Filters))
	}
	f := rule.Filters[0]
	if f.Type != "URLRewrite" {
		t.Errorf("expected URLRewrite, got %s", f.Type)
	}
	if f.URLRewrite == nil || f.URLRewrite.Path == nil {
		t.Fatal("expected URLRewrite path")
	}
	if f.URLRewrite.Path.Type != "ReplaceFullPath" {
		t.Errorf("expected ReplaceFullPath, got %s", f.URLRewrite.Path.Type)
	}
	if f.URLRewrite.Path.Value != "/" {
		t.Errorf("expected path /, got %s", f.URLRewrite.Path.Value)
	}
}

func TestGatewayConvert_RewriteTarget_Regex_Warning(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("rewrite-regex", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/rewrite-target": "/$2",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/api(/|$)(.*)", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	// Should warn about regex rewrite not being supported
	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "regex captures") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected warning about regex rewrite, got warnings: %v", result.Warnings)
	}
}

func TestGatewayConvert_SSLRedirect(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("ssl-redirect", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/ssl-redirect": "true",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)
	rule := result.HTTPRoutes[0].Spec.Rules[0]

	foundRedirect := false
	for _, f := range rule.Filters {
		if f.Type == "RequestRedirect" && f.RequestRedirect != nil && f.RequestRedirect.Scheme == "https" {
			foundRedirect = true
			if f.RequestRedirect.StatusCode != 301 {
				t.Errorf("expected statusCode 301, got %d", f.RequestRedirect.StatusCode)
			}
		}
	}
	if !foundRedirect {
		t.Errorf("expected RequestRedirect filter with scheme=https, got filters: %v", rule.Filters)
	}
}

func TestGatewayConvert_ForceSSLRedirect(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("force-ssl", "default",
				map[string]string{
					"ingress.kubernetes.io/force-ssl-redirect": "true",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)
	rule := result.HTTPRoutes[0].Spec.Rules[0]

	foundRedirect := false
	for _, f := range rule.Filters {
		if f.Type == "RequestRedirect" && f.RequestRedirect != nil && f.RequestRedirect.Scheme == "https" {
			foundRedirect = true
		}
	}
	if !foundRedirect {
		t.Errorf("expected RequestRedirect filter from force-ssl-redirect, got filters: %v", rule.Filters)
	}
}

func TestGatewayConvert_PermanentRedirect(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("perm-redirect", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/permanent-redirect": "https://new.example.com/path",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)
	rule := result.HTTPRoutes[0].Spec.Rules[0]

	foundRedirect := false
	for _, f := range rule.Filters {
		if f.Type == "RequestRedirect" && f.RequestRedirect != nil && f.RequestRedirect.StatusCode == 308 {
			foundRedirect = true
			if f.RequestRedirect.Hostname != "new.example.com" {
				t.Errorf("expected hostname new.example.com, got %s", f.RequestRedirect.Hostname)
			}
		}
	}
	if !foundRedirect {
		t.Errorf("expected RequestRedirect filter with statusCode 308, got filters: %v", rule.Filters)
	}
}

func TestGatewayConvert_TemporalRedirect(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("temp-redirect", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/temporal-redirect": "https://maintenance.example.com",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)
	rule := result.HTTPRoutes[0].Spec.Rules[0]

	foundRedirect := false
	for _, f := range rule.Filters {
		if f.Type == "RequestRedirect" && f.RequestRedirect != nil && f.RequestRedirect.StatusCode == 302 {
			foundRedirect = true
		}
	}
	if !foundRedirect {
		t.Errorf("expected RequestRedirect filter with statusCode 302, got filters: %v", rule.Filters)
	}
}

func TestGatewayConvert_UpstreamVhost(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("vhost-ingress", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/upstream-vhost": "backend.internal",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)
	rule := result.HTTPRoutes[0].Spec.Rules[0]

	foundRewrite := false
	for _, f := range rule.Filters {
		if f.Type == "URLRewrite" && f.URLRewrite != nil && f.URLRewrite.Hostname == "backend.internal" {
			foundRewrite = true
		}
	}
	if !foundRewrite {
		t.Errorf("expected URLRewrite hostname filter, got filters: %v", rule.Filters)
	}
}

// --- Warning tests ---

func TestGatewayConvert_UnsupportedAnnotations_Warnings(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("unsupported", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/backend-protocol":       "GRPC",
					"nginx.ingress.kubernetes.io/enable-websocket":       "true",
					"nginx.ingress.kubernetes.io/whitelist-source-range": "10.0.0.0/8",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	// Should warn about truly unsupported annotations
	warns := result.Warnings
	checkWarn := func(ann string) {
		for _, w := range warns {
			if strings.Contains(w, ann) && strings.Contains(w, "not natively supported") {
				return
			}
		}
		t.Errorf("expected warning for unsupported annotation %s, got warnings: %v", ann, warns)
	}

	checkWarn("backend-protocol")
	checkWarn("enable-websocket")
	checkWarn("whitelist-source-range")

	// Annotations handled via plugins should NOT produce "not natively supported" warnings
	noWarn := func(ann string) {
		for _, w := range warns {
			if strings.Contains(w, ann) && strings.Contains(w, "not natively supported") {
				t.Errorf("expected NO 'not natively supported' warning for annotation %s (handled via plugins), but got: %s", ann, w)
			}
		}
	}
	noWarn("enable-cors")
	noWarn("limit-rps")
	noWarn("proxy-body-size")
	// Auth and timeout annotations are now handled via plugins
	noWarn("auth-url")
	noWarn("proxy-connect-timeout")
	noWarn("health-check-path")
	noWarn("affinity")
}

func TestGatewayConvert_SnippetRewrite_Warning(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("snippet-rewrite", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/configuration-snippet": `rewrite ^/api/(.*) /$1 break;`,
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	foundWarn := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "configuration-snippet") && strings.Contains(w, "regex pattern") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected warning about configuration-snippet rewrite regex, got warnings: %v", result.Warnings)
	}
}

// --- PluginConfig via ExtensionRef tests ---

func TestGatewayConvert_RateLimit_ProducesPluginConfigWithExtensionRef(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("rate-limit-ing", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/limit-rps": "100",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	// Should produce exactly 1 ApisixPluginConfig
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 ApisixPluginConfig, got %d", len(result.PluginConfigs))
	}
	pc := result.PluginConfigs[0]
	if pc.Kind != "ApisixPluginConfig" {
		t.Errorf("expected kind ApisixPluginConfig, got %s", pc.Kind)
	}
	if pc.Metadata.Name != "rate-limit-ing-plugins" {
		t.Errorf("expected name rate-limit-ing-plugins, got %s", pc.Metadata.Name)
	}
	if len(pc.Spec.Plugins) != 1 {
		t.Fatalf("expected 1 plugin in PluginConfig, got %d", len(pc.Spec.Plugins))
	}
	if pc.Spec.Plugins[0].Name != "limit-req" {
		t.Errorf("expected plugin limit-req, got %s", pc.Spec.Plugins[0].Name)
	}

	// The HTTPRoute should have an ExtensionRef filter pointing to the PluginConfig
	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}
	route := result.HTTPRoutes[0]
	if len(route.Spec.Rules) < 1 {
		t.Fatal("expected at least 1 rule")
	}

	foundExtRef := false
	for _, f := range route.Spec.Rules[0].Filters {
		if f.Type == "ExtensionRef" && f.ExtensionRef != nil {
			foundExtRef = true
			if f.ExtensionRef.Group != "apisix.apache.org" {
				t.Errorf("expected group apisix.apache.org, got %s", f.ExtensionRef.Group)
			}
			if f.ExtensionRef.Kind != "PluginConfig" {
				t.Errorf("expected kind PluginConfig, got %s", f.ExtensionRef.Kind)
			}
			if f.ExtensionRef.Name != "rate-limit-ing-plugins" {
				t.Errorf("expected name rate-limit-ing-plugins, got %s", f.ExtensionRef.Name)
			}
		}
	}
	if !foundExtRef {
		t.Errorf("expected ExtensionRef filter in HTTPRoute rule, got filters: %v", route.Spec.Rules[0].Filters)
	}
}

func TestGatewayConvert_CORS_ProducesPluginConfig(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("cors-ing", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/enable-cors":            "true",
					"nginx.ingress.kubernetes.io/cors-allow-origin":      "https://example.com",
					"nginx.ingress.kubernetes.io/cors-allow-methods":     "GET,POST",
					"nginx.ingress.kubernetes.io/cors-allow-headers":     "Content-Type",
					"nginx.ingress.kubernetes.io/cors-allow-credentials": "true",
					"nginx.ingress.kubernetes.io/cors-max-age":           "3600",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	// Should produce exactly 1 ApisixPluginConfig (for cors-allow-credentials and cors-max-age)
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 ApisixPluginConfig, got %d", len(result.PluginConfigs))
	}
	pc := result.PluginConfigs[0]
	if pc.Kind != "ApisixPluginConfig" {
		t.Errorf("expected kind ApisixPluginConfig, got %s", pc.Kind)
	}
	if pc.Metadata.Name != "cors-ing-plugins" {
		t.Errorf("expected name cors-ing-plugins, got %s", pc.Metadata.Name)
	}

	// PluginConfig should have a cors plugin
	foundCors := false
	for _, p := range pc.Spec.Plugins {
		if p.Name == "cors" {
			foundCors = true
			cfg, ok := p.Config.(map[string]interface{})
			if !ok {
				t.Fatal("expected cors plugin config to be a map")
			}
			if cfg["allow_credential"] != true {
				t.Errorf("expected allow_credential=true, got %v", cfg["allow_credential"])
			}
			if cfg["max_age"] != 3600 {
				t.Errorf("expected max_age=3600, got %v", cfg["max_age"])
			}
		}
	}
	if !foundCors {
		t.Errorf("expected cors plugin in PluginConfig, got plugins: %v", pc.Spec.Plugins)
	}

	// The HTTPRoute should have an ExtensionRef filter pointing to the PluginConfig
	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}
	foundExtRef := false
	for _, f := range result.HTTPRoutes[0].Spec.Rules[0].Filters {
		if f.Type == "ExtensionRef" && f.ExtensionRef != nil && f.ExtensionRef.Name == "cors-ing-plugins" {
			foundExtRef = true
		}
	}
	if !foundExtRef {
		t.Errorf("expected ExtensionRef filter in HTTPRoute, got filters: %v", result.HTTPRoutes[0].Spec.Rules[0].Filters)
	}
}

func TestGatewayConvert_NoPluginConfig_NoExtensionRef(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("no-plugins", "default", nil, nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	// No annotations → no PluginConfig
	if len(result.PluginConfigs) != 0 {
		t.Errorf("expected 0 PluginConfigs, got %d", len(result.PluginConfigs))
	}

	// HTTPRoute should NOT have an ExtensionRef filter
	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}
	for _, f := range result.HTTPRoutes[0].Spec.Rules[0].Filters {
		if f.Type == "ExtensionRef" {
			t.Errorf("did not expect ExtensionRef filter when no plugin config, got: %v", f)
		}
	}
}

func TestGatewayConvert_MultiplePaths(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("multi-path", "default", nil, nil,
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
											Name: "api-svc",
											Port: ingress.ServiceBackendPort{Number: 8080},
										},
									},
								},
								{
									Path:     "/web",
									PathType: strPtr("Prefix"),
									Backend: ingress.IngressBackend{
										Service: &ingress.IngressServiceBackend{
											Name: "web-svc",
											Port: ingress.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}
	route := result.HTTPRoutes[0]
	if len(route.Spec.Rules) != 2 {
		t.Fatalf("expected 2 rules (one per path), got %d", len(route.Spec.Rules))
	}

	// First rule: /api → api-svc:8080
	r0 := route.Spec.Rules[0]
	if r0.Matches[0].Path.Value != "/api" {
		t.Errorf("expected /api, got %s", r0.Matches[0].Path.Value)
	}
	if r0.BackendRefs[0].Name != "api-svc" || r0.BackendRefs[0].Port != 8080 {
		t.Errorf("expected api-svc:8080, got %s:%d", r0.BackendRefs[0].Name, r0.BackendRefs[0].Port)
	}

	// Second rule: /web → web-svc:80
	r1 := route.Spec.Rules[1]
	if r1.Matches[0].Path.Value != "/web" {
		t.Errorf("expected /web, got %s", r1.Matches[0].Path.Value)
	}
	if r1.BackendRefs[0].Name != "web-svc" || r1.BackendRefs[0].Port != 80 {
		t.Errorf("expected web-svc:80, got %s:%d", r1.BackendRefs[0].Name, r1.BackendRefs[0].Port)
	}
}

// --- Multi-Ingress tests ---

func TestGatewayConvert_MultipleIngresses(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("ing-a", "ns1", nil, nil,
				[]ingress.IngressRule{makeSimpleRule("a.com", "/", "svc-a", 80)}),
			makeTestIngress("ing-b", "ns2", nil, nil,
				[]ingress.IngressRule{makeSimpleRule("b.com", "/", "svc-b", 80)}),
		},
		Format: apisix.FormatMultiDoc,
	}

	result := gc.ConvertList(input)

	// 1 GatewayClass, 2 Gateways (one per host), 2 HTTPRoutes
	if len(result.GatewayClasses) != 1 {
		t.Fatalf("expected 1 GatewayClass, got %d", len(result.GatewayClasses))
	}
	if len(result.Gateways) != 2 {
		t.Fatalf("expected 2 Gateways, got %d", len(result.Gateways))
	}
	if len(result.HTTPRoutes) != 2 {
		t.Fatalf("expected 2 HTTPRoutes, got %d", len(result.HTTPRoutes))
	}
}

func TestGatewayConvert_SameHost_TLSAndNonTLS(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("tls-ing", "default", nil,
				[]ingress.IngressTLS{
					{Hosts: []string{"app.com"}, SecretName: "tls-sec"},
				},
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 443)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	// Should have 1 Gateway with 2 listeners (HTTP + HTTPS)
	if len(result.Gateways) != 1 {
		t.Fatalf("expected 1 Gateway, got %d", len(result.Gateways))
	}
	if len(result.Gateways[0].Spec.Listeners) != 2 {
		t.Fatalf("expected 2 listeners, got %d", len(result.Gateways[0].Spec.Listeners))
	}
}

// --- WriteOutput test ---

func TestWriteGatewayAPIResult(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("test-write", "default", nil, nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	var buf bytes.Buffer
	err := WriteGatewayAPIResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "kind: GatewayClass") {
		t.Error("output should contain GatewayClass")
	}
	if !strings.Contains(output, "kind: Gateway") {
		t.Error("output should contain Gateway")
	}
	if !strings.Contains(output, "kind: HTTPRoute") {
		t.Error("output should contain HTTPRoute")
	}
}

func TestWriteConversionResult_GatewayAPI(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("test-write-via-main", "default", nil, nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	var buf bytes.Buffer
	err := WriteConversionResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "kind: GatewayClass") {
		t.Error("WriteConversionResult should delegate to WriteGatewayAPIResult for Gateway API results")
	}
}

func TestWriteGatewayAPIResult_WithPluginConfig(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("write-pc", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/limit-rps": "50",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	var buf bytes.Buffer
	err := WriteGatewayAPIResult(&buf, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "kind: ApisixPluginConfig") {
		t.Error("output should contain ApisixPluginConfig")
	}
	if !strings.Contains(output, "limit-req") {
		t.Error("output should contain limit-req plugin")
	}
}

// --- FormatSummary test ---

func TestFormatGatewayAPIResultSummary(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("summary-test", "default", nil, nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)
	summary := FormatGatewayAPIResultSummary(result)

	if !strings.Contains(summary, "GatewayClass") {
		t.Error("summary should mention GatewayClass")
	}
	if !strings.Contains(summary, "Gateway") {
		t.Error("summary should mention Gateway")
	}
	if !strings.Contains(summary, "HTTPRoute") {
		t.Error("summary should mention HTTPRoute")
	}
}

// --- Helper: extractHostname ---

func TestExtractHostname(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://new.example.com/path", "new.example.com"},
		{"http://example.com", "example.com"},
		{"https://example.com:8443/path", "example.com"},
		{"example.com/path", "example.com"},
	}
	for _, tt := range tests {
		got := extractHostname(tt.input)
		if got != tt.want {
			t.Errorf("extractHostname(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Label tests ---

func TestGatewayConvert_ManagedByLabel(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("label-test", "default", nil, nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	if result.GatewayClasses[0].Metadata.Labels["managed-by"] != "ingress2apisix" {
		t.Error("GatewayClass should have managed-by label")
	}
	if result.Gateways[0].Metadata.Labels["managed-by"] != "ingress2apisix" {
		t.Error("Gateway should have managed-by label")
	}
	if result.HTTPRoutes[0].Metadata.Labels["managed-by"] != "ingress2apisix" {
		t.Error("HTTPRoute should have managed-by label")
	}
	if result.HTTPRoutes[0].Metadata.Labels["ingress-name"] != "label-test" {
		t.Error("HTTPRoute should have ingress-name label")
	}
}

// --- Gateway-only plugin tests (auth, proxy timeouts, etc.) ---

func TestGatewayConvert_AuthAnnotations_ProducesForwardAuthPlugin(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("auth-ing", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/auth-url":              "http://auth-svc/verify",
					"nginx.ingress.kubernetes.io/auth-method":           "GET",
					"nginx.ingress.kubernetes.io/auth-request-headers":  "Authorization",
					"nginx.ingress.kubernetes.io/auth-response-headers": "X-User-Id",
					"nginx.ingress.kubernetes.io/auth-signin":           "http://auth-svc/login",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	// Should produce a PluginConfig with forward-auth
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	pc := result.PluginConfigs[0]

	foundFA := false
	for _, p := range pc.Spec.Plugins {
		if p.Name == "forward-auth" {
			foundFA = true
			cfg, ok := p.Config.(map[string]interface{})
			if !ok {
				t.Fatal("expected forward-auth config to be a map")
			}
			if cfg["uri"] != "http://auth-svc/verify" {
				t.Errorf("expected uri http://auth-svc/verify, got %v", cfg["uri"])
			}
			if cfg["request_method"] != "GET" {
				t.Errorf("expected request_method GET, got %v", cfg["request_method"])
			}
		}
	}
	if !foundFA {
		t.Errorf("expected forward-auth plugin in PluginConfig, got plugins: %v", pc.Spec.Plugins)
	}

	// HTTPRoute should have ExtensionRef
	if len(result.HTTPRoutes) != 1 {
		t.Fatalf("expected 1 HTTPRoute, got %d", len(result.HTTPRoutes))
	}
	foundExtRef := false
	for _, f := range result.HTTPRoutes[0].Spec.Rules[0].Filters {
		if f.Type == "ExtensionRef" && f.ExtensionRef != nil {
			foundExtRef = true
		}
	}
	if !foundExtRef {
		t.Errorf("expected ExtensionRef filter in HTTPRoute, got filters: %v", result.HTTPRoutes[0].Spec.Rules[0].Filters)
	}
}

func TestGatewayConvert_ProxyTimeout_ProducesProxyRewritePlugin(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("timeout-ing", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/proxy-connect-timeout": "30",
					"nginx.ingress.kubernetes.io/proxy-send-timeout":    "60",
					"nginx.ingress.kubernetes.io/proxy-read-timeout":    "120",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	pc := result.PluginConfigs[0]

	foundProxyRewrite := false
	for _, p := range pc.Spec.Plugins {
		if p.Name == "proxy-rewrite" {
			foundProxyRewrite = true
			cfg, ok := p.Config.(map[string]interface{})
			if !ok {
				t.Fatal("expected proxy-rewrite config to be a map")
			}
			if cfg["connect_timeout"] != 30000 {
				t.Errorf("expected connect_timeout=30000, got %v", cfg["connect_timeout"])
			}
			if cfg["send_timeout"] != 60000 {
				t.Errorf("expected send_timeout=60000, got %v", cfg["send_timeout"])
			}
			if cfg["read_timeout"] != 120000 {
				t.Errorf("expected read_timeout=120000, got %v", cfg["read_timeout"])
			}
		}
	}
	if !foundProxyRewrite {
		t.Errorf("expected proxy-rewrite plugin in PluginConfig, got plugins: %v", pc.Spec.Plugins)
	}
}

func TestGatewayConvert_Whitelist_ProducesIPRestrictionPlugin(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("whitelist-ing", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/whitelist-source-range": "10.0.0.0/8,172.16.0.0/12",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	pc := result.PluginConfigs[0]

	foundIPRestriction := false
	for _, p := range pc.Spec.Plugins {
		if p.Name == "ip-restriction" {
			foundIPRestriction = true
			cfg, ok := p.Config.(map[string]interface{})
			if !ok {
				t.Fatal("expected ip-restriction config to be a map")
			}
			whitelist, ok := cfg["whitelist"].([]string)
			if !ok {
				t.Fatal("expected whitelist to be a []string")
			}
			if len(whitelist) != 2 {
				t.Errorf("expected 2 entries in whitelist, got %d", len(whitelist))
			}
		}
	}
	if !foundIPRestriction {
		t.Errorf("expected ip-restriction plugin in PluginConfig, got plugins: %v", pc.Spec.Plugins)
	}
}

func TestGatewayConvert_Denylist_ProducesIPRestrictionPlugin(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("denylist-ing", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/denylist-source-range": "192.168.1.0/24",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig, got %d", len(result.PluginConfigs))
	}
	pc := result.PluginConfigs[0]

	foundIPRestriction := false
	for _, p := range pc.Spec.Plugins {
		if p.Name == "ip-restriction" {
			foundIPRestriction = true
			cfg, ok := p.Config.(map[string]interface{})
			if !ok {
				t.Fatal("expected ip-restriction config to be a map")
			}
			if _, ok := cfg["blacklist"]; !ok {
				t.Error("expected blacklist key in ip-restriction config")
			}
		}
	}
	if !foundIPRestriction {
		t.Errorf("expected ip-restriction plugin in PluginConfig, got plugins: %v", pc.Spec.Plugins)
	}
}

func TestGatewayConvert_HealthCheck_ProducesApisixUpstream(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("hc-ing", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/health-check-path":     "/healthz",
					"nginx.ingress.kubernetes.io/health-check-interval": "10",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	// Health checks now go into ApisixUpstream
	if len(result.ApisixUpstreams) != 1 {
		t.Fatalf("expected 1 ApisixUpstream, got %d", len(result.ApisixUpstreams))
	}
	au := result.ApisixUpstreams[0]
	if au.Spec.HealthCheck == nil {
		t.Fatal("expected healthCheck in ApisixUpstream")
	}
	if au.Spec.HealthCheck.Active == nil {
		t.Fatal("expected active health check")
	}
	if au.Spec.HealthCheck.Active.HTTPPath != "/healthz" {
		t.Errorf("expected health check path /healthz, got %s", au.Spec.HealthCheck.Active.HTTPPath)
	}

	// Should NOT produce "not natively supported" warnings for health-check
	for _, w := range result.Warnings {
		if strings.Contains(w, "health-check") && strings.Contains(w, "not natively supported") {
			t.Errorf("should not warn about health-check as unsupported: %s", w)
		}
	}
}

func TestGatewayConvert_Affinity_ProducesBackendTrafficPolicy(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("affinity-ing", "default",
				map[string]string{
					"nginx.ingress.kubernetes.io/affinity":            "cookie",
					"nginx.ingress.kubernetes.io/session-cookie-name": "MYCOOKIE",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	if len(result.BackendTrafficPolicies) != 1 {
		t.Fatalf("expected 1 BackendTrafficPolicy, got %d", len(result.BackendTrafficPolicies))
	}
	btp := result.BackendTrafficPolicies[0]
	if btp.Spec.LoadBalancer.Type != "chash" {
		t.Errorf("expected chash, got %s", btp.Spec.LoadBalancer.Type)
	}
	if btp.Spec.LoadBalancer.Key != "MYCOOKIE" {
		t.Errorf("expected key MYCOOKIE, got %s", btp.Spec.LoadBalancer.Key)
	}

	// Should NOT produce "not natively supported" warnings for affinity
	for _, w := range result.Warnings {
		if strings.Contains(w, "affinity") && strings.Contains(w, "not natively supported") {
			t.Errorf("should not warn about affinity as unsupported: %s", w)
		}
	}
}

func TestGatewayConvert_MergedPluginsIntoSamePluginConfig(t *testing.T) {
	gc := newTestGatewayConverter()
	input := ParsedInput{
		Ingresses: []ingress.Ingress{
			makeTestIngress("merged-ing", "default",
				map[string]string{
					// These go into buildPluginConfig → limit-req
					"nginx.ingress.kubernetes.io/limit-rps": "100",
					// These go into buildGatewayOnlyPlugins → forward-auth
					"nginx.ingress.kubernetes.io/auth-url": "http://auth-svc/verify",
					// These go into buildGatewayOnlyPlugins → proxy-rewrite
					"nginx.ingress.kubernetes.io/proxy-connect-timeout": "15",
				},
				nil,
				[]ingress.IngressRule{makeSimpleRule("app.com", "/", "svc", 80)}),
		},
		Format: apisix.FormatSingleDoc,
	}

	result := gc.ConvertList(input)

	// Should produce exactly 1 PluginConfig (merged)
	if len(result.PluginConfigs) != 1 {
		t.Fatalf("expected 1 PluginConfig (merged), got %d", len(result.PluginConfigs))
	}
	pc := result.PluginConfigs[0]

	// Should have 3 plugins: limit-req, forward-auth, proxy-rewrite
	if len(pc.Spec.Plugins) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(pc.Spec.Plugins))
	}

	pluginNames := make(map[string]bool)
	for _, p := range pc.Spec.Plugins {
		pluginNames[p.Name] = true
	}
	if !pluginNames["limit-req"] {
		t.Error("expected limit-req plugin")
	}
	if !pluginNames["forward-auth"] {
		t.Error("expected forward-auth plugin")
	}
	if !pluginNames["proxy-rewrite"] {
		t.Error("expected proxy-rewrite plugin")
	}
}
