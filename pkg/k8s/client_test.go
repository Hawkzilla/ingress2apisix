package k8s

import (
	"context"
	"testing"

	networkingV1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/ingress2apisix/pkg/apisix"
	"github.com/ingress2apisix/pkg/ingress"
)

func strPtr(s string) *string {
	return &s
}

func pathTypePtr(pt networkingV1.PathType) *networkingV1.PathType {
	return &pt
}

func makeK8sIngress(name, ns string, annotations map[string]string) *networkingV1.Ingress {
	return &networkingV1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
		},
		Spec: networkingV1.IngressSpec{
			IngressClassName: strPtr("nginx"),
			Rules: []networkingV1.IngressRule{
				{
					Host: "test.com",
					IngressRuleValue: networkingV1.IngressRuleValue{
						HTTP: &networkingV1.HTTPIngressRuleValue{
							Paths: []networkingV1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: pathTypePtr(networkingV1.PathTypePrefix),
									Backend: networkingV1.IngressBackend{
										Service: &networkingV1.IngressServiceBackend{
											Name: "test-svc",
											Port: networkingV1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestConvertK8sIngress(t *testing.T) {
	k8sIng := makeK8sIngress("test-ing", "default",
		map[string]string{
			"nginx.ingress.kubernetes.io/enable-cors": "true",
		})

	ing := convertK8sIngress(k8sIng)

	if ing.Metadata.Name != "test-ing" {
		t.Errorf("expected name 'test-ing', got '%s'", ing.Metadata.Name)
	}
	if ing.Metadata.Namespace != "default" {
		t.Errorf("expected namespace 'default', got '%s'", ing.Metadata.Namespace)
	}
	if *ing.Spec.IngressClassName != "nginx" {
		t.Errorf("expected ingressClassName 'nginx', got '%s'", *ing.Spec.IngressClassName)
	}
	if len(ing.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(ing.Spec.Rules))
	}
	if ing.Spec.Rules[0].Host != "test.com" {
		t.Errorf("expected host 'test.com', got '%s'", ing.Spec.Rules[0].Host)
	}
	if ing.Metadata.Annotations["nginx.ingress.kubernetes.io/enable-cors"] != "true" {
		t.Error("expected cors annotation to be preserved")
	}
}

func TestReadIngressFromCluster(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		makeK8sIngress("ing-1", "ns1", nil),
		makeK8sIngress("ing-2", "ns2", nil),
	)

	c := &ClusterClient{k8s: fakeClient, ns: ""}

	input, err := c.ReadIngressFromCluster(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(input.Ingresses) != 2 {
		t.Fatalf("expected 2 ingresses, got %d", len(input.Ingresses))
	}
	if input.Format != apisix.FormatMultiDoc {
		t.Errorf("expected FormatMultiDoc, got %d", input.Format)
	}
}

func TestReadIngressFromCluster_Namespaced(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		makeK8sIngress("ing-1", "ns1", nil),
		makeK8sIngress("ing-2", "ns2", nil),
		makeK8sIngress("ing-3", "ns1", nil),
	)

	c := &ClusterClient{k8s: fakeClient, ns: "ns1"}

	input, err := c.ReadIngressFromCluster(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(input.Ingresses) != 2 {
		t.Fatalf("expected 2 ingresses in ns1, got %d", len(input.Ingresses))
	}
}

func TestReadIngressFromCluster_Empty(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()

	c := &ClusterClient{k8s: fakeClient, ns: ""}

	_, err := c.ReadIngressFromCluster(context.Background())
	if err == nil {
		t.Fatal("expected error for empty cluster")
	}
}

func TestBuildIngressPatch(t *testing.T) {
	ing := ingress.Ingress{
		Metadata: ingress.Metadata{
			Name:      "test",
			Namespace: "default",
			Annotations: map[string]string{
				"k8s.apisix.apache.org/enable-cors":   "true",
				"k8s.apisix.apache.org/http-to-https": "true",
			},
		},
		Spec: ingress.IngressSpec{
			IngressClassName: strPtr("apisix"),
		},
	}

	patch := buildIngressPatch(ing)
	patchStr := string(patch)

	if patchStr == "" {
		t.Fatal("expected non-empty patch")
	}
	// Should contain annotation and ingressClassName
	if !containsStr(patchStr, "apisix") {
		t.Error("patch should contain 'apisix'")
	}
	if !containsStr(patchStr, "enable-cors") {
		t.Error("patch should contain 'enable-cors'")
	}
}

func TestBuildUnstructuredPluginConfig(t *testing.T) {
	pc := apisix.ApisixPluginConfig{
		APIVersion: "apisix.apache.org/v2",
		Kind:       "ApisixPluginConfig",
		Metadata: apisix.Metadata{
			Name:      "test-plugins",
			Namespace: "default",
			Labels: map[string]string{
				"managed-by": "ingress2apisix",
			},
		},
		Spec: apisix.PluginConfigSpec{
			Plugins: []apisix.Plugin{
				{
					Name:   "limit-req",
					Enable: true,
					Config: map[string]interface{}{
						"rate":          "100",
						"rejected_code": "429",
					},
				},
			},
		},
	}

	obj := buildUnstructuredPluginConfig(pc)

	if obj.GetName() != "test-plugins" {
		t.Errorf("expected name 'test-plugins', got '%s'", obj.GetName())
	}
	if obj.GetNamespace() != "default" {
		t.Errorf("expected namespace 'default', got '%s'", obj.GetNamespace())
	}
	if obj.GetAPIVersion() != "apisix.apache.org/v2" {
		t.Errorf("expected apiVersion 'apisix.apache.org/v2', got '%s'", obj.GetAPIVersion())
	}
	if obj.GetKind() != "ApisixPluginConfig" {
		t.Errorf("expected kind 'ApisixPluginConfig', got '%s'", obj.GetKind())
	}

	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		t.Fatal("spec not found")
	}
	plugins, found, err := unstructured.NestedSlice(spec, "plugins")
	if err != nil || !found {
		t.Fatal("plugins not found in spec")
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
}

func TestApplyPluginConfig_Create(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicFake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			apisixPluginConfigGVR: "ApisixPluginConfigList",
		},
	)

	c := &ClusterClient{dynamic: dynClient}

	pc := apisix.ApisixPluginConfig{
		APIVersion: "apisix.apache.org/v2",
		Kind:       "ApisixPluginConfig",
		Metadata: apisix.Metadata{
			Name:      "new-plugins",
			Namespace: "default",
		},
		Spec: apisix.PluginConfigSpec{
			Plugins: []apisix.Plugin{
				{Name: "limit-req", Enable: true},
			},
		},
	}

	err := c.applyPluginConfig(context.Background(), pc, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyResult_DryRun(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		makeK8sIngress("test", "default", nil),
	)
	scheme := runtime.NewScheme()
	dynClient := dynamicFake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			apisixPluginConfigGVR: "ApisixPluginConfigList",
		},
	)

	c := &ClusterClient{k8s: fakeClient, dynamic: dynClient}

	result := apisix.ConversionResult{
		Ingresses: []interface{}{
			ingress.Ingress{
				APIVersion: "networking.k8s.io/v1",
				Kind:       "Ingress",
				Metadata: ingress.Metadata{
					Name:      "test",
					Namespace: "default",
					Annotations: map[string]string{
						"k8s.apisix.apache.org/enable-cors": "true",
					},
				},
				Spec: ingress.IngressSpec{
					IngressClassName: strPtr("apisix"),
				},
			},
		},
		PluginConfigs: []apisix.ApisixPluginConfig{
			{
				APIVersion: "apisix.apache.org/v2",
				Kind:       "ApisixPluginConfig",
				Metadata:   apisix.Metadata{Name: "test-plugins", Namespace: "default"},
				Spec:       apisix.PluginConfigSpec{Plugins: []apisix.Plugin{{Name: "limit-req", Enable: true}}},
			},
		},
	}

	err := c.ApplyResult(context.Background(), result, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNamespaceFilter(t *testing.T) {
	c := &ClusterClient{ns: "production"}
	if c.NamespaceFilter() != "production" {
		t.Errorf("expected 'production', got '%s'", c.NamespaceFilter())
	}

	c2 := &ClusterClient{ns: ""}
	if c2.NamespaceFilter() != "" {
		t.Errorf("expected empty, got '%s'", c2.NamespaceFilter())
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
