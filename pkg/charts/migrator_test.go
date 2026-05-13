package charts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrate_SimpleAnnotationRename(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
  namespace: default
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/enable-cors: "true"
spec:
  ingressClassName: nginx
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: test-svc
                port:
                  number: 80
`
	result, configs, warns, changed := migrateFileContent(input, "test.yaml")
	if !changed {
		t.Fatal("expected changes")
	}
	if len(configs) > 0 {
		t.Errorf("expected no plugin configs, got %d", len(configs))
	}
	if len(warns) > 0 {
		t.Errorf("expected no warnings, got %v", warns)
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/http-to-https: \"true\"") {
		t.Error("expected http-to-https annotation")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/enable-cors: \"true\"") {
		t.Error("expected enable-cors annotation")
	}
	if strings.Contains(result, "nginx.ingress.kubernetes.io/ssl-redirect") {
		t.Error("expected ssl-redirect to be removed")
	}
	if !strings.Contains(result, "ingressClassName: apisix") {
		t.Error("expected ingressClassName: apisix")
	}
	if strings.Contains(result, "ingressClassName: nginx") {
		t.Error("expected ingressClassName: nginx to be replaced")
	}
}

func TestMigrate_ValueTransform(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: timeout-ingress
  annotations:
    nginx.ingress.kubernetes.io/proxy-connect-timeout: "30"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "60"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "120"
    nginx.ingress.kubernetes.io/auth-type: "basic"
    nginx.ingress.kubernetes.io/backend-protocol: "GRPC"
spec:
  ingressClassName: nginx
  rules: []
`
	result, _, _, changed := migrateFileContent(input, "test.yaml")
	if !changed {
		t.Fatal("expected changes")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/upstream-connect-timeout: 30s") {
		t.Error("expected connect timeout with 's' suffix")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/upstream-send-timeout: 60s") {
		t.Error("expected send timeout with 's' suffix")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/upstream-read-timeout: 120s") {
		t.Error("expected read timeout with 's' suffix")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/auth-type: basicAuth") {
		t.Error("expected auth-type: basicAuth")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/upstream-scheme: grpc") {
		t.Error("expected upstream-scheme: grpc (lowercase)")
	}
}

func TestMigrate_ProxyRedirectPair(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: redirect-ingress
  annotations:
    nginx.ingress.kubernetes.io/proxy-redirect-from: "http://"
    nginx.ingress.kubernetes.io/proxy-redirect-to: "https://"
spec:
  ingressClassName: nginx
  rules: []
`
	result, _, _, changed := migrateFileContent(input, "test.yaml")
	if !changed {
		t.Fatal("expected changes")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/http-to-https: \"true\"") {
		t.Error("expected http-to-https annotation")
	}
	if strings.Contains(result, "proxy-redirect-from") {
		t.Error("expected proxy-redirect-from to be removed")
	}
	if strings.Contains(result, "proxy-redirect-to") {
		t.Error("expected proxy-redirect-to to be removed")
	}
}

func TestMigrate_PluginConfig_Generation(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: rate-limit-ingress
  namespace: default
  annotations:
    nginx.ingress.kubernetes.io/limit-rps: "100"
    nginx.ingress.kubernetes.io/limit-connections: "200"
spec:
  ingressClassName: nginx
  rules: []
`
	result, configs, _, changed := migrateFileContent(input, "test.yaml")
	if !changed {
		t.Fatal("expected changes")
	}
	if len(configs) == 0 {
		t.Fatal("expected at least one plugin config")
	}

	cfg := configs[0]
	if len(cfg.plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(cfg.plugins))
	}
	if cfg.plugins[0].name != "limit-req" {
		t.Errorf("expected limit-req plugin, got %s", cfg.plugins[0].name)
	}
	if cfg.plugins[1].name != "limit-conn" {
		t.Errorf("expected limit-conn plugin, got %s", cfg.plugins[1].name)
	}
	if len(cfg.values) == 0 {
		t.Error("expected values to be populated")
	}

	// Check annotations removed
	if strings.Contains(result, "nginx.ingress.kubernetes.io/limit-rps") {
		t.Error("expected limit-rps to be removed")
	}
	if strings.Contains(result, "nginx.ingress.kubernetes.io/limit-connections") {
		t.Error("expected limit-connections to be removed")
	}
}

func TestMigrate_ConfigurationSnippet_Rewrite(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: snippet-ingress
  annotations:
    nginx.ingress.kubernetes.io/configuration-snippet: |
      rewrite ^/api/(.*) /$1 break;
      rewrite ^/v2/(.*) /$1 break;
spec:
  ingressClassName: nginx
  rules: []
`
	result, configs, _, changed := migrateFileContent(input, "test.yaml")
	if !changed {
		t.Fatal("expected changes")
	}
	if len(configs) == 0 {
		t.Fatal("expected plugin config for multiple rewrites")
	}

	found := false
	for _, p := range configs[0].plugins {
		if p.name == "proxy-rewrite" {
			found = true
		}
	}
	if !found {
		t.Error("expected proxy-rewrite plugin")
	}

	// Snippet should be removed (both rewrites consumed)
	if strings.Contains(result, "configuration-snippet") {
		t.Error("expected snippet to be removed")
	}
}

func TestMigrate_ConfigurationSnippet_SingleRewrite(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: snippet-single
  annotations:
    nginx.ingress.kubernetes.io/configuration-snippet: |
      rewrite ^/api/(.*) /$1 break;
spec:
  ingressClassName: nginx
  rules: []
`
	result, configs, _, changed := migrateFileContent(input, "test.yaml")
	if !changed {
		t.Fatal("expected changes")
	}
	// Single rewrite should be removed from snippet (consumed as native annotation)
	if strings.Contains(result, "configuration-snippet") {
		t.Error("expected snippet to be fully consumed for single rewrite")
	}
	// Should not generate plugin config for single rewrite
	if len(configs) > 0 {
		t.Errorf("expected no plugin configs for single rewrite, got %d", len(configs))
	}
}

func TestMigrate_ConfigurationSnippet_CookieFlags(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: cookie-flags-ingress
  annotations:
    nginx.ingress.kubernetes.io/configuration-snippet: |
      proxy_cookie_flags sessionid SameSite=None Secure;
spec:
  ingressClassName: nginx
  rules: []
`
	_, configs, _, changed := migrateFileContent(input, "test.yaml")
	if !changed {
		t.Fatal("expected changes")
	}
	if len(configs) == 0 {
		t.Fatal("expected plugin config for proxy-cookie-flags")
	}

	found := false
	for _, p := range configs[0].plugins {
		if p.name == "proxy-cookie-flags" {
			found = true
			if !strings.Contains(p.config, "sessionid") {
				t.Error("expected sessionid in config")
			}
			if !strings.Contains(p.config, "SameSite=None") {
				t.Error("expected SameSite=None in config")
			}
			if !strings.Contains(p.config, "Secure") {
				t.Error("expected Secure in config")
			}
		}
	}
	if !found {
		t.Error("expected proxy-cookie-flags plugin")
	}
}

func TestMigrate_IngressClassName(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: class-ingress
spec:
  ingressClassName: nginx
  rules: []
`
	result, _, _, changed := migrateFileContent(input, "test.yaml")
	if !changed {
		t.Fatal("expected changes")
	}
	if !strings.Contains(result, "ingressClassName: apisix") {
		t.Error("expected ingressClassName: apisix")
	}
	if strings.Contains(result, "ingressClassName: nginx") {
		t.Error("expected nginx to be replaced")
	}
}

func TestMigrate_RemoveIngressClassAnnotation(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: class-annotation-ingress
  annotations:
    kubernetes.io/ingress.class: nginx
    nginx.ingress.kubernetes.io/enable-cors: "true"
spec:
  rules: []
`
	result, _, _, changed := migrateFileContent(input, "test.yaml")
	if !changed {
		t.Fatal("expected changes")
	}
	if strings.Contains(result, "kubernetes.io/ingress.class") {
		t.Error("expected kubernetes.io/ingress.class to be removed")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/enable-cors") {
		t.Error("expected enable-cors to be converted")
	}
}

func TestMigrate_ManualAnnotations(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: manual-ingress
  annotations:
    nginx.ingress.kubernetes.io/affinity: "cookie"
    nginx.ingress.kubernetes.io/auth-tls-verify-client: "on"
spec:
  rules: []
`
	result, _, _, changed := migrateFileContent(input, "test.yaml")
	if !changed {
		t.Fatal("expected changes")
	}
	if !strings.Contains(result, "# TODO [ingress2apisix]") {
		t.Error("expected TODO comment for manual annotations")
	}
	if !strings.Contains(result, "affinity") {
		t.Error("expected affinity to be preserved with TODO")
	}
}

func TestMigrate_HelmTemplatePreserved(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .Release.Name }}-ingress
  annotations:
    nginx.ingress.kubernetes.io/proxy-connect-timeout: {{ .Values.timeout | default "30" }}
    nginx.ingress.kubernetes.io/limit-rps: {{ .Values.rateLimit }}
spec:
  ingressClassName: nginx
  rules: []
`
	result, configs, _, changed := migrateFileContent(input, "test.yaml")
	if !changed {
		t.Fatal("expected changes")
	}
	// Helm template in timeout value should be preserved
	if !strings.Contains(result, `{{ .Values.timeout | default "30" }}`) {
		t.Error("expected Helm template to be preserved in timeout value")
	}
	// limit-rps with Helm template should generate plugin config
	if len(configs) == 0 {
		t.Error("expected plugin config for limit-rps with Helm template")
	}
}

func TestMigrate_PluginConfigTemplate(t *testing.T) {
	configs := []pluginConfigData{
		{
			ingressName:      "test-ingress",
			pluginConfigName: "test-ingress-plugins",
			plugins: []pluginEntry{
				{
					name:   "limit-req",
					config: "        rate: \"100\"\n        burst: 0\n        key: remote_addr\n        rejected_code: 429",
				},
			},
			values: map[string]string{"limitRps": "100"},
			isHelm: true,
		},
	}
	tmpl := generatePluginConfigTemplate(configs)
	if !strings.Contains(tmpl, "ApisixPluginConfig") {
		t.Error("expected ApisixPluginConfig kind")
	}
	if !strings.Contains(tmpl, "limit-req") {
		t.Error("expected limit-req plugin")
	}
	if !strings.Contains(tmpl, "test-ingress-plugins") {
		t.Error("expected plugin config name")
	}
	if !strings.Contains(tmpl, "ingress2apisix") {
		t.Error("expected managed-by label")
	}
	if !strings.Contains(tmpl, "{{ .Release.Name }}") {
		t.Error("expected Helm template syntax")
	}
}

func TestMigrate_ValuesYamlUpdated(t *testing.T) {
	dir := t.TempDir()
	valuesPath := filepath.Join(dir, "values.yaml")
	os.WriteFile(valuesPath, []byte("host: example.com\n"), 0644)

	configs := []pluginConfigData{
		{
			values: map[string]string{"limitRps": "100"},
		},
	}
	err := updateValuesYaml(valuesPath, configs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "host: example.com") {
		t.Error("expected original values preserved")
	}
	if !strings.Contains(content, "limitRps") {
		t.Error("expected limitRps entry")
	}
	if !strings.Contains(content, "100") {
		t.Error("expected default value 100")
	}
	if !strings.Contains(content, "ingress2apisix") {
		t.Error("expected ingress2apisix section header")
	}
}

func TestMigrate_PluginConfigAnnotation(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: pc-annot-ingress
  namespace: default
  annotations:
    nginx.ingress.kubernetes.io/limit-rps: "100"
spec:
  ingressClassName: nginx
  rules: []
`
	result, configs, _, _ := migrateFileContent(input, "test.yaml")
	if len(configs) == 0 {
		t.Fatal("expected plugin config")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/plugin-config-name") {
		t.Error("expected plugin-config-name annotation to be added")
	}
}

func TestMigrate_DryRun(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "my-chart")
	templatesDir := filepath.Join(chartDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	// Create Chart.yaml
	os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: test\n"), 0644)

	// Create values.yaml
	os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("host: example.com\n"), 0644)

	// Create ingress template
	ingressContent := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/limit-rps: "100"
spec:
  ingressClassName: nginx
  rules: []
`
	os.WriteFile(filepath.Join(templatesDir, "ingress.yaml"), []byte(ingressContent), 0644)

	report, err := MigrateChartsDir(dir, MigrateOptions{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.FilesModified != 1 {
		t.Errorf("expected 1 file modified, got %d", report.FilesModified)
	}
	if len(report.Diffs) != 1 {
		t.Errorf("expected 1 diff, got %d", len(report.Diffs))
	}

	// Verify file was NOT modified
	data, _ := os.ReadFile(filepath.Join(templatesDir, "ingress.yaml"))
	if !strings.Contains(string(data), "ssl-redirect") {
		t.Error("dry-run should not modify files")
	}
}

func TestMigrate_Backup(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "my-chart")
	templatesDir := filepath.Join(chartDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: test\n"), 0644)
	os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("host: example.com\n"), 0644)

	ingressContent := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  rules: []
`
	ingressPath := filepath.Join(templatesDir, "ingress.yaml")
	os.WriteFile(ingressPath, []byte(ingressContent), 0644)

	report, err := MigrateChartsDir(dir, MigrateOptions{Backup: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.FilesModified != 1 {
		t.Errorf("expected 1 file modified, got %d", report.FilesModified)
	}

	// Verify backup was created
	bakPath := ingressPath + ".bak"
	bakData, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("expected backup file: %v", err)
	}
	if !strings.Contains(string(bakData), "ssl-redirect") {
		t.Error("backup should contain original content")
	}
}

func TestMigrate_NoIngress(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "service.yaml"), []byte(`apiVersion: v1
kind: Service
metadata:
  name: my-svc
spec: {}
`), 0644)

	report, err := MigrateChartsDir(dir, MigrateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.FilesModified != 0 {
		t.Errorf("expected no files modified, got %d", report.FilesModified)
	}
}

func TestMigrate_RewriteTarget_Regex(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: regex-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /$2
spec:
  ingressClassName: nginx
  rules: []
`
	result, _, _, _ := migrateFileContent(input, "test.yaml")
	if !strings.Contains(result, "k8s.apisix.apache.org/rewrite-target-regex") {
		t.Error("expected rewrite-target-regex for $2 capture")
	}
}

func TestMigrate_AuthType_Digest(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: digest-ingress
  annotations:
    nginx.ingress.kubernetes.io/auth-type: "digest"
spec:
  ingressClassName: nginx
  rules: []
`
	result, _, _, _ := migrateFileContent(input, "test.yaml")
	if !strings.Contains(result, "k8s.apisix.apache.org/auth-type: keyAuth") {
		t.Error("expected auth-type: keyAuth for digest")
	}
}

func TestMigrate_WebsocketServices(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ws-ingress
  annotations:
    nginx.ingress.kubernetes.io/websocket-services: "my-ws-svc"
spec:
  ingressClassName: nginx
  rules: []
`
	result, _, _, _ := migrateFileContent(input, "test.yaml")
	if !strings.Contains(result, "k8s.apisix.apache.org/enable-websocket: true") {
		t.Error("expected enable-websocket: true")
	}
	if strings.Contains(result, "websocket-services") {
		t.Error("expected websocket-services to be removed")
	}
}

func TestMigrate_CORS_Overrides(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: cors-ingress
  annotations:
    nginx.ingress.kubernetes.io/enable-cors: "true"
    nginx.ingress.kubernetes.io/cors-allow-origin: "https://example.com"
    nginx.ingress.kubernetes.io/cors-allow-methods: "GET,POST"
spec:
  ingressClassName: nginx
  rules: []
`
	result, _, _, _ := migrateFileContent(input, "test.yaml")
	if !strings.Contains(result, "k8s.apisix.apache.org/enable-cors: \"true\"") {
		t.Error("expected enable-cors")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/cors-allow-origin: \"https://example.com\"") {
		t.Error("expected cors-allow-origin override")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/cors-allow-methods: \"GET,POST\"") {
		t.Error("expected cors-allow-methods override")
	}
}

func TestMigrate_PrefixDualSupport(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dual-prefix
  annotations:
    ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/enable-cors: "true"
spec:
  ingressClassName: nginx
  rules: []
`
	result, _, _, _ := migrateFileContent(input, "test.yaml")
	if !strings.Contains(result, "k8s.apisix.apache.org/http-to-https") {
		t.Error("expected ingress.kubernetes.io/ssl-redirect to be converted")
	}
	if !strings.Contains(result, "k8s.apisix.apache.org/enable-cors") {
		t.Error("expected enable-cors to be converted")
	}
}

func TestMigrate_ProxyCookiePath(t *testing.T) {
	input := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: cookie-path-ingress
  annotations:
    nginx.ingress.kubernetes.io/proxy-cookie-path: "/old /new"
spec:
  ingressClassName: nginx
  rules: []
`
	_, configs, _, _ := migrateFileContent(input, "test.yaml")
	if len(configs) == 0 {
		t.Fatal("expected plugin config for proxy-cookie-path")
	}
	found := false
	for _, p := range configs[0].plugins {
		if p.name == "proxy-cookie-path" {
			found = true
		}
	}
	if !found {
		t.Error("expected proxy-cookie-path plugin")
	}
}

func TestMigrate_PluginConfigTemplate_GeneratesFile(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "my-chart")
	templatesDir := filepath.Join(chartDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: test\n"), 0644)
	os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("host: example.com\n"), 0644)

	ingressContent := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
  namespace: default
  annotations:
    nginx.ingress.kubernetes.io/limit-rps: "100"
spec:
  ingressClassName: nginx
  rules: []
`
	os.WriteFile(filepath.Join(templatesDir, "ingress.yaml"), []byte(ingressContent), 0644)

	report, err := MigrateChartsDir(dir, MigrateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.FilesModified != 1 {
		t.Errorf("expected 1 file modified, got %d", report.FilesModified)
	}
	if report.PluginConfigs != 1 {
		t.Errorf("expected 1 plugin config, got %d", report.PluginConfigs)
	}

	// Check plugin config template was generated
	pluginPath := filepath.Join(templatesDir, "apisix-plugin-configs.yaml")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("expected plugin config file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "ApisixPluginConfig") {
		t.Error("expected ApisixPluginConfig in generated file")
	}
	if !strings.Contains(content, "limit-req") {
		t.Error("expected limit-req plugin in generated file")
	}

	// Check ingress was modified with plugin-config-name annotation
	ingressData, _ := os.ReadFile(filepath.Join(templatesDir, "ingress.yaml"))
	ingressContent2 := string(ingressData)
	if !strings.Contains(ingressContent2, "k8s.apisix.apache.org/plugin-config-name") {
		t.Error("expected plugin-config-name annotation in ingress")
	}

	// Check values.yaml was updated
	valuesData, _ := os.ReadFile(filepath.Join(chartDir, "values.yaml"))
	valuesContent := string(valuesData)
	if !strings.Contains(valuesContent, "limitRps") {
		t.Error("expected limitRps in values.yaml")
	}
}

func TestScanAnnotations(t *testing.T) {
	lines := []string{
		"  annotations:",
		"    nginx.ingress.kubernetes.io/ssl-redirect: \"true\"",
		"    nginx.ingress.kubernetes.io/proxy-connect-timeout: \"30\"",
		"    ingress.kubernetes.io/enable-cors: \"true\"",
	}

	entries := scanAnnotations(lines)
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	suffixes := make(map[string]bool)
	for _, e := range entries {
		suffixes[e.suffix] = true
	}
	if !suffixes["ssl-redirect"] {
		t.Error("expected ssl-redirect")
	}
	if !suffixes["proxy-connect-timeout"] {
		t.Error("expected proxy-connect-timeout")
	}
	if !suffixes["enable-cors"] {
		t.Error("expected enable-cors")
	}
}

func TestScanAnnotations_MultiLine(t *testing.T) {
	lines := []string{
		"  annotations:",
		"    nginx.ingress.kubernetes.io/configuration-snippet: |",
		"      rewrite ^/api/(.*) /$1 break;",
		"      more_set_headers \"X-Test: value\";",
		"    nginx.ingress.kubernetes.io/ssl-redirect: \"true\"",
	}

	entries := scanAnnotations(lines)
	snippetFound := false
	for _, e := range entries {
		if e.suffix == "configuration-snippet" {
			snippetFound = true
			if !e.isMultiLine {
				t.Error("expected multi-line snippet")
			}
			if len(e.blockLines) != 2 {
				t.Errorf("expected 2 block lines, got %d", len(e.blockLines))
			}
		}
	}
	if !snippetFound {
		t.Error("expected configuration-snippet entry")
	}
}

func TestEnsureTimeSuffix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"30", "30s"},
		{"30s", "30s"},
		{"500ms", "500ms"},
		{" 60 ", "60s"},
	}
	for _, tt := range tests {
		got := ensureTimeSuffix(tt.input)
		if got != tt.expected {
			t.Errorf("ensureTimeSuffix(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestQuoteValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"\"quoted\"", "\"quoted\""},
		{"has: colon", "\"has: colon\""},
		{"has # hash", "\"has # hash\""},
		{"100s", "100s"},
	}
	for _, tt := range tests {
		got := quoteValue(tt.input)
		if got != tt.expected {
			t.Errorf("quoteValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestMigrateChartsDir_Integration(t *testing.T) {
	dir := t.TempDir()

	// Create a chart with multiple Ingress templates
	chartDir := filepath.Join(dir, "my-app")
	templatesDir := filepath.Join(chartDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: my-app\n"), 0644)
	os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("host: example.com\n"), 0644)

	// Main ingress
	ingress1 := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: main-ingress
  namespace: default
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/enable-cors: "true"
    nginx.ingress.kubernetes.io/rewrite-target: /$2
    nginx.ingress.kubernetes.io/proxy-connect-timeout: "30"
    nginx.ingress.kubernetes.io/limit-rps: "100"
    nginx.ingress.kubernetes.io/affinity: "cookie"
spec:
  ingressClassName: nginx
  rules: []
`
	os.WriteFile(filepath.Join(templatesDir, "ingress.yaml"), []byte(ingress1), 0644)

	// Non-ingress file (should be skipped)
	svc := `apiVersion: v1
kind: Service
metadata:
  name: my-svc
spec: {}
`
	os.WriteFile(filepath.Join(templatesDir, "service.yaml"), []byte(svc), 0644)

	report, err := MigrateChartsDir(dir, MigrateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.FilesProcessed != 4 {
		t.Errorf("expected 4 files processed (Chart.yaml + values.yaml + ingress.yaml + service.yaml), got %d", report.FilesProcessed)
	}
	if report.FilesModified != 1 {
		t.Errorf("expected 1 file modified, got %d", report.FilesModified)
	}
	if report.PluginConfigs != 1 {
		t.Errorf("expected 1 plugin config, got %d", report.PluginConfigs)
	}

	// Verify ingress was modified
	data, _ := os.ReadFile(filepath.Join(templatesDir, "ingress.yaml"))
	content := string(data)
	if strings.Contains(content, "nginx.ingress.kubernetes.io/ssl-redirect") {
		t.Error("ssl-redirect should be converted")
	}
	if !strings.Contains(content, "k8s.apisix.apache.org/http-to-https") {
		t.Error("expected http-to-https")
	}
	if !strings.Contains(content, "ingressClassName: apisix") {
		t.Error("expected apisix class name")
	}
	if !strings.Contains(content, "# TODO") {
		t.Error("expected TODO for affinity")
	}

	// Verify plugin config template was generated
	pluginData, err := os.ReadFile(filepath.Join(templatesDir, "apisix-plugin-configs.yaml"))
	if err != nil {
		t.Fatalf("expected plugin config file: %v", err)
	}
	if !strings.Contains(string(pluginData), "ApisixPluginConfig") {
		t.Error("expected ApisixPluginConfig")
	}
}

func TestBuildProxyCookiePathConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "simple",
			input:    "/old /new",
			contains: []string{`match: "/old"`, `replacement: "/new"`},
		},
		{
			name:     "regex",
			input:    "~ ^/api/(.*) /$1",
			contains: []string{`match: "~ ^/api/(.*)"`, `replacement: "/$1"`},
		},
		{
			name:     "empty",
			input:    "",
			contains: []string{"path_pairs", "match"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := buildProxyCookiePathConfig(tt.input)
			for _, s := range tt.contains {
				if !strings.Contains(config, s) {
					t.Errorf("expected %q in config: %s", s, config)
				}
			}
		})
	}
}
