package ingress

// IngressList represents a list of Kubernetes Ingress resources (or a single
// Ingress wrapped in a YAML document separator).
type IngressList struct {
	Items []Ingress `yaml:"items"`
}

// IngressOutputList is used for writing an IngressList to YAML output.
// Items are interface{} because they may be Ingress structs with APISIX annotations.
type IngressOutputList struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Items      []interface{} `yaml:"items"`
}

// Ingress is a simplified representation of a networking.k8s.io/v1 Ingress.
type Ingress struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   Metadata       `yaml:"metadata"`
	Spec       IngressSpec    `yaml:"spec"`
	Status     *IngressStatus `yaml:"status,omitempty"`
}

type Metadata struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

type IngressSpec struct {
	IngressClassName *string         `yaml:"ingressClassName,omitempty"`
	TLS              []IngressTLS    `yaml:"tls,omitempty"`
	Rules            []IngressRule   `yaml:"rules,omitempty"`
	DefaultBackend   *IngressBackend `yaml:"defaultBackend,omitempty"`
}

type IngressTLS struct {
	Hosts      []string `yaml:"hosts,omitempty"`
	SecretName string   `yaml:"secretName,omitempty"`
}

type IngressRule struct {
	Host string                `yaml:"host,omitempty"`
	HTTP *HTTPIngressRuleValue `yaml:"http,omitempty"`
}

type HTTPIngressRuleValue struct {
	Paths []HTTPIngressPath `yaml:"paths"`
}

type HTTPIngressPath struct {
	Path     string         `yaml:"path,omitempty"`
	PathType *string        `yaml:"pathType,omitempty"`
	Backend  IngressBackend `yaml:"backend"`
}

type IngressBackend struct {
	Service *IngressServiceBackend `yaml:"service,omitempty"`
}

type IngressServiceBackend struct {
	Name string             `yaml:"name"`
	Port ServiceBackendPort `yaml:"port"`
}

type ServiceBackendPort struct {
	Name   string `yaml:"name,omitempty"`
	Number int32  `yaml:"number,omitempty"`
}

type IngressStatus struct {
	LoadBalancer *LoadBalancerStatus `yaml:"loadBalancer,omitempty"`
}

type LoadBalancerStatus struct {
	Ingress []LoadBalancerIngress `yaml:"ingress,omitempty"`
}

type LoadBalancerIngress struct {
	IP string `yaml:"ip,omitempty"`
}
