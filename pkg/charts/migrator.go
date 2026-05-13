package charts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MigrateOptions controls migration behavior.
type MigrateOptions struct {
	Backup bool // Create .bak files before modifying
	DryRun bool // Show diffs without modifying files
}

// MigrateReport summarizes the result of a migration run.
type MigrateReport struct {
	FilesProcessed int
	FilesModified  int
	PluginConfigs  int
	Warnings       []string
	Diffs          []FileDiff
}

// FileDiff represents before/after content of a modified file.
type FileDiff struct {
	Path   string
	Before string
	After  string
}

// pluginConfigData holds plugin configuration collected from a single Ingress.
type pluginConfigData struct {
	ingressName      string
	pluginConfigName string
	plugins          []pluginEntry
	values           map[string]string // key → default value for values.yaml
	isHelm           bool              // whether this is from a Helm template
}

// pluginEntry represents a single APISIX plugin configuration block.
type pluginEntry struct {
	name   string
	config string // YAML config block
}

// annotationEntry represents a single annotation found in a YAML file.
type annotationEntry struct {
	suffix      string // e.g. "ssl-redirect"
	value       string // raw value (may contain {{ .Values.xxx }})
	fullKey     string // full annotation key with prefix
	lineNum     int    // 0-indexed
	isMultiLine bool
	blockLines  []string // for multi-line values (after | or >)
	blockEnd    int      // last line of multi-line block
}

// annotationMigration describes how to transform a single annotation.
type annotationMigration struct {
	action    string                    // "rename", "remove", "pluginConfig"
	target    string                    // target suffix (for "rename")
	transform func(value string) string // value transformer (nil = identity)
}

// reAnnotationKey matches annotation key lines in YAML.
var (
	reAnnotationKey          = regexp.MustCompile(`^(\s*)((?:nginx\.ingress\.kubernetes\.io|ingress\.kubernetes\.io)/([\w-]+))\s*:\s*(.*)$`)
	reIngressClassAnnotation = regexp.MustCompile(`^(\s*)(kubernetes\.io/ingress\.class)\s*:\s*(.*)$`)
	reIngressClass           = regexp.MustCompile(`(?m)^(\s*ingressClassName\s*:\s*)(nginx)(\s*)$`)
	reCommented              = regexp.MustCompile(`^\s*#`)
)

// ensureTimeSuffix adds 's' suffix to timeout values if not already present.
func ensureTimeSuffix(v string) string {
	v = stripQuotes(strings.TrimSpace(v))
	if strings.HasSuffix(v, "s") || strings.HasSuffix(v, "ms") {
		return v
	}
	return v + "s"
}

// stripQuotes removes surrounding quotes from a YAML value.
func stripQuotes(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

// regexCapturePattern matches $1, $2, ... in rewrite targets.
var regexCapturePattern = regexp.MustCompile(`\$\d+`)

// isHelmTemplate returns true if value contains Go template syntax.
func isHelmTemplate(v string) bool {
	return strings.Contains(v, "{{") && strings.Contains(v, "}}")
}

// --- Migration mapping table ---

var migrationTable = map[string]annotationMigration{
	// CORS
	"enable-cors":        {action: "rename", target: "enable-cors", transform: nil},
	"cors-allow-origin":  {action: "rename", target: "cors-allow-origin", transform: nil},
	"cors-allow-methods": {action: "rename", target: "cors-allow-methods", transform: nil},
	"cors-allow-headers": {action: "rename", target: "cors-allow-headers", transform: nil},

	// SSL redirect
	"ssl-redirect":       {action: "rename", target: "http-to-https", transform: nil},
	"force-ssl-redirect": {action: "rename", target: "http-to-https", transform: func(_ string) string { return "true" }},

	// Rewrite
	"rewrite-target": {action: "rename", target: "rewrite-target", transform: func(v string) string {
		if regexCapturePattern.MatchString(v) {
			// Will be handled by applyMigrations to set rewrite-target-regex
			return v
		}
		return v
	}},

	// Proxy timeouts
	"proxy-connect-timeout": {action: "rename", target: "upstream-connect-timeout", transform: func(v string) string {
		if isHelmTemplate(v) {
			return v
		}
		return ensureTimeSuffix(v)
	}},
	"proxy-send-timeout": {action: "rename", target: "upstream-send-timeout", transform: func(v string) string {
		if isHelmTemplate(v) {
			return v
		}
		return ensureTimeSuffix(v)
	}},
	"proxy-read-timeout": {action: "rename", target: "upstream-read-timeout", transform: func(v string) string {
		if isHelmTemplate(v) {
			return v
		}
		return ensureTimeSuffix(v)
	}},

	// Backend protocol
	"backend-protocol": {action: "rename", target: "upstream-scheme", transform: func(v string) string {
		if isHelmTemplate(v) {
			return v
		}
		return strings.ToLower(stripQuotes(v))
	}},

	// Access control
	"whitelist-source-range": {action: "rename", target: "allowlist-source-range", transform: nil},

	// External auth
	"auth-url":              {action: "rename", target: "auth-uri", transform: nil},
	"auth-response-headers": {action: "rename", target: "auth-upstream-headers", transform: nil},
	"auth-type": {action: "rename", target: "auth-type", transform: func(v string) string {
		if isHelmTemplate(v) {
			return v
		}
		switch strings.ToLower(stripQuotes(v)) {
		case "basic":
			return "basicAuth"
		case "digest":
			return "keyAuth"
		default:
			return stripQuotes(v)
		}
	}},

	// WebSocket
	"websocket-services": {action: "rename", target: "enable-websocket", transform: func(_ string) string { return "true" }},

	// Regex
	"use-regex": {action: "rename", target: "use-regex", transform: nil},

	// Proxy redirect (paired handling in applyMigrations)
	"proxy-redirect-from": {action: "rename", target: "http-to-https", transform: nil},
	"proxy-redirect-to":   {action: "rename", target: "http-to-https", transform: nil},

	// PluginConfig (removed from annotations, handled separately)
	"limit-rps":         {action: "pluginConfig", target: "", transform: nil},
	"limit-rpm":         {action: "pluginConfig", target: "", transform: nil},
	"limit-connections": {action: "pluginConfig", target: "", transform: nil},
	"proxy-cookie-path": {action: "pluginConfig", target: "", transform: nil},

	// configuration-snippet: complex, handled in processSnippet
	"configuration-snippet": {action: "snippet", target: "", transform: nil},
}

// knownManualAnnotations are recognized but require manual intervention.
var knownManualAnnotations = map[string]string{
	"custom-http-errors":     "需自定义 Lua 插件 (custom-error-page)",
	"affinity":               "需 BackendTrafficPolicy CRD",
	"session-cookie-name":    "需 BackendTrafficPolicy CRD",
	"session-cookie-hash":    "需 BackendTrafficPolicy CRD",
	"upstream-hash-by":       "APISIX Ingress 无等价原生注解，需 BackendTrafficPolicy 或 ApisixRoute/ApisixUpstream",
	"auth-tls-secret":        "需 ApisixTls CRD (mTLS)",
	"auth-tls-verify-client": "需 ApisixTls CRD (mTLS)",
	"auth-secret":            "需 ApisixConsumer CRD",
	"proxy-body-size":        "需全局配置 client_max_body_size",
	"proxy-buffer-size":      "需全局配置 proxy_buffer_size",
	"proxy-buffers-number":   "需全局配置 proxy_buffers",
}

// MigrateChartsDir walks a directory and modifies Ingress YAML files in-place.
func MigrateChartsDir(dir string, opts MigrateOptions) (*MigrateReport, error) {
	report := &MigrateReport{}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !reYAMLFile.MatchString(info.Name()) {
			return nil
		}

		relPath, _ := filepath.Rel(dir, path)

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		content := string(data)
		if !reIngressKind.MatchString(content) {
			report.FilesProcessed++
			return nil
		}

		newContent, configs, warns, changed := migrateFileContent(content, relPath)
		report.Warnings = append(report.Warnings, warns...)
		report.FilesProcessed++

		if !changed {
			return nil
		}

		report.FilesModified++
		report.PluginConfigs += len(configs)

		if opts.DryRun {
			report.Diffs = append(report.Diffs, FileDiff{
				Path:   relPath,
				Before: content,
				After:  newContent,
			})
			return nil
		}

		if opts.Backup {
			bakPath := path + ".bak"
			if err := os.WriteFile(bakPath, data, 0644); err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("failed to create backup %s: %v", bakPath, err))
			}
		}

		if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("failed to write %s: %v", relPath, err))
			return nil
		}

		// Generate PluginConfig template if needed
		if len(configs) > 0 {
			pluginTmpl := generatePluginConfigTemplate(configs)
			tmplDir := filepath.Dir(path)
			pluginPath := filepath.Join(tmplDir, "apisix-plugin-configs.yaml")

			if opts.Backup {
				if existing, err := os.ReadFile(pluginPath); err == nil {
					_ = os.WriteFile(pluginPath+".bak", existing, 0644)
				}
			}

			if err := os.WriteFile(pluginPath, []byte(pluginTmpl), 0644); err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("failed to write plugin config template: %v", err))
			}

			// Update values.yaml in chart root
			chartDir := findChartRoot(path)
			if chartDir != "" {
				valuesPath := filepath.Join(chartDir, "values.yaml")
				if err := updateValuesYaml(valuesPath, configs); err != nil {
					report.Warnings = append(report.Warnings, fmt.Sprintf("failed to update values.yaml: %v", err))
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(report.Warnings)
	return report, nil
}

// reMetadataName matches metadata: → name: in YAML.
var reMetadataName = regexp.MustCompile(`^\s+name\s*:\s*(.+)`)

// migrateFileContent transforms a single file's content. Returns new content,
// plugin config data, warnings, and whether content changed.
func migrateFileContent(content, fileName string) (string, []pluginConfigData, []string, bool) {
	lines := strings.Split(content, "\n")
	entries := scanAnnotations(lines)
	isHelmFile := reHelmTemplate.MatchString(content)

	// Extract ingress name from metadata.name
	ingressName := ""
	inMetadata := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "metadata:" {
			inMetadata = true
			continue
		}
		if inMetadata && reMetadataName.MatchString(line) {
			m := reMetadataName.FindStringSubmatch(line)
			if len(m) > 1 {
				name := strings.Trim(m[1], "\"'")
				if isHelmTemplate(name) {
					ingressName = "ingress"
				} else {
					ingressName = name
				}
			}
			break
		}
		if inMetadata && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
			inMetadata = false
		}
	}
	if ingressName == "" {
		ingressName = "ingress"
	}

	result, configs, warns := applyMigrations(lines, entries, ingressName, isHelmFile)

	newContent := strings.Join(result, "\n")

	// Fix ingressClassName: nginx → apisix
	newContent = reIngressClass.ReplaceAllString(newContent, "${1}apisix${3}")

	// Add plugin-config-name annotation if plugin configs were generated
	if len(configs) > 0 {
		for _, cfg := range configs {
			pcName := cfg.pluginConfigName
			if cfg.isHelm {
				pcName = fmt.Sprintf("{{ .Release.Name }}-%s", cfg.pluginConfigName)
			}
			annotation := fmt.Sprintf("k8s.apisix.apache.org/plugin-config-name: %s", pcName)
			newContent = addAnnotationToContent(newContent, annotation, ingressName)
		}
	}

	changed := newContent != content || len(configs) > 0
	return newContent, configs, warns, changed
}

// addAnnotationToContent inserts an annotation into the annotations block of an Ingress.
func addAnnotationToContent(content, annotation, ingressName string) string {
	lines := strings.Split(content, "\n")
	inAnnotations := false
	annotationsLine := -1
	lastAnnotationLine := -1
	annotationIndent := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "annotations:" {
			inAnnotations = true
			annotationsLine = i
			annotationIndent = len(line) - len(strings.TrimLeft(line, " \t")) + 2
			continue
		}
		if inAnnotations {
			if trimmed == "" {
				continue
			}
			lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
			if lineIndent >= annotationIndent {
				lastAnnotationLine = i
			} else {
				break
			}
		}
	}

	insertAt := lastAnnotationLine
	if insertAt < 0 {
		insertAt = annotationsLine
	}

	if insertAt >= 0 {
		indent := strings.Repeat(" ", annotationIndent)
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:insertAt+1]...)
		newLines = append(newLines, indent+annotation)
		newLines = append(newLines, lines[insertAt+1:]...)
		return strings.Join(newLines, "\n")
	}

	return content
}

// scanAnnotations scans lines for annotation entries.
func scanAnnotations(lines []string) []annotationEntry {
	var entries []annotationEntry
	seen := make(map[string]bool)

	for i := 0; i < len(lines); i++ {
		if reCommented.MatchString(lines[i]) {
			continue
		}

		m := reAnnotationKey.FindStringSubmatch(lines[i])
		if m == nil || len(m) < 5 {
			// Also check for kubernetes.io/ingress.class
			if cm := reIngressClassAnnotation.FindStringSubmatch(lines[i]); cm != nil {
				entries = append(entries, annotationEntry{
					suffix:  "kubernetes.io/ingress.class",
					value:   strings.TrimSpace(cm[3]),
					fullKey: "kubernetes.io/ingress.class",
					lineNum: i,
				})
			}
			continue
		}

		prefix := m[1]
		fullKey := m[2]
		suffix := m[3]
		value := strings.TrimSpace(m[4])

		// Skip if already an APISIX annotation
		if strings.Contains(fullKey, "k8s.apisix.apache.org") {
			continue
		}

		// Skip kubernetes.io/ingress.class (handled separately)
		if strings.Contains(fullKey, "kubernetes.io/ingress.class") {
			entries = append(entries, annotationEntry{
				suffix:  "kubernetes.io/ingress.class",
				value:   value,
				fullKey: fullKey,
				lineNum: i,
			})
			continue
		}

		// De-duplicate: nginx prefix takes precedence
		key := suffix
		if seen[key] {
			// If current is nginx prefix and previous was plain, override
			if strings.HasPrefix(fullKey, "nginx.ingress.kubernetes.io") {
				// Mark previous entry for removal, replace with this one
				for j := range entries {
					if entries[j].suffix == suffix && !strings.HasPrefix(entries[j].fullKey, "nginx.ingress.kubernetes.io") {
						entries[j] = annotationEntry{
							suffix:  suffix,
							value:   value,
							fullKey: fullKey,
							lineNum: i,
						}
					}
				}
			}
			continue
		}
		seen[key] = true

		entry := annotationEntry{
			suffix:  suffix,
			value:   value,
			fullKey: fullKey,
			lineNum: i,
		}

		// Check for multi-line value (| or >)
		if value == "|" || value == ">" {
			entry.isMultiLine = true
			var block []string
			indent := len(prefix) + 2 // base indent for block content
			j := i + 1
			for j < len(lines) {
				line := lines[j]
				trimmed := strings.TrimLeft(line, " ")
				if trimmed == "" {
					block = append(block, "")
					j++
					continue
				}
				lineIndent := len(line) - len(trimmed)
				if lineIndent < indent && trimmed != "" {
					break
				}
				block = append(block, line)
				j++
			}
			entry.blockLines = block
			entry.blockEnd = j - 1
		}

		entries = append(entries, entry)
	}

	return entries
}

// applyMigrations transforms the lines based on collected annotation entries.
func applyMigrations(lines []string, entries []annotationEntry, ingressName string, isHelm bool) ([]string, []pluginConfigData, []string) {
	// Collect lines to remove (set of line indices)
	removeLines := make(map[int]bool)
	// Collect lines to append after the annotation block
	insertAfter := map[int][]string{} // lineNum → lines to insert
	var configs []pluginConfigData
	var warnings []string

	// Track paired annotations
	hasProxyRedirectFrom := false
	hasProxyRedirectTo := false

	// Collect plugin config data
	var pluginEntries []pluginEntry
	valuesMap := make(map[string]string)

	// First pass: mark lines for removal and collect plugin data
	for _, entry := range entries {
		// Remove kubernetes.io/ingress.class
		if entry.suffix == "kubernetes.io/ingress.class" {
			removeLines[entry.lineNum] = true
			continue
		}

		migration, handled := migrationTable[entry.suffix]
		if !handled {
			// Check known manual annotations
			if hint, ok := knownManualAnnotations[entry.suffix]; ok {
				// Add TODO comment before the line
				indent := extractIndent(lines[entry.lineNum])
				comment := indent + fmt.Sprintf("# TODO [ingress2apisix]: %s — %s", entry.suffix, hint)
				insertAfter[entry.lineNum] = []string{comment}
			}
			continue
		}

		switch migration.action {
		case "rename":
			// Handle paired proxy-redirect-from/to
			if entry.suffix == "proxy-redirect-from" {
				hasProxyRedirectFrom = true
				removeLines[entry.lineNum] = true
				if entry.isMultiLine {
					for l := entry.lineNum + 1; l <= entry.blockEnd; l++ {
						removeLines[l] = true
					}
				}
				continue
			}
			if entry.suffix == "proxy-redirect-to" {
				hasProxyRedirectTo = true
				removeLines[entry.lineNum] = true
				if entry.isMultiLine {
					for l := entry.lineNum + 1; l <= entry.blockEnd; l++ {
						removeLines[l] = true
					}
				}
				continue
			}

			// rewrite-target special handling
			if entry.suffix == "rewrite-target" {
				targetSuffix := migration.target
				if regexCapturePattern.MatchString(entry.value) {
					targetSuffix = "rewrite-target-regex"
				}
				newKey := "k8s.apisix.apache.org/" + targetSuffix
				transformed := entry.value
				if migration.transform != nil {
					transformed = migration.transform(entry.value)
				}
				replaceAnnotationLine(lines, &entry, newKey, transformed, removeLines, insertAfter)
				continue
			}

			newKey := "k8s.apisix.apache.org/" + migration.target
			transformed := entry.value
			if migration.transform != nil {
				transformed = migration.transform(entry.value)
			}
			replaceAnnotationLine(lines, &entry, newKey, transformed, removeLines, insertAfter)

		case "remove":
			removeLines[entry.lineNum] = true
			if entry.isMultiLine {
				for l := entry.lineNum + 1; l <= entry.blockEnd; l++ {
					removeLines[l] = true
				}
			}

		case "pluginConfig":
			removeLines[entry.lineNum] = true
			if entry.isMultiLine {
				for l := entry.lineNum + 1; l <= entry.blockEnd; l++ {
					removeLines[l] = true
				}
			}

			switch entry.suffix {
			case "limit-rps":
				val := entry.value
				valuesKey := "limitRps"
				if isHelmTemplate(val) {
					// Keep template reference
					pluginEntries = append(pluginEntries, pluginEntry{
						name: "limit-req",
						config: fmt.Sprintf(`        rate: %s
        burst: 0
        key: remote_addr
        rejected_code: 429`, val),
					})
				} else {
					val = strings.Trim(val, "\"")
					valuesMap[valuesKey] = val
					pluginEntries = append(pluginEntries, pluginEntry{
						name: "limit-req",
						config: `        rate: {{ .Values.limitRps | default "100" }}
        burst: 0
        key: remote_addr
        rejected_code: 429`,
					})
				}

			case "limit-rpm":
				val := entry.value
				valuesKey := "limitRpm"
				if isHelmTemplate(val) {
					pluginEntries = append(pluginEntries, pluginEntry{
						name: "limit-req",
						config: fmt.Sprintf(`        rate: %s
        burst: 0
        key: remote_addr
        rejected_code: 429`, val),
					})
				} else {
					val = strings.Trim(val, "\"")
					valuesMap[valuesKey] = val
					pluginEntries = append(pluginEntries, pluginEntry{
						name: "limit-req",
						config: `        rate: {{ .Values.limitRpm | default "100" }}/min
        burst: 0
        key: remote_addr
        rejected_code: 429`,
					})
				}

			case "limit-connections":
				val := entry.value
				valuesKey := "limitConnections"
				if isHelmTemplate(val) {
					pluginEntries = append(pluginEntries, pluginEntry{
						name: "limit-conn",
						config: fmt.Sprintf(`        conn: %s
        burst: 0
        key: remote_addr
        rejected_code: 503`, val),
					})
				} else {
					val = strings.Trim(val, "\"")
					valuesMap[valuesKey] = val
					pluginEntries = append(pluginEntries, pluginEntry{
						name: "limit-conn",
						config: `        conn: {{ .Values.limitConnections | default "100" }}
        burst: 0
        key: remote_addr
                        rejected_code: 503`,
					})
				}

			case "proxy-cookie-path":
				val := entry.value
				if isHelmTemplate(val) {
					pluginEntries = append(pluginEntries, pluginEntry{
						name:   "proxy-cookie-path",
						config: fmt.Sprintf("        path_pairs:\n          - match: \"/old-path\"\n            replacement: %s", val),
					})
				} else {
					pluginEntries = append(pluginEntries, pluginEntry{
						name:   "proxy-cookie-path",
						config: buildProxyCookiePathConfig(val),
					})
				}
			}

		case "snippet":
			// Handle configuration-snippet
			var snippetLines []string
			if entry.isMultiLine {
				snippetLines = entry.blockLines
			} else {
				snippetLines = []string{entry.value}
			}

			var remaining []string
			var snippetWarns []string
			pluginEntries, remaining, snippetWarns = processSnippet(entry, snippetLines, pluginEntries, valuesMap)
			warnings = append(warnings, snippetWarns...)

			// Remove the entire snippet line and block
			removeLines[entry.lineNum] = true
			if entry.isMultiLine {
				for l := entry.lineNum + 1; l <= entry.blockEnd; l++ {
					removeLines[l] = true
				}
			}

			// Re-add snippet if there are remaining (unhandled) directives
			if len(remaining) > 0 {
				indent := extractIndent(lines[entry.lineNum])
				var newSnippet strings.Builder
				newSnippet.WriteString(indent + "nginx.ingress.kubernetes.io/configuration-snippet: |\n")
				for _, sl := range remaining {
					newSnippet.WriteString(indent + "  " + sl + "\n")
				}
				// Insert after the annotation block
				lastLine := entry.lineNum
				if entry.isMultiLine {
					lastLine = entry.blockEnd
				}
				insertAfter[lastLine] = append(insertAfter[lastLine],
					strings.TrimRight(newSnippet.String(), "\n"))
			}
		}
	}

	// Handle proxy-redirect-from/to pair
	if hasProxyRedirectFrom && hasProxyRedirectTo {
		// Find the indentation from one of the entries
		indent := "    " // default
		for _, e := range entries {
			if e.suffix == "proxy-redirect-from" {
				indent = extractIndent(lines[e.lineNum])
				break
			}
		}
		// Insert the APISIX equivalent
		// Find a good insertion point (after annotations block start)
		for _, e := range entries {
			if e.suffix == "proxy-redirect-from" {
				insertAfter[e.lineNum] = append(insertAfter[e.lineNum],
					indent+"k8s.apisix.apache.org/http-to-https: \"true\"")
				break
			}
		}
	}

	// If we collected plugin config data, build a config entry
	if len(pluginEntries) > 0 {
		pcName := ingressName + "-plugins"
		if isHelm {
			// Template already prepends {{ .Release.Name }}-, so just use the short name
			pcName = ingressName + "-plugins"
		}
		if len(pcName) > 64 {
			pcName = pcName[:64]
		}

		configs = append(configs, pluginConfigData{
			ingressName:      ingressName,
			pluginConfigName: pcName,
			plugins:          pluginEntries,
			values:           valuesMap,
			isHelm:           isHelm,
		})
	}

	// Reconstruct lines, skipping removed lines and inserting new lines
	var result []string
	for i := 0; i < len(lines); i++ {
		if removeLines[i] {
			// Still check insertAfter for removed lines (replaceAnnotationLine uses this)
			if inserts, ok := insertAfter[i]; ok {
				result = append(result, inserts...)
			}
			continue
		}
		result = append(result, lines[i])
		if inserts, ok := insertAfter[i]; ok {
			result = append(result, inserts...)
		}
	}

	// Clean up empty lines at end
	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}

	return result, configs, warnings
}

// replaceAnnotationLine replaces an annotation line with a new key and value.
func replaceAnnotationLine(lines []string, entry *annotationEntry, newKey, newValue string, removeLines map[int]bool, insertAfter map[int][]string) {
	indent := extractIndent(lines[entry.lineNum])
	quoted := quoteValue(newValue)
	newLine := fmt.Sprintf("%s%s: %s", indent, newKey, quoted)

	// Remove old line and its multi-line block
	removeLines[entry.lineNum] = true
	if entry.isMultiLine {
		for l := entry.lineNum + 1; l <= entry.blockEnd; l++ {
			removeLines[l] = true
		}
	}

	insertAfter[entry.lineNum] = append(insertAfter[entry.lineNum], newLine)
}

// processSnippet handles configuration-snippet directives.
func processSnippet(entry annotationEntry, snippetLines []string, pluginEntries []pluginEntry, valuesMap map[string]string) ([]pluginEntry, []string, []string) {
	rewritePattern := regexp.MustCompile(`rewrite\s+(\S+)\s+(\S+)\s+`)
	cookieFlagsPattern := regexp.MustCompile(`proxy_cookie_flags\s+(.+)`)
	limitReqStatusPattern := regexp.MustCompile(`limit_req_status\s+(\d{3})`)

	var rewriteURIs []string
	var remaining []string // lines to keep
	var cookieFlagRules []string
	var warnings []string

	for _, line := range snippetLines {
		line = strings.TrimSpace(line)
		if line == "" {
			remaining = append(remaining, line)
			continue
		}

		// rewrite directive
		if m := rewritePattern.FindStringSubmatch(line); len(m) > 2 {
			rewriteURIs = append(rewriteURIs, m[1], m[2])
			continue // consumed
		}

		// proxy_cookie_flags directive
		if m := cookieFlagsPattern.FindStringSubmatch(line); len(m) > 1 {
			cookieFlagRules = append(cookieFlagRules, strings.TrimRight(m[1], "; "))
			continue // consumed
		}

		// limit_req_status: consumed by limit-rps handling, skip
		if limitReqStatusPattern.MatchString(line) {
			continue // consumed
		}

		// more_set_headers: keep in snippet with comment
		if strings.Contains(line, "more_set_headers") {
			remaining = append(remaining, line)
			continue
		}

		// custom-http-errors in snippet: keep with comment
		if strings.Contains(line, "custom-http-errors") {
			remaining = append(remaining, line)
			continue
		}

		// Unknown directive: keep
		remaining = append(remaining, line)
	}

	// Generate proxy-rewrite plugin for multiple rewrites
	if len(rewriteURIs) > 2 {
		var regexURIs []string
		for j := 0; j < len(rewriteURIs)-1; j += 2 {
			regexURIs = append(regexURIs, fmt.Sprintf("          - \"%s\"", rewriteURIs[j]))
			regexURIs = append(regexURIs, fmt.Sprintf("          - \"%s\"", rewriteURIs[j+1]))
		}
		pluginEntries = append(pluginEntries, pluginEntry{
			name:   "proxy-rewrite",
			config: "        regex_uri:\n" + strings.Join(regexURIs, "\n"),
		})
	}

	// Generate proxy-cookie-flags plugin
	if len(cookieFlagRules) > 0 {
		var rulesYAML []string
		for _, rule := range cookieFlagRules {
			parts := strings.Fields(rule)
			if len(parts) < 2 {
				continue
			}
			matchPattern := parts[0]
			flags := parts[1:]
			rulesYAML = append(rulesYAML, fmt.Sprintf("          - match: \"%s\"", matchPattern))
			flagsYAML := "            flags:\n"
			for _, f := range flags {
				flagsYAML += fmt.Sprintf("              - \"%s\"\n", f)
			}
			rulesYAML = append(rulesYAML, strings.TrimRight(flagsYAML, "\n"))
		}
		if len(rulesYAML) > 0 {
			pluginEntries = append(pluginEntries, pluginEntry{
				name:   "proxy-cookie-flags",
				config: "        rules:\n" + strings.Join(rulesYAML, "\n"),
			})
		}
	}

	return pluginEntries, remaining, warnings
}

// generatePluginConfigTemplate generates a Helm template for ApisixPluginConfig.
func generatePluginConfigTemplate(configs []pluginConfigData) string {
	var sb strings.Builder
	for i, cfg := range configs {
		if i > 0 {
			sb.WriteString("---\n")
		}

		// Build the name field based on whether it's a Helm template
		nameField := cfg.pluginConfigName
		if cfg.isHelm {
			nameField = fmt.Sprintf("{{ .Release.Name }}-%s", cfg.pluginConfigName)
		}

		sb.WriteString(fmt.Sprintf(`{{- if .Values.ingress2ApisixPluginConfigEnabled | default true }}
apiVersion: apisix.apache.org/v2
kind: ApisixPluginConfig
metadata:
  name: %s
  namespace: {{ .Release.Namespace }}
  labels:
    managed-by: ingress2apisix
spec:
  plugins:
`, nameField))

		for _, p := range cfg.plugins {
			sb.WriteString(fmt.Sprintf("    - name: %s\n", p.name))
			sb.WriteString("      enable: true\n")
			sb.WriteString("      config:\n")
			sb.WriteString(p.config + "\n")
		}
		sb.WriteString("{{- end }}\n")
	}
	return sb.String()
}

// buildProxyCookiePathConfig builds a proxy-cookie-path plugin config.
func buildProxyCookiePathConfig(value string) string {
	value = stripQuotes(strings.TrimSpace(value))
	lines := strings.Split(value, "\n")
	var pairs []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if it's a regex pattern (starts with ~)
		if strings.HasPrefix(line, "~") {
			// "~ pattern replacement" or "~pattern replacement"
			rest := strings.TrimPrefix(line, "~")
			rest = strings.TrimSpace(rest)
			parts := strings.SplitN(rest, " ", 2)
			if len(parts) >= 2 {
				match := strings.TrimSpace(parts[0])
				replacement := strings.TrimSpace(parts[1])
				pairs = append(pairs, fmt.Sprintf(`          - match: "~ %s"`, match))
				pairs = append(pairs, fmt.Sprintf(`            replacement: "%s"`, replacement))
			}
		} else {
			// Simple pattern: "pattern replacement"
			parts := strings.SplitN(line, " ", 2)
			if len(parts) >= 2 {
				match := strings.TrimSpace(parts[0])
				replacement := strings.TrimSpace(parts[1])
				pairs = append(pairs, fmt.Sprintf(`          - match: "%s"`, match))
				pairs = append(pairs, fmt.Sprintf(`            replacement: "%s"`, replacement))
			}
		}
	}

	if len(pairs) == 0 {
		return `        path_pairs:
          - match: "/old-path"
            replacement: "/new-path"`
	}
	return "        path_pairs:\n" + strings.Join(pairs, "\n")
}

// updateValuesYaml adds new default values to values.yaml.
func updateValuesYaml(valuesPath string, configs []pluginConfigData) error {
	newEntries := make(map[string]string)
	for _, cfg := range configs {
		for k, v := range cfg.values {
			newEntries[k] = v
		}
	}
	if len(newEntries) == 0 {
		return nil
	}

	var content string
	if data, err := os.ReadFile(valuesPath); err == nil {
		content = string(data)
	} else {
		content = ""
	}

	// Append new entries
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n# ingress2apisix generated defaults\n"
	keys := make([]string, 0, len(newEntries))
	for k := range newEntries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		content += fmt.Sprintf("%s: \"%s\"\n", k, newEntries[k])
	}

	return os.WriteFile(valuesPath, []byte(content), 0644)
}

// findChartRoot walks up from a file to find Chart.yaml.
func findChartRoot(filePath string) string {
	dir := filepath.Dir(filePath)
	for {
		if _, err := os.Stat(filepath.Join(dir, "Chart.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// extractIndent returns the leading whitespace of a line.
func extractIndent(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	return line[:len(line)-len(trimmed)]
}

// quoteValue ensures a YAML value is properly quoted.
func quoteValue(v string) string {
	v = strings.TrimSpace(v)
	// If it already has matching quotes, leave it
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v
		}
	}
	// Helm template expressions should not be quoted
	if isHelmTemplate(v) {
		return v
	}
	// If it contains special characters, quote it
	if strings.ContainsAny(v, ": #{}[]&*!|>',\"@`") {
		return "\"" + strings.ReplaceAll(v, "\"", "\\\"") + "\""
	}
	return v
}

// FormatMigrateReport returns a human-readable migration report.
func FormatMigrateReport(report *MigrateReport, verbose bool) string {
	var sb strings.Builder

	sb.WriteString("\n=== Charts Migration Report ===\n\n")
	sb.WriteString(fmt.Sprintf("Files processed: %d\n", report.FilesProcessed))
	sb.WriteString(fmt.Sprintf("Files modified:  %d\n", report.FilesModified))
	sb.WriteString(fmt.Sprintf("Plugin configs:  %d\n", report.PluginConfigs))

	if len(report.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf("\nWarnings (%d):\n", len(report.Warnings)))
		for _, w := range report.Warnings {
			sb.WriteString(fmt.Sprintf("  ! %s\n", w))
		}
	}

	if verbose && len(report.Diffs) > 0 {
		sb.WriteString("\n--- Diffs ---\n")
		for _, d := range report.Diffs {
			sb.WriteString(fmt.Sprintf("\n  %s:\n", d.Path))
			sb.WriteString(fmt.Sprintf("  --- before\n  +++ after\n"))
			sb.WriteString(fmt.Sprintf("  %d lines → %d lines\n",
				strings.Count(d.Before, "\n")+1, strings.Count(d.After, "\n")+1))
		}
	}

	return sb.String()
}
