package converter

import (
	"strings"
)

const (
	// defaultPrefix marks source values that represent defaults.
	// Renders as: # Default — set <annotation> to customize
	defaultPrefix = "DEFAULT:"
	// defaultHintPrefix marks source values that have a custom hint text.
	// Renders as: # Default — <hint text>
	defaultHintPrefix = "DEFAULT_HINT:"
)

// formatSourceComment formats a source value into a YAML comment.
// If the source starts with "DEFAULT:", it renders a default hint comment.
// If the source starts with "DEFAULT_HINT:", it renders a custom hint comment.
// Otherwise it renders a standard source comment.
func formatSourceComment(src string) string {
	if strings.HasPrefix(src, defaultHintPrefix) {
		hint := src[len(defaultHintPrefix):]
		return "# Default — " + hint
	}
	if strings.HasPrefix(src, defaultPrefix) {
		annotation := src[len(defaultPrefix):]
		return "# Default — set " + annotation + " to customize"
	}
	return "# Source: " + src
}

// addAnnotationSourceComments injects YAML comments showing which original nginx
// annotation produced each APISIX annotation. It works by scanning lines for
// known annotation keys and appending a "# Source: ..." or "# Default — ..." comment.
//
// sources maps output annotation key → original nginx annotation key.
// Source values prefixed with "DEFAULT:" are rendered as default hint comments.
func addAnnotationSourceComments(yamlStr string, sources map[string]string) string {
	if len(sources) == 0 {
		return yamlStr
	}

	lines := strings.Split(yamlStr, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for key, src := range sources {
			// Match lines like:  k8s.apisix.apache.org/enable-cors: "true"
			// or:  k8s.apisix.apache.org/rewrite-target: /
			prefix := key + ":"
			if strings.HasPrefix(trimmed, prefix) {
				lines[i] = line + "  " + formatSourceComment(src)
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

// addPluginSourceCommentsV2 is a robust version that handles the multi-pass issue
// by processing all plugins in a single pass. It also handles plugin field defaults.
func addPluginSourceCommentsV2(yamlStr string, pluginSources map[string]string) string {
	if len(pluginSources) == 0 {
		return yamlStr
	}

	lines := strings.Split(yamlStr, "\n")
	var result []string

	inPlugins := false
	pluginsIndent := -1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect the "plugins:" key
		if trimmed == "plugins:" && !inPlugins {
			inPlugins = true
			pluginsIndent = len(line) - len(strings.TrimLeft(line, " "))
			result = append(result, line)
			continue
		}

		if !inPlugins {
			result = append(result, line)
			continue
		}

		// Check if we've exited the plugins section
		currentIndent := len(line) - len(strings.TrimLeft(line, " "))
		if len(trimmed) > 0 && currentIndent <= pluginsIndent {
			inPlugins = false
			result = append(result, line)
			continue
		}

		// Look for plugin name entries: "- name: plugin-name" or "name: plugin-name"
		nameValue := ""
		if strings.HasPrefix(trimmed, "- name:") {
			nameValue = strings.TrimSpace(strings.TrimPrefix(trimmed, "- name:"))
		} else if strings.HasPrefix(trimmed, "name:") {
			nameValue = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
		}
		if nameValue != "" {
			nameValue = strings.Trim(nameValue, "\"'")
			if src, ok := pluginSources[nameValue]; ok {
				commentIndent := strings.Repeat(" ", currentIndent)
				result = append(result, commentIndent+formatSourceComment(src))
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// addPluginFieldDefaultComments injects default hint comments into plugin config
// YAML for fields that use default values.
//
// fieldDefaults maps "pluginName.configKey" → nginx annotation key.
// For example: "session-cookie-hash.cookie_name" → "nginx.ingress.kubernetes.io/session-cookie-name"
//
// The function tracks the current plugin name and injects comments on config
// field lines that match entries in fieldDefaults.
func addPluginFieldDefaultComments(yamlStr string, fieldDefaults map[string]string) string {
	if len(fieldDefaults) == 0 {
		return yamlStr
	}

	// Copy the map since we mutate it during iteration
	remaining := make(map[string]string, len(fieldDefaults))
	for k, v := range fieldDefaults {
		remaining[k] = v
	}

	lines := strings.Split(yamlStr, "\n")
	var result []string

	inPlugins := false
	pluginsIndent := -1
	currentPlugin := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect the "plugins:" key
		if trimmed == "plugins:" && !inPlugins {
			inPlugins = true
			pluginsIndent = len(line) - len(strings.TrimLeft(line, " "))
			result = append(result, line)
			continue
		}

		if !inPlugins {
			result = append(result, line)
			continue
		}

		// Check if we've exited the plugins section
		currentIndent := len(line) - len(strings.TrimLeft(line, " "))
		if len(trimmed) > 0 && currentIndent <= pluginsIndent {
			inPlugins = false
			currentPlugin = ""
			result = append(result, line)
			continue
		}

		// Track current plugin name
		nameValue := ""
		if strings.HasPrefix(trimmed, "- name:") {
			nameValue = strings.TrimSpace(strings.TrimPrefix(trimmed, "- name:"))
		} else if strings.HasPrefix(trimmed, "name:") {
			nameValue = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
		}
		if nameValue != "" {
			currentPlugin = strings.Trim(nameValue, "\"'")
		}

		matched := false
		// If we're inside a plugin's config block, check for field defaults
		if currentPlugin != "" {
			for defaultKey, annotation := range remaining {
				parts := strings.SplitN(defaultKey, ".", 2)
				if len(parts) != 2 {
					continue
				}
				pluginName, configKey := parts[0], parts[1]
				if pluginName != currentPlugin {
					continue
				}
				// Match "configKey: value" or "configKey:" at the start of the trimmed line
				if strings.HasPrefix(trimmed, configKey+":") {
					comment := formatSourceComment(annotation)
					result = append(result, line+"  "+comment)
					delete(remaining, defaultKey)
					matched = true
					break
				}
			}
		}

		if !matched {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// addSourceCommentsForResource is the main entry point. Given a marshalled YAML
// string and source tracking data from the converter, it injects appropriate
// source comments.
//
// annotationSources maps annotation key → source nginx annotation (for Ingress metadata.annotations)
// pluginSources maps plugin name → source nginx annotation (for PluginConfig plugins)
// fieldSources maps dotted field path → source nginx annotation (for other CRD fields)
// pluginFieldDefaults maps "pluginName.configKey" → nginx annotation for default hints in plugin configs
func addSourceCommentsForResource(yamlStr string, annotationSources, pluginSources, fieldSources map[string]string, pluginFieldDefaults map[string]string) string {
	if annotationSources == nil && pluginSources == nil && fieldSources == nil && pluginFieldDefaults == nil {
		return yamlStr
	}

	result := yamlStr

	// Phase 1: Add comments to annotation values
	if len(annotationSources) > 0 {
		result = addAnnotationSourceComments(result, annotationSources)
	}

	// Phase 2: Add comments before plugin names
	if len(pluginSources) > 0 {
		result = addPluginSourceCommentsV2(result, pluginSources)
	}

	// Phase 3: Add comments to spec fields using field path matching
	if len(fieldSources) > 0 {
		result = addFieldSourceComments(result, fieldSources)
	}

	// Phase 4: Add default hint comments to plugin config fields
	if len(pluginFieldDefaults) > 0 {
		result = addPluginFieldDefaultComments(result, pluginFieldDefaults)
	}

	return result
}

// addFieldSourceComments injects comments into YAML for spec fields.
// fieldSources maps dotted field paths (e.g. "healthCheck.active.httpPath",
// "loadbalancer", "hosts") to source nginx annotations.
// For dotted paths, the leaf key (last segment) is matched against YAML lines.
func addFieldSourceComments(yamlStr string, fieldSources map[string]string) string {
	if len(fieldSources) == 0 {
		return yamlStr
	}

	// Build a map of leaf YAML key → source annotation
	leafToSrc := make(map[string]string)
	for path, src := range fieldSources {
		parts := strings.Split(path, ".")
		leafKey := parts[len(parts)-1]
		leafToSrc[leafKey] = src
	}

	lines := strings.Split(yamlStr, "\n")
	matched := make(map[string]bool) // track which leaf keys we've already annotated
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for leafKey, src := range leafToSrc {
			if matched[leafKey] {
				continue
			}
			// Match YAML key at start of the trimmed line:
			// "leafKey:" or "leafKey: value"
			if strings.HasPrefix(trimmed, leafKey+":") {
				lines[i] = line + "  " + formatSourceComment(src)
				matched[leafKey] = true
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}
