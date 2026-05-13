package apisix

// InputFormat describes how the input Ingress resources were structured.
type InputFormat int

const (
	// FormatSingleDoc means the input was a single Ingress document.
	FormatSingleDoc InputFormat = iota
	// FormatList means the input was a single IngressList document.
	FormatList
	// FormatMultiDoc means the input was multiple Ingress documents separated by "---".
	FormatMultiDoc
)

// ApisixPluginConfig represents an apisix.apache.org/v2 ApisixPluginConfig CRD.
// Used when a plugin's configuration is too complex for an annotation.
type ApisixPluginConfig struct {
	APIVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Metadata   Metadata         `yaml:"metadata"`
	Spec       PluginConfigSpec `yaml:"spec"`
}

// BackendTrafficPolicy represents an APISIX BackendTrafficPolicy CRD used for
// upstream load-balancing policies such as cookie-based consistent hashing.
type BackendTrafficPolicy struct {
	APIVersion string                   `yaml:"apiVersion"`
	Kind       string                   `yaml:"kind"`
	Metadata   Metadata                 `yaml:"metadata"`
	Spec       BackendTrafficPolicySpec `yaml:"spec"`
}

type BackendTrafficPolicySpec struct {
	TargetRefs   []PolicyTargetRef   `yaml:"targetRefs"`
	LoadBalancer BackendLoadBalancer `yaml:"loadbalancer"`
}

type PolicyTargetRef struct {
	Group string `yaml:"group"`
	Kind  string `yaml:"kind"`
	Name  string `yaml:"name"`
}

type BackendLoadBalancer struct {
	Type   string `yaml:"type"`
	HashOn string `yaml:"hashOn"`
	Key    string `yaml:"key"`
}

type PluginConfigSpec struct {
	Plugins []Plugin `yaml:"plugins"`
}

// Plugin represents a single APISIX plugin entry.
type Plugin struct {
	Name   string      `yaml:"name"`
	Enable bool        `yaml:"enable"`
	Config interface{} `yaml:"config,omitempty"`
}

// Metadata is the standard K8s object metadata subset.
type Metadata struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// ConversionResult holds all output from the conversion.
type ConversionResult struct {
	// Ingresses are the converted Ingress resources with APISIX annotations.
	Ingresses []interface{}
	// PluginConfigs are ApisixPluginConfig CRDs for complex plugin configs.
	PluginConfigs []ApisixPluginConfig
	// BackendTrafficPolicies are CRDs for upstream load-balancing policies.
	BackendTrafficPolicies []BackendTrafficPolicy
	// Errors encountered during conversion.
	Errors []error
	// Warnings encountered during conversion.
	Warnings []string
	// InputFormat records how the original input was structured.
	InputFormat InputFormat
}

// ConversionOptions controls how the conversion is performed.
type ConversionOptions struct {
	// DefaultNamespace is used when Ingress has no namespace.
	DefaultNamespace string
	// ApisixVersion is the target APISIX CRD API version.
	ApisixVersion string
	// TargetIngressClassName sets the ingressClassName on output Ingresses.
	TargetIngressClassName string
	// SSLRedirect generates ssl-redirect annotation for TLS hosts.
	SSLRedirect bool
}

func DefaultConversionOptions() ConversionOptions {
	return ConversionOptions{
		DefaultNamespace:       "default",
		ApisixVersion:          "apisix.apache.org/v2",
		TargetIngressClassName: "apisix",
		SSLRedirect:            true,
	}
}
