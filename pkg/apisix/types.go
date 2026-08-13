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
	// PluginSources maps plugin name → source nginx annotation suffix(es).
	// Not marshalled to YAML; used for source comment injection.
	PluginSources map[string]string `yaml:"-"`
	// PluginFieldDefaults maps "pluginName.configKey" → nginx annotation key
	// for plugin config fields that use default values.
	// Not marshalled to YAML; used for default hint comment injection.
	PluginFieldDefaults map[string]string `yaml:"-"`
}

// BackendTrafficPolicy represents an APISIX BackendTrafficPolicy CRD used for
// upstream load-balancing policies such as cookie-based consistent hashing.
type BackendTrafficPolicy struct {
	APIVersion string                   `yaml:"apiVersion"`
	Kind       string                   `yaml:"kind"`
	Metadata   Metadata                 `yaml:"metadata"`
	Spec       BackendTrafficPolicySpec `yaml:"spec"`
	// SourceAnnotations maps spec field paths to source nginx annotation suffix(es).
	// Not marshalled to YAML; used for source comment injection.
	SourceAnnotations map[string]string `yaml:"-"`
}

type BackendTrafficPolicySpec struct {
	TargetRefs   []PolicyTargetRef   `yaml:"targetRefs"`
	LoadBalancer BackendLoadBalancer `yaml:"loadbalancer,omitempty"`
}

type PolicyTargetRef struct {
	Group string `yaml:"group"`
	Kind  string `yaml:"kind"`
	Name  string `yaml:"name"`
}

type BackendLoadBalancer struct {
	Type   string `yaml:"type"`
	HashOn string `yaml:"hashOn,omitempty"`
	Key    string `yaml:"key,omitempty"`
}

// ApisixUpstream represents an apisix.apache.org/v2 ApisixUpstream CRD.
// Used for upstream-level configuration such as keepalive pools.
type ApisixUpstream struct {
	APIVersion string             `yaml:"apiVersion"`
	Kind       string             `yaml:"kind"`
	Metadata   Metadata           `yaml:"metadata"`
	Spec       ApisixUpstreamSpec `yaml:"spec"`
	// SourceAnnotations maps spec field paths to source nginx annotation suffix(es).
	// Not marshalled to YAML; used for source comment injection.
	SourceAnnotations map[string]string `yaml:"-"`
}

type ApisixUpstreamSpec struct {
	IngressClassName     string `yaml:"ingressClassName,omitempty"`
	ApisixUpstreamConfig `yaml:",inline"`
}

type ApisixUpstreamConfig struct {
	LoadBalancer *BackendLoadBalancer `yaml:"loadbalancer,omitempty"`
	Scheme       string               `yaml:"scheme,omitempty"`
	Retries      *int                 `yaml:"retries,omitempty"`
	Timeout      *UpstreamTimeout     `yaml:"timeout,omitempty"`
	HealthCheck  *HealthCheck         `yaml:"healthCheck,omitempty"`
	PassHost     string               `yaml:"passHost,omitempty"`
	UpstreamHost string               `yaml:"upstreamHost,omitempty"`
}

type UpstreamTimeout struct {
	Connect string `yaml:"connect,omitempty"`
	Send    string `yaml:"send,omitempty"`
	Read    string `yaml:"read,omitempty"`
}

// HealthCheck defines active/passive health check config for upstream nodes.
// Matches AIC ApisixUpstreamConfig.HealthCheck schema.
type HealthCheck struct {
	Active  *ActiveHealthCheck  `yaml:"active"`
	Passive *PassiveHealthCheck `yaml:"passive,omitempty"`
}

type ActiveHealthCheck struct {
	Type           string                      `yaml:"type,omitempty"`
	Timeout        string                      `yaml:"timeout,omitempty"`
	HTTPPath       string                      `yaml:"httpPath,omitempty"`
	Host           string                      `yaml:"host,omitempty"`
	RequestHeaders []string                    `yaml:"requestHeaders,omitempty"`
	Healthy        *ActiveHealthCheckHealthy   `yaml:"healthy,omitempty"`
	Unhealthy      *ActiveHealthCheckUnhealthy `yaml:"unhealthy,omitempty"`
}

type ActiveHealthCheckHealthy struct {
	HTTPCodes []int  `yaml:"httpCodes,omitempty"`
	Successes int    `yaml:"successes,omitempty"`
	Interval  string `yaml:"interval,omitempty"`
}

type ActiveHealthCheckUnhealthy struct {
	HTTPCodes    []int  `yaml:"httpCodes,omitempty"`
	TCPFailures  int    `yaml:"tcpFailures,omitempty"`
	Timeouts     int    `yaml:"timeout,omitempty"`
	HTTPFailures int    `yaml:"httpFailures,omitempty"`
	Interval     string `yaml:"interval,omitempty"`
}

type PassiveHealthCheck struct {
	Type      string                       `yaml:"type,omitempty"`
	Healthy   *PassiveHealthCheckHealthy   `yaml:"healthy,omitempty"`
	Unhealthy *PassiveHealthCheckUnhealthy `yaml:"unhealthy,omitempty"`
}

type PassiveHealthCheckHealthy struct {
	HTTPCodes []int `yaml:"httpCodes,omitempty"`
	Successes int   `yaml:"successes,omitempty"`
}

type PassiveHealthCheckUnhealthy struct {
	HTTPCodes    []int `yaml:"httpCodes,omitempty"`
	TCPFailures  int   `yaml:"tcpFailures,omitempty"`
	Timeouts     int   `yaml:"timeout,omitempty"`
	HTTPFailures int   `yaml:"httpFailures,omitempty"`
}

// ApisixTls represents an apisix.apache.org/v2 ApisixTls CRD.
// Used for TLS termination configuration with certificate references.
type ApisixTls struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   Metadata      `yaml:"metadata"`
	Spec       ApisixTlsSpec `yaml:"spec"`
	// SourceAnnotations maps spec field paths to source nginx annotation suffix(es).
	// Not marshalled to YAML; used for source comment injection.
	SourceAnnotations map[string]string `yaml:"-"`
}

type ApisixTlsSpec struct {
	IngressClassName string            `yaml:"ingressClassName,omitempty"`
	Hosts            []string          `yaml:"hosts"`
	Secret           ApisixSecret      `yaml:"secret"`
	Client           *ApisixMtlsClient `yaml:"client,omitempty"`
}

type ApisixSecret struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type ApisixMtlsClient struct {
	CASecret ApisixSecret `yaml:"caSecret,omitempty"`
	Depth    int          `yaml:"depth,omitempty"`
}

type PluginConfigSpec struct {
	// IngressClassName is the name of an IngressClass cluster resource.
	// The controller uses this field to decide whether the resource should be managed.
	IngressClassName string   `yaml:"ingressClassName,omitempty"`
	Plugins          []Plugin `yaml:"plugins"`
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
	// SourceAnnotations maps output annotation key → source nginx annotation(s).
	// Used for YAML comment injection. Not marshalled to YAML.
	SourceAnnotations map[string]string `yaml:"-"`
}

// ConversionResult holds all output from the conversion.
type ConversionResult struct {
	// Ingresses are the converted Ingress resources with APISIX annotations.
	Ingresses []interface{}
	// PluginConfigs are ApisixPluginConfig CRDs for complex plugin configs.
	PluginConfigs []ApisixPluginConfig
	// BackendTrafficPolicies are CRDs for upstream load-balancing policies.
	BackendTrafficPolicies []BackendTrafficPolicy
	// ApisixUpstreams are CRDs for upstream-level configuration (keepalive, timeouts, etc.).
	ApisixUpstreams []ApisixUpstream
	// ApisixTls are CRDs for TLS/SSL configuration derived from Ingress spec.tls.
	ApisixTls []ApisixTls
	// GatewayClasses are Gateway API GatewayClass resources (Gateway API mode only).
	GatewayClasses []GatewayClass
	// Gateways are Gateway API Gateway resources (Gateway API mode only).
	Gateways []Gateway
	// HTTPRoutes are Gateway API HTTPRoute resources (Gateway API mode only).
	HTTPRoutes []HTTPRoute
	// Errors encountered during conversion.
	Errors []error
	// Warnings encountered during conversion.
	Warnings []string
	// InputFormat records how the original input was structured.
	InputFormat InputFormat
	// CRDHints maps annotation name → complete CRD YAML example for manual migration.
	// Displayed after warnings with syntax highlighting.
	CRDHints map[string]string
}

// --- Gateway API types ---

// GatewayClass represents a gateway.networking.k8s.io/v1 GatewayClass resource.
type GatewayClass struct {
	APIVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Metadata   Metadata         `yaml:"metadata"`
	Spec       GatewayClassSpec `yaml:"spec"`
}

type GatewayClassSpec struct {
	ControllerName string `yaml:"controllerName"`
}

// Gateway represents a gateway.networking.k8s.io/v1 Gateway resource.
type Gateway struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   Metadata    `yaml:"metadata"`
	Spec       GatewaySpec `yaml:"spec"`
}

type GatewaySpec struct {
	GatewayClassName string     `yaml:"gatewayClassName"`
	Listeners        []Listener `yaml:"listeners"`
}

type Listener struct {
	Name     string       `yaml:"name"`
	Protocol string       `yaml:"protocol"`
	Port     int          `yaml:"port"`
	Hostname string       `yaml:"hostname,omitempty"`
	TLS      *ListenerTLS `yaml:"tls,omitempty"`
}

type ListenerTLS struct {
	Mode     string    `yaml:"mode"`
	CertRefs []CertRef `yaml:"certificateRefs,omitempty"`
}

type CertRef struct {
	Kind      string `yaml:"kind"`
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

// HTTPRoute represents a gateway.networking.k8s.io/v1 HTTPRoute resource.
type HTTPRoute struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   Metadata      `yaml:"metadata"`
	Spec       HTTPRouteSpec `yaml:"spec"`
}

type HTTPRouteSpec struct {
	ParentRefs []ParentRef     `yaml:"parentRefs,omitempty"`
	Hostnames  []string        `yaml:"hostnames,omitempty"`
	Rules      []HTTPRouteRule `yaml:"rules,omitempty"`
}

type ParentRef struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

type HTTPRouteRule struct {
	Matches     []HTTPRouteMatch  `yaml:"matches,omitempty"`
	Filters     []HTTPRouteFilter `yaml:"filters,omitempty"`
	BackendRefs []BackendRef      `yaml:"backendRefs,omitempty"`
}

type HTTPRouteMatch struct {
	Path *PathMatch `yaml:"path,omitempty"`
}

type PathMatch struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

type BackendRef struct {
	Name string `yaml:"name"`
	Port int    `yaml:"port"`
}

type HTTPRouteFilter struct {
	Type            string               `yaml:"type"`
	URLRewrite      *URLRewrite          `yaml:"urlRewrite,omitempty"`
	RequestRedirect *HTTPRequestRedirect `yaml:"requestRedirect,omitempty"`
	ExtensionRef    *ExtensionRef        `yaml:"extensionRef,omitempty"`
}

// ExtensionRef references a custom resource for Gateway API ExtensionRef filters.
// Used to link ApisixPluginConfig or other CRDs from an HTTPRoute rule.
type ExtensionRef struct {
	Group string `yaml:"group"`
	Kind  string `yaml:"kind"`
	Name  string `yaml:"name"`
}

type URLRewrite struct {
	Path     *PathModifier `yaml:"path,omitempty"`
	Hostname string        `yaml:"hostname,omitempty"`
}

type PathModifier struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

type HTTPRequestRedirect struct {
	Scheme     string `yaml:"scheme,omitempty"`
	Hostname   string `yaml:"hostname,omitempty"`
	StatusCode int    `yaml:"statusCode,omitempty"`
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
