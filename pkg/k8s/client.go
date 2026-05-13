package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	networkingV1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/ingress2apisix/pkg/apisix"
	"github.com/ingress2apisix/pkg/converter"
	"github.com/ingress2apisix/pkg/ingress"
)

// ClusterClient wraps Kubernetes and dynamic clients for reading Ingress
// resources and applying conversion results back to the cluster.
type ClusterClient struct {
	k8s     kubernetes.Interface
	dynamic dynamic.Interface
	ns      string // namespace filter; empty = all namespaces
}

// NewClusterClient creates a ClusterClient from kubeconfig path and context.
// If kubeconfig is empty, uses InClusterConfig or default kubeconfig.
// If ns is non-empty, only operates on that namespace.
func NewClusterClient(kubeconfig, ctx, ns string) (*ClusterClient, error) {
	cfg, err := buildRestConfig(kubeconfig, ctx)
	if err != nil {
		return nil, fmt.Errorf("building kubernetes config: %w", err)
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &ClusterClient{k8s: k8sClient, dynamic: dynClient, ns: ns}, nil
}

// buildRestConfig resolves kubeconfig path and builds a rest.Config.
func buildRestConfig(kubeconfig, ctx string) (*rest.Config, error) {
	if kubeconfig == "" {
		// Try in-cluster config first
		if cfg, err := rest.InClusterConfig(); err == nil {
			return cfg, nil
		}
		// Fall back to default kubeconfig
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if ctx != "" {
		overrides.CurrentContext = ctx
	}

	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	return cfg.ClientConfig()
}

// ReadIngressFromCluster fetches Ingress resources from the cluster,
// returning a ParsedInput suitable for conversion.
func (c *ClusterClient) ReadIngressFromCluster(ctx context.Context) (converter.ParsedInput, error) {
	ns := c.ns
	if ns == "" {
		ns = "" // all namespaces
	}

	ingList, err := c.k8s.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return converter.ParsedInput{}, fmt.Errorf("listing ingresses: %w", err)
	}

	var all []ingress.Ingress
	for i := range ingList.Items {
		k8sIng := &ingList.Items[i]
		ing := convertK8sIngress(k8sIng)
		all = append(all, ing)
	}

	if len(all) == 0 {
		return converter.ParsedInput{}, fmt.Errorf("no Ingress resources found in cluster (namespace=%q)", c.ns)
	}

	// When reading from cluster, always treat as multi-document
	return converter.ParsedInput{Ingresses: all, Format: apisix.FormatMultiDoc}, nil
}

// convertK8sIngress converts a real k8s networking/v1 Ingress to our simplified struct.
func convertK8sIngress(k8sIng *networkingV1.Ingress) ingress.Ingress {
	ing := ingress.Ingress{
		APIVersion: "networking.k8s.io/v1",
		Kind:       "Ingress",
		Metadata: ingress.Metadata{
			Name:        k8sIng.Name,
			Namespace:   k8sIng.Namespace,
			Labels:      copyMap(k8sIng.Labels),
			Annotations: copyMap(k8sIng.Annotations),
		},
	}

	if k8sIng.Spec.IngressClassName != nil {
		ing.Spec.IngressClassName = k8sIng.Spec.IngressClassName
	}

	for _, tls := range k8sIng.Spec.TLS {
		ing.Spec.TLS = append(ing.Spec.TLS, ingress.IngressTLS{
			Hosts:      tls.Hosts,
			SecretName: tls.SecretName,
		})
	}

	for _, rule := range k8sIng.Spec.Rules {
		ir := ingress.IngressRule{Host: rule.Host}
		if rule.HTTP != nil {
			httpRule := &ingress.HTTPIngressRuleValue{}
			for _, p := range rule.HTTP.Paths {
				pt := string(*p.PathType)
				path := ingress.HTTPIngressPath{
					Path:     p.Path,
					PathType: &pt,
					Backend: ingress.IngressBackend{
						Service: &ingress.IngressServiceBackend{
							Name: p.Backend.Service.Name,
							Port: ingress.ServiceBackendPort{
								Number: p.Backend.Service.Port.Number,
								Name:   p.Backend.Service.Port.Name,
							},
						},
					},
				}
				httpRule.Paths = append(httpRule.Paths, path)
			}
			ir.HTTP = httpRule
		}
		ing.Spec.Rules = append(ing.Spec.Rules, ir)
	}

	return ing
}

// copyMap returns a shallow copy of a map, or nil if the source is nil.
func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// ApplyResult applies the conversion result to the cluster.
// It updates Ingress annotations/ingressClassName and creates/updates generated APISIX CRDs.
// If dryRun is true, it only prints what would be done.
func (c *ClusterClient) ApplyResult(ctx context.Context, result apisix.ConversionResult, dryRun bool) error {
	// Apply Ingress updates
	for _, obj := range result.Ingresses {
		ing, ok := obj.(ingress.Ingress)
		if !ok {
			continue
		}
		if err := c.applyIngress(ctx, ing, dryRun); err != nil {
			return err
		}
	}

	// Apply ApisixPluginConfig CRDs
	for _, pc := range result.PluginConfigs {
		if err := c.applyPluginConfig(ctx, pc, dryRun); err != nil {
			return err
		}
	}

	for _, btp := range result.BackendTrafficPolicies {
		if err := c.applyBackendTrafficPolicy(ctx, btp, dryRun); err != nil {
			return err
		}
	}

	return nil
}

// applyIngress patches an existing Ingress's annotations and ingressClassName.
func (c *ClusterClient) applyIngress(ctx context.Context, ing ingress.Ingress, dryRun bool) error {
	ns := ing.Metadata.Namespace
	if ns == "" {
		ns = "default"
	}

	patch := buildIngressPatch(ing)

	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] PATCH Ingress %s/%s\n", ns, ing.Metadata.Name)
		fmt.Fprintf(os.Stderr, "  annotations: %v\n", ing.Metadata.Annotations)
		if ing.Spec.IngressClassName != nil {
			fmt.Fprintf(os.Stderr, "  ingressClassName: %s\n", *ing.Spec.IngressClassName)
		}
		return nil
	}

	_, err := c.k8s.NetworkingV1().Ingresses(ns).Patch(
		ctx,
		ing.Metadata.Name,
		types.MergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("patching Ingress %s/%s: %w", ns, ing.Metadata.Name, err)
	}

	fmt.Fprintf(os.Stderr, "[updated] Ingress %s/%s\n", ns, ing.Metadata.Name)
	return nil
}

// buildIngressPatch builds a JSON merge patch payload for the Ingress.
func buildIngressPatch(ing ingress.Ingress) []byte {
	var buf bytes.Buffer
	buf.WriteString(`{"metadata":{"annotations":{`)

	first := true
	for k, v := range ing.Metadata.Annotations {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		buf.WriteString(fmt.Sprintf("%q:%q", k, v))
	}

	buf.WriteString(`}},"spec":{`)
	if ing.Spec.IngressClassName != nil {
		buf.WriteString(fmt.Sprintf(`"ingressClassName":%q`, *ing.Spec.IngressClassName))
	}
	buf.WriteString(`}}`)

	return buf.Bytes()
}

// apisixPluginConfigGVR is the GroupVersionResource for ApisixPluginConfig CRD.
var apisixPluginConfigGVR = schema.GroupVersionResource{
	Group:    "apisix.apache.org",
	Version:  "v2",
	Resource: "apisixpluginconfigs",
}

var backendTrafficPolicyGVR = schema.GroupVersionResource{
	Group:    "apisix.apache.org",
	Version:  "v1alpha1",
	Resource: "backendtrafficpolicies",
}

// applyPluginConfig creates or updates an ApisixPluginConfig CRD using the dynamic client.
func (c *ClusterClient) applyPluginConfig(ctx context.Context, pc apisix.ApisixPluginConfig, dryRun bool) error {
	ns := pc.Metadata.Namespace
	if ns == "" {
		ns = "default"
	}

	obj := buildUnstructuredPluginConfig(pc)

	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] Apply ApisixPluginConfig %s/%s\n", ns, pc.Metadata.Name)
		return nil
	}

	client := c.dynamic.Resource(apisixPluginConfigGVR).Namespace(ns)

	// Try to get existing resource
	existing, err := client.Get(ctx, pc.Metadata.Name, metav1.GetOptions{})
	if err != nil {
		// Not found → create
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("creating ApisixPluginConfig %s/%s: %w", ns, pc.Metadata.Name, err)
		}
		fmt.Fprintf(os.Stderr, "[created] ApisixPluginConfig %s/%s\n", ns, pc.Metadata.Name)
		return nil
	}

	// Exists → update (preserve resourceVersion)
	obj.SetResourceVersion(existing.GetResourceVersion())
	_, err = client.Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating ApisixPluginConfig %s/%s: %w", ns, pc.Metadata.Name, err)
	}
	fmt.Fprintf(os.Stderr, "[updated] ApisixPluginConfig %s/%s\n", ns, pc.Metadata.Name)
	return nil
}

// buildUnstructuredPluginConfig converts an ApisixPluginConfig to an unstructured object
// for the dynamic client.
func buildUnstructuredPluginConfig(pc apisix.ApisixPluginConfig) *unstructured.Unstructured {
	plugins := make([]interface{}, len(pc.Spec.Plugins))
	for i, p := range pc.Spec.Plugins {
		plugin := map[string]interface{}{
			"name":   p.Name,
			"enable": p.Enable,
		}
		if p.Config != nil {
			plugin["config"] = p.Config
		}
		plugins[i] = plugin
	}

	// Convert labels to map[string]interface{} for deep copy compatibility
	var labels map[string]interface{}
	if pc.Metadata.Labels != nil {
		labels = make(map[string]interface{}, len(pc.Metadata.Labels))
		for k, v := range pc.Metadata.Labels {
			labels[k] = v
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": pc.APIVersion,
			"kind":       "ApisixPluginConfig",
			"metadata": map[string]interface{}{
				"name":      pc.Metadata.Name,
				"namespace": pc.Metadata.Namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"plugins": plugins,
			},
		},
	}
}

func (c *ClusterClient) applyBackendTrafficPolicy(ctx context.Context, btp apisix.BackendTrafficPolicy, dryRun bool) error {
	ns := btp.Metadata.Namespace
	if ns == "" {
		ns = "default"
	}

	obj := buildUnstructuredBackendTrafficPolicy(btp)

	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] Apply BackendTrafficPolicy %s/%s\n", ns, btp.Metadata.Name)
		return nil
	}

	client := c.dynamic.Resource(backendTrafficPolicyGVR).Namespace(ns)

	existing, err := client.Get(ctx, btp.Metadata.Name, metav1.GetOptions{})
	if err != nil {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("creating BackendTrafficPolicy %s/%s: %w", ns, btp.Metadata.Name, err)
		}
		fmt.Fprintf(os.Stderr, "[created] BackendTrafficPolicy %s/%s\n", ns, btp.Metadata.Name)
		return nil
	}

	obj.SetResourceVersion(existing.GetResourceVersion())
	_, err = client.Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating BackendTrafficPolicy %s/%s: %w", ns, btp.Metadata.Name, err)
	}
	fmt.Fprintf(os.Stderr, "[updated] BackendTrafficPolicy %s/%s\n", ns, btp.Metadata.Name)
	return nil
}

func buildUnstructuredBackendTrafficPolicy(btp apisix.BackendTrafficPolicy) *unstructured.Unstructured {
	targetRefs := make([]interface{}, 0, len(btp.Spec.TargetRefs))
	for _, ref := range btp.Spec.TargetRefs {
		targetRefs = append(targetRefs, map[string]interface{}{
			"group": ref.Group,
			"kind":  ref.Kind,
			"name":  ref.Name,
		})
	}

	var labels map[string]interface{}
	if btp.Metadata.Labels != nil {
		labels = make(map[string]interface{}, len(btp.Metadata.Labels))
		for k, v := range btp.Metadata.Labels {
			labels[k] = v
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": btp.APIVersion,
			"kind":       "BackendTrafficPolicy",
			"metadata": map[string]interface{}{
				"name":      btp.Metadata.Name,
				"namespace": btp.Metadata.Namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"targetRefs": targetRefs,
				"loadbalancer": map[string]interface{}{
					"type":   btp.Spec.LoadBalancer.Type,
					"hashOn": btp.Spec.LoadBalancer.HashOn,
					"key":    btp.Spec.LoadBalancer.Key,
				},
			},
		},
	}
}

// NamespaceFilter returns the configured namespace filter.
func (c *ClusterClient) NamespaceFilter() string {
	return c.ns
}
