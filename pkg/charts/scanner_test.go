package charts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckChartsDir_SimpleIngress(t *testing.T) {
	dir := t.TempDir()

	writeTempFile(t, dir, "templates/ingress.yaml", `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
  annotations:
    nginx.ingress.kubernetes.io/enable-cors: "true"
    nginx.ingress.kubernetes.io/limit-rps: "100"
    nginx.ingress.kubernetes.io/session-cookie-hash: "sha1"
    nginx.ingress.kubernetes.io/affinity: "cookie"
    nginx.ingress.kubernetes.io/some-future-thing: "value"
spec:
  ingressClassName: nginx
  rules:
    - host: app.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: svc
                port:
                  number: 80
`)

	report, err := CheckChartsDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalFiles != 1 {
		t.Errorf("expected 1 total file, got %d", report.TotalFiles)
	}
	if report.IngressFiles != 1 {
		t.Errorf("expected 1 ingress file, got %d", report.IngressFiles)
	}
	if report.Converted != 1 {
		t.Errorf("expected 1 converted, got %d", report.Converted)
	}
	if report.PluginConfig != 1 {
		t.Errorf("expected 1 plugin config, got %d", report.PluginConfig)
	}
	if report.CustomPlugin != 1 {
		t.Errorf("expected 1 custom plugin, got %d", report.CustomPlugin)
	}
	if report.Manual != 1 {
		t.Errorf("expected 1 manual, got %d", report.Manual)
	}
	if report.Unknown != 1 {
		t.Errorf("expected 1 unknown, got %d", report.Unknown)
	}
}

func TestCheckChartsDir_HelmTemplate(t *testing.T) {
	dir := t.TempDir()

	writeTempFile(t, dir, "charts/myapp/templates/ingress.yaml", `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .Release.Name }}-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: {{ .Values.rewriteTarget }}
    nginx.ingress.kubernetes.io/enable-cors: "true"
    ingress.kubernetes.io/force-ssl-redirect: "true"
spec:
  ingressClassName: nginx
  rules:
    - host: {{ .Values.host }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .Values.service }}
                port:
                  number: 80
`)

	report, err := CheckChartsDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.IngressFiles != 1 {
		t.Fatalf("expected 1 ingress file, got %d", report.IngressFiles)
	}
	if !report.Files[0].HasHelmTpl {
		t.Error("expected HasHelmTpl=true for Helm template file")
	}

	// Should find: enable-cors (converted), rewrite-target (converted), force-ssl-redirect (converted)
	if report.Converted != 3 {
		t.Errorf("expected 3 converted annotations, got %d", report.Converted)
	}
}

func TestCheckChartsDir_NoIngress(t *testing.T) {
	dir := t.TempDir()

	writeTempFile(t, dir, "templates/configmap.yaml", `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value
`)

	report, err := CheckChartsDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.IngressFiles != 0 {
		t.Errorf("expected 0 ingress files, got %d", report.IngressFiles)
	}
}

func TestCheckChartsDir_MixedPrefixes(t *testing.T) {
	dir := t.TempDir()

	writeTempFile(t, dir, "ingress.yaml", `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: mixed
  annotations:
    nginx.ingress.kubernetes.io/enable-cors: "true"
    ingress.kubernetes.io/proxy-connect-timeout: "30"
    ingress.kubernetes.io/custom-http-errors: "404"
spec:
  ingressClassName: nginx
`)

	report, err := CheckChartsDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Converted != 2 {
		t.Errorf("expected 2 converted (cors + timeout), got %d", report.Converted)
	}
	if report.Manual != 1 {
		t.Errorf("expected 1 manual (custom-http-errors), got %d", report.Manual)
	}
}

func TestCheckChartsDir_TemplateFile(t *testing.T) {
	dir := t.TempDir()

	writeTempFile(t, dir, "templates/ingress.yaml.tpl", `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "0"
spec:
  ingressClassName: nginx
`)

	report, err := CheckChartsDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.IngressFiles != 1 {
		t.Errorf("expected 1 ingress file (.yaml.tpl), got %d", report.IngressFiles)
	}
	if report.Manual != 1 {
		t.Errorf("expected 1 manual (proxy-body-size), got %d", report.Manual)
	}
}

func TestCheckChartsDir_NestedHelmCharts(t *testing.T) {
	dir := t.TempDir()

	writeTempFile(t, dir, "charts/app-a/templates/ingress.yaml", `
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/enable-cors: "true"
`)

	writeTempFile(t, dir, "charts/app-b/templates/ingress.yaml", `
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/limit-rps: "50"
    nginx.ingress.kubernetes.io/limit-connections: "10"
`)

	writeTempFile(t, dir, "charts/app-c/templates/deployment.yaml", `
kind: Deployment
metadata:
  name: app-c
`)

	report, err := CheckChartsDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.IngressFiles != 2 {
		t.Errorf("expected 2 ingress files, got %d", report.IngressFiles)
	}
	if report.TotalFiles < 3 {
		t.Errorf("expected at least 3 total files, got %d", report.TotalFiles)
	}
	if report.Converted != 1 {
		t.Errorf("expected 1 converted (cors), got %d", report.Converted)
	}
	if report.PluginConfig != 2 {
		t.Errorf("expected 2 plugin config (rps + connections), got %d", report.PluginConfig)
	}
}

func TestClassifyAnnotation(t *testing.T) {
	tests := []struct {
		suffix string
		status AnnotationStatus
	}{
		{"enable-cors", StatusConverted},
		{"limit-rps", StatusPluginConfig},
		{"session-cookie-hash", StatusCustomPlugin},
		{"proxy-cookie-path", StatusPluginConfig},
		{"affinity", StatusManual},
		{"totally-unknown-key", StatusUnknown},
	}

	for _, tt := range tests {
		f := classifyAnnotation("test/"+tt.suffix, tt.suffix)
		if f.Status != tt.status {
			t.Errorf("classifyAnnotation(%q): got status %v, want %v", tt.suffix, f.Status, tt.status)
		}
	}
}

func TestFormatCheckReport(t *testing.T) {
	report := &CheckReport{
		TotalFiles:   5,
		IngressFiles: 2,
		Converted:    3,
		PluginConfig: 1,
		Manual:       1,
		Unknown:      0,
		Files: []FileReport{
			{
				Path:      "templates/ingress.yaml",
				IsIngress: true,
				Findings: []AnnotationFinding{
					{Annotation: "nginx.ingress.kubernetes.io/affinity", Status: StatusManual, Detail: "BackendTrafficPolicy"},
				},
			},
		},
	}

	output := FormatCheckReport(report, false)
	if !strings.Contains(output, "Ingress files:") {
		t.Error("report should contain 'Ingress files:'")
	}
	if !strings.Contains(output, "MANUAL") {
		t.Error("report should contain 'MANUAL'")
	}

	verboseOutput := FormatCheckReport(report, true)
	if !strings.Contains(verboseOutput, "Full annotation listing") {
		t.Error("verbose report should contain 'Full annotation listing'")
	}
}

func TestFormatCheckReportMarkdown(t *testing.T) {
	report := &CheckReport{
		TotalFiles:   2,
		IngressFiles: 1,
		Converted:    1,
		Manual:       1,
		Files: []FileReport{
			{
				Path:      "templates/ingress.yaml",
				IsIngress: true,
				Findings: []AnnotationFinding{
					{Annotation: "nginx.ingress.kubernetes.io/affinity", Status: StatusManual, Detail: "BackendTrafficPolicy"},
				},
			},
		},
	}

	output := FormatCheckReportMarkdown(report)
	if !strings.Contains(output, "# Ingress Annotation Migration Check") {
		t.Error("markdown report should contain heading")
	}
	if !strings.Contains(output, "| `nginx.ingress.kubernetes.io/affinity`") {
		t.Error("markdown report should contain annotation in table")
	}
}
