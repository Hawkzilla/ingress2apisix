package converter

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ingress2apisix/pkg/apisix"
	"github.com/ingress2apisix/pkg/ingress"
)

// ParsedInput holds the parsed ingresses along with the original input format.
type ParsedInput struct {
	Ingresses []ingress.Ingress
	Format    apisix.InputFormat
}

// ReadIngressFile reads one or more Ingress YAML documents from a file.
func ReadIngressFile(path string) (ParsedInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ParsedInput{}, fmt.Errorf("reading file %s: %w", path, err)
	}
	return ParseIngressYAML(data)
}

// ParseIngressYAML parses YAML bytes into Ingress objects.
// Returns a ParsedInput that preserves whether the source was a single Ingress,
// an IngressList, or multi-document YAML.
func ParseIngressYAML(data []byte) (ParsedInput, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var all []ingress.Ingress
	var docCount int
	var hadList bool

	for {
		var raw yaml.Node
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return ParsedInput{}, fmt.Errorf("decoding YAML: %w", err)
		}

		kind, err := extractKind(&raw)
		if err != nil {
			continue
		}

		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&raw); err != nil {
			return ParsedInput{}, fmt.Errorf("re-encoding YAML node: %w", err)
		}
		enc.Close()

		docData := buf.Bytes()
		docCount++

		switch kind {
		case "Ingress":
			var ing ingress.Ingress
			if err := yaml.Unmarshal(docData, &ing); err != nil {
				return ParsedInput{}, fmt.Errorf("unmarshalling Ingress: %w", err)
			}
			if ing.APIVersion == "" {
				ing.APIVersion = "networking.k8s.io/v1"
			}
			all = append(all, ing)

		case "IngressList":
			var list ingress.IngressList
			if err := yaml.Unmarshal(docData, &list); err != nil {
				return ParsedInput{}, fmt.Errorf("unmarshalling IngressList: %w", err)
			}
			hadList = true
			for _, ing := range list.Items {
				if ing.APIVersion == "" {
					ing.APIVersion = "networking.k8s.io/v1"
				}
				all = append(all, ing)
			}

		case "List":
			// A Kubernetes List can contain mixed resource types.
			// We need to filter for Ingress items only.
			var list ingress.IngressList
			if err := yaml.Unmarshal(docData, &list); err != nil {
				return ParsedInput{}, fmt.Errorf("unmarshalling List: %w", err)
			}
			hadList = true
			for _, ing := range list.Items {
				if ing.Kind != "" && ing.Kind != "Ingress" {
					continue
				}
				if ing.APIVersion == "" {
					ing.APIVersion = "networking.k8s.io/v1"
				}
				all = append(all, ing)
			}

		default:
			continue
		}
	}

	if len(all) == 0 {
		return ParsedInput{}, fmt.Errorf("no Ingress resources found in the input")
	}

	// Determine input format
	var format apisix.InputFormat
	if hadList {
		format = apisix.FormatList
	} else if docCount == 1 {
		format = apisix.FormatSingleDoc
	} else {
		format = apisix.FormatMultiDoc
	}

	return ParsedInput{Ingresses: all, Format: format}, nil
}

// extractKind extracts the "kind" field from a YAML node tree.
func extractKind(node *yaml.Node) (string, error) {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return "", fmt.Errorf("expected mapping node")
	}

	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]
		if key.Value == "kind" {
			return val.Value, nil
		}
	}
	return "", fmt.Errorf("kind field not found")
}

// marshalWithIndent marshals a value to YAML with 2-space indentation.
func marshalWithIndent(v interface{}) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	enc.Close()
	return buf.String(), nil
}

// WriteConversionResult writes all output resources as multi-document YAML.
// When the original input was an IngressList, the converted Ingresses are
// wrapped into a single IngressList document instead of being written individually.
// For Gateway API results (containing GatewayClasses/Gateways/HTTPRoutes),
// it delegates to WriteGatewayAPIResult.
func WriteConversionResult(w io.Writer, result apisix.ConversionResult) error {
	// Gateway API mode: delegate to dedicated writer
	if len(result.GatewayClasses) > 0 || len(result.Gateways) > 0 || len(result.HTTPRoutes) > 0 {
		return WriteGatewayAPIResult(w, result)
	}

	first := true

	writeDoc := func(yamlStr string) error {
		if !first {
			fmt.Fprint(w, "---\n")
		}
		first = false
		_, err := io.WriteString(w, yamlStr)
		return err
	}

	// If original input was an IngressList, output as IngressList too
	if result.InputFormat == apisix.FormatList && len(result.Ingresses) > 0 {
		apiVersion := "networking.k8s.io/v1"
		if ing, ok := result.Ingresses[0].(ingress.Ingress); ok && ing.APIVersion != "" {
			apiVersion = ing.APIVersion
		}

		list := ingress.IngressOutputList{
			APIVersion: apiVersion,
			Kind:       "IngressList",
			Items:      result.Ingresses,
		}
		yamlStr, err := marshalWithIndent(&list)
		if err != nil {
			return fmt.Errorf("encoding IngressList: %w", err)
		}
		// Apply source comments from the first Ingress in the list
		if len(result.Ingresses) > 0 {
			if ing, ok := result.Ingresses[0].(ingress.Ingress); ok && ing.Metadata.SourceAnnotations != nil {
				yamlStr = addAnnotationSourceComments(yamlStr, ing.Metadata.SourceAnnotations)
			}
		}
		if err := writeDoc(yamlStr); err != nil {
			return err
		}
	} else {
		// Write each Ingress as a separate document
		for _, obj := range result.Ingresses {
			yamlStr, err := marshalWithIndent(obj)
			if err != nil {
				return fmt.Errorf("encoding Ingress: %w", err)
			}
			if ing, ok := obj.(ingress.Ingress); ok {
				yamlStr = addSourceCommentsForResource(yamlStr, ing.Metadata.SourceAnnotations, nil, nil, nil)
			}
			if err := writeDoc(yamlStr); err != nil {
				return err
			}
		}
	}

	// Write ApisixPluginConfig resources (always as individual documents)
	for _, p := range result.PluginConfigs {
		yamlStr, err := marshalWithIndent(&p)
		if err != nil {
			return fmt.Errorf("encoding ApisixPluginConfig %s: %w", p.Metadata.Name, err)
		}
		yamlStr = addSourceCommentsForResource(yamlStr, nil, p.PluginSources, nil, p.PluginFieldDefaults)
		if err := writeDoc(yamlStr); err != nil {
			return err
		}
	}

	// Write BackendTrafficPolicy resources (always as individual documents)
	for _, p := range result.BackendTrafficPolicies {
		yamlStr, err := marshalWithIndent(&p)
		if err != nil {
			return fmt.Errorf("encoding BackendTrafficPolicy %s: %w", p.Metadata.Name, err)
		}
		yamlStr = addSourceCommentsForResource(yamlStr, nil, nil, p.SourceAnnotations, nil)
		if err := writeDoc(yamlStr); err != nil {
			return err
		}
	}

	// Write ApisixUpstream resources (always as individual documents)
	for _, u := range result.ApisixUpstreams {
		yamlStr, err := marshalWithIndent(&u)
		if err != nil {
			return fmt.Errorf("encoding ApisixUpstream %s: %w", u.Metadata.Name, err)
		}
		yamlStr = addSourceCommentsForResource(yamlStr, nil, nil, u.SourceAnnotations, nil)
		if err := writeDoc(yamlStr); err != nil {
			return err
		}
	}

	// Write ApisixTls resources (always as individual documents)
	for _, t := range result.ApisixTls {
		yamlStr, err := marshalWithIndent(&t)
		if err != nil {
			return fmt.Errorf("encoding ApisixTls %s: %w", t.Metadata.Name, err)
		}
		yamlStr = addSourceCommentsForResource(yamlStr, nil, nil, t.SourceAnnotations, nil)
		if err := writeDoc(yamlStr); err != nil {
			return err
		}
	}

	return nil
}

// WriteConversionFile writes the conversion result to a file.
func WriteConversionFile(path string, result apisix.ConversionResult) error {
	var buf bytes.Buffer

	fmt.Fprintln(&buf, "# Generated by ingress2apisix")
	fmt.Fprintf(&buf, "# Total: %d ingresses, %d pluginConfigs, %d backendTrafficPolicies, %d apisixUpstreams, %d apisixTls\n",
		len(result.Ingresses), len(result.PluginConfigs), len(result.BackendTrafficPolicies), len(result.ApisixUpstreams), len(result.ApisixTls))
	fmt.Fprintln(&buf, "---")

	if err := WriteConversionResult(&buf, result); err != nil {
		return err
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}

// FormatResultSummary returns a human-readable summary of the conversion.
func FormatResultSummary(result apisix.ConversionResult) string {
	var sb strings.Builder
	sb.WriteString("Conversion Summary:\n")

	formatStr := "single-document"
	switch result.InputFormat {
	case apisix.FormatList:
		formatStr = "IngressList"
	case apisix.FormatMultiDoc:
		formatStr = "multi-document"
	}
	sb.WriteString(fmt.Sprintf("  Input format:     %s\n", formatStr))
	sb.WriteString(fmt.Sprintf("  Ingress:          %d\n", len(result.Ingresses)))
	sb.WriteString(fmt.Sprintf("  ApisixPluginConfig: %d\n", len(result.PluginConfigs)))
	sb.WriteString(fmt.Sprintf("  BackendTrafficPolicy: %d\n", len(result.BackendTrafficPolicies)))
	sb.WriteString(fmt.Sprintf("  ApisixUpstream:     %d\n", len(result.ApisixUpstreams)))
	sb.WriteString(fmt.Sprintf("  ApisixTls:          %d\n", len(result.ApisixTls)))

	if len(result.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf("\nWarnings (%d):\n", len(result.Warnings)))
		for _, w := range result.Warnings {
			sb.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}

	if len(result.Errors) > 0 {
		sb.WriteString(fmt.Sprintf("\nErrors (%d):\n", len(result.Errors)))
		for _, e := range result.Errors {
			sb.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	return sb.String()
}
