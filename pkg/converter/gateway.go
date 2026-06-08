package converter

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ingress2apisix/pkg/apisix"
	"github.com/ingress2apisix/pkg/ingress"
)

// GatewayConverter transforms nginx ingress-nginx Ingress resources into
// Kubernetes Gateway API resources (GatewayClass, Gateway, HTTPRoute).
type GatewayConverter struct {
	opts apisix.ConversionOptions
}

// hostInfo tracks per-host configuration for Gateway construction.
type hostInfo struct {
	hosts  []string
	tls    *ingress.IngressTLS
	ns     string
	gwName string
}

// NewGatewayConverter creates a GatewayConverter with the given options.
func NewGatewayConverter(opts apisix.ConversionOptions) *GatewayConverter {
	return &GatewayConverter{opts: opts}
}

// ConvertList converts a list of parsed Ingresses into Gateway API resources.
// It also generates ApisixPluginConfig and BackendTrafficPolicy CRDs for
// annotations that can be handled via APISIX plugins (e.g. rate-limiting, CORS).
func (gc *GatewayConverter) ConvertList(input ParsedInput) apisix.ConversionResult {
	result := apisix.ConversionResult{InputFormat: input.Format}

	// Create a single GatewayClass for APISIX
	gcName := "apisix"
	gatewayClass := apisix.GatewayClass{
		APIVersion: "gateway.networking.k8s.io/v1",
		Kind:       "GatewayClass",
		Metadata: apisix.Metadata{
			Name: gcName,
			Labels: map[string]string{
				"managed-by": "ingress2apisix",
			},
		},
		Spec: apisix.GatewayClassSpec{
			ControllerName: "apisix.apache.org/gateway-controller",
		},
	}
	result.GatewayClasses = append(result.GatewayClasses, gatewayClass)

	// Collect unique hosts and TLS configurations across all ingresses
	hostMap := make(map[string]*hostInfo) // key: sorted hosts joined

	for _, ing := range input.Ingresses {
		ns := ing.Metadata.Namespace
		if ns == "" {
			ns = gc.opts.DefaultNamespace
		}

		// Process TLS configurations
		for _, tls := range ing.Spec.TLS {
			key := strings.Join(tls.Hosts, ",")
			if _, exists := hostMap[key]; !exists {
				hostMap[key] = &hostInfo{
					hosts:  tls.Hosts,
					tls:    &tls,
					ns:     ns,
					gwName: gc.gatewayName(tls.Hosts),
				}
			}
		}

		// Process rules for hosts without TLS
		for _, rule := range ing.Spec.Rules {
			if rule.Host == "" {
				continue
			}
			key := rule.Host
			// Check if this host is already covered by a TLS entry
			covered := false
			for _, h := range hostMap {
				for _, th := range h.hosts {
					if th == rule.Host {
						covered = true
						break
					}
				}
				if covered {
					break
				}
			}
			if !covered {
				if _, exists := hostMap[key]; !exists {
					hostMap[key] = &hostInfo{
						hosts:  []string{rule.Host},
						ns:     ns,
						gwName: gc.gatewayName([]string{rule.Host}),
					}
				}
			}
		}
	}

	// Build Gateways from the collected host info
	gwSeen := make(map[string]bool)
	for _, hi := range hostMap {
		if gwSeen[hi.gwName] {
			continue
		}
		gwSeen[hi.gwName] = true

		gw := gc.buildGateway(hi.gwName, hi.ns, hi.hosts, hi.tls, gcName)
		result.Gateways = append(result.Gateways, gw)
	}

	// Sort gateways for deterministic output
	sort.Slice(result.Gateways, func(i, j int) bool {
		return result.Gateways[i].Metadata.Name < result.Gateways[j].Metadata.Name
	})

	// Create a standard Converter to reuse plugin-building logic
	stdConverter := New(gc.opts)

	// Build HTTPRoutes for each Ingress, along with plugin configs
	for _, ing := range input.Ingresses {
		ns := ing.Metadata.Namespace
		if ns == "" {
			ns = gc.opts.DefaultNamespace
		}

		// Build ApisixPluginConfig for this Ingress using the standard converter
		var pluginConfigName string
		if pc, warns := stdConverter.buildPluginConfig(ing, ns); pc != nil {
			result.PluginConfigs = append(result.PluginConfigs, *pc)
			result.Warnings = append(result.Warnings, warns...)
			pluginConfigName = pc.Metadata.Name
		}

		// Build additional Gateway-API-only plugins (auth, proxy timeouts, etc.)
		// These annotations are normally converted to native APISIX annotations,
		// but in Gateway API mode there's no Ingress to annotate, so we generate
		// APISIX plugins instead.
		if gwPlugins, gwWarns := gc.buildGatewayOnlyPlugins(ing, ns); len(gwPlugins) > 0 {
			if pluginConfigName != "" {
				// Append to existing PluginConfig
				for i := range result.PluginConfigs {
					if result.PluginConfigs[i].Metadata.Name == pluginConfigName {
						result.PluginConfigs[i].Spec.Plugins = append(
							result.PluginConfigs[i].Spec.Plugins, gwPlugins...)
						break
					}
				}
			} else {
				// Create a new PluginConfig
				name := truncateName(sanitizeK8sName(fmt.Sprintf("%s-gw-plugins", ing.Metadata.Name)), 64)
				pc := apisix.ApisixPluginConfig{
					APIVersion: gc.opts.ApisixVersion,
					Kind:       "ApisixPluginConfig",
					Metadata: apisix.Metadata{
						Name:      name,
						Namespace: ns,
						Labels: map[string]string{
							"managed-by":   "ingress2apisix",
							"ingress-name": ing.Metadata.Name,
						},
					},
					Spec: apisix.PluginConfigSpec{
						IngressClassName: gc.opts.TargetIngressClassName,
						Plugins:          gwPlugins,
					},
				}
				result.PluginConfigs = append(result.PluginConfigs, pc)
				pluginConfigName = name
			}
			result.Warnings = append(result.Warnings, gwWarns...)
		}

		// Build BackendTrafficPolicies for this Ingress
		if btps, warns := stdConverter.buildBackendTrafficPolicies(ing, ns); len(btps) > 0 {
			result.BackendTrafficPolicies = append(result.BackendTrafficPolicies, btps...)
			result.Warnings = append(result.Warnings, warns...)
		}

		// Build ApisixUpstream for health checks etc.
		if au, auWarns := stdConverter.buildApisixUpstream(ing, ns); au != nil || len(auWarns) > 0 {
			result.Warnings = append(result.Warnings, auWarns...)
			if au != nil {
				result.ApisixUpstreams = append(result.ApisixUpstreams, *au)
			}
		}

		// Build HTTPRoutes, passing the plugin config name for ExtensionRef injection
		routes, warns := gc.buildHTTPRoutes(ing, ns, hostMap, pluginConfigName)
		result.HTTPRoutes = append(result.HTTPRoutes, routes...)
		result.Warnings = append(result.Warnings, warns...)
	}

	// Sort HTTPRoutes for deterministic output
	sort.Slice(result.HTTPRoutes, func(i, j int) bool {
		if result.HTTPRoutes[i].Metadata.Namespace != result.HTTPRoutes[j].Metadata.Namespace {
			return result.HTTPRoutes[i].Metadata.Namespace < result.HTTPRoutes[j].Metadata.Namespace
		}
		return result.HTTPRoutes[i].Metadata.Name < result.HTTPRoutes[j].Metadata.Name
	})

	// Warn about unsupported annotations
	for _, ing := range input.Ingresses {
		gc.warnUnsupportedAnnotations(ing, &result)
	}

	return result
}

// gatewayName generates a Gateway name from a set of hosts.
func (gc *GatewayConverter) gatewayName(hosts []string) string {
	if len(hosts) == 0 {
		return "gateway"
	}
	// Use first host, sanitized
	name := strings.ReplaceAll(hosts[0], ".", "-")
	name = strings.ReplaceAll(name, "*", "wildcard")
	name = sanitizeK8sName(name)
	if name == "" {
		return "gateway"
	}
	return truncateName(name+"-gw", 63)
}

// buildGateway creates a Gateway resource.
func (gc *GatewayConverter) buildGateway(name, ns string, hosts []string, tls *ingress.IngressTLS, gcName string) apisix.Gateway {
	gw := apisix.Gateway{
		APIVersion: "gateway.networking.k8s.io/v1",
		Kind:       "Gateway",
		Metadata: apisix.Metadata{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"managed-by": "ingress2apisix",
			},
		},
		Spec: apisix.GatewaySpec{
			GatewayClassName: gcName,
		},
	}

	// Determine hostnames for listeners
	var hostnames []string
	for _, h := range hosts {
		if !strings.HasPrefix(h, "*.") {
			hostnames = append(hostnames, h)
		}
	}

	// HTTP listener
	httpListener := apisix.Listener{
		Name:     "http",
		Protocol: "HTTP",
		Port:     80,
	}
	if len(hostnames) == 1 {
		httpListener.Hostname = hostnames[0]
	}
	gw.Spec.Listeners = append(gw.Spec.Listeners, httpListener)

	// HTTPS listener if TLS is configured
	if tls != nil {
		httpsListener := apisix.Listener{
			Name:     "https",
			Protocol: "HTTPS",
			Port:     443,
			TLS: &apisix.ListenerTLS{
				Mode: "Terminate",
			},
		}
		if len(hostnames) == 1 {
			httpsListener.Hostname = hostnames[0]
		}
		// Add wildcard hostname if present
		for _, h := range hosts {
			if strings.HasPrefix(h, "*.") {
				httpsListener.Hostname = h
				break
			}
		}
		// Add certificate reference
		nsRef := ns
		if tls.SecretName != "" {
			httpsListener.TLS.CertRefs = []apisix.CertRef{
				{
					Kind:      "Secret",
					Name:      tls.SecretName,
					Namespace: nsRef,
				},
			}
		}
		gw.Spec.Listeners = append(gw.Spec.Listeners, httpsListener)
	}

	return gw
}

// buildHTTPRoutes creates HTTPRoute resources from an Ingress.
// pluginConfigName, if non-empty, is added as an ExtensionRef filter to each rule.
func (gc *GatewayConverter) buildHTTPRoutes(ing ingress.Ingress, ns string, hostMap map[string]*hostInfo, pluginConfigName string) ([]apisix.HTTPRoute, []string) {
	var routes []apisix.HTTPRoute
	var warnings []string

	anns := ing.Metadata.Annotations

	// Find the Gateway name for this Ingress's hosts
	findGateway := func(host string) string {
		for _, hi := range hostMap {
			for _, h := range hi.hosts {
				if h == host {
					return hi.gwName
				}
			}
		}
		// Fallback: generate a gateway name
		return gc.gatewayName([]string{host})
	}

	for ri, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}

		gwName := findGateway(rule.Host)
		routeName := fmt.Sprintf("%s-rule%d", ing.Metadata.Name, ri)
		routeName = truncateName(sanitizeK8sName(routeName), 63)

		route := apisix.HTTPRoute{
			APIVersion: "gateway.networking.k8s.io/v1",
			Kind:       "HTTPRoute",
			Metadata: apisix.Metadata{
				Name:      routeName,
				Namespace: ns,
				Labels: map[string]string{
					"managed-by":   "ingress2apisix",
					"ingress-name": ing.Metadata.Name,
				},
			},
			Spec: apisix.HTTPRouteSpec{
				ParentRefs: []apisix.ParentRef{
					{
						Name:      gwName,
						Namespace: ns,
					},
				},
			},
		}

		// Set hostname
		if rule.Host != "" {
			route.Spec.Hostnames = []string{rule.Host}
		}

		// Build rules for each path
		for _, path := range rule.HTTP.Paths {
			hrRule := apisix.HTTPRouteRule{}

			// Match
			match := apisix.HTTPRouteMatch{}
			match.Path = gc.convertPathMatch(path.Path, path.PathType)
			hrRule.Matches = append(hrRule.Matches, match)

			// Backend refs
			if path.Backend.Service != nil {
				port := int(path.Backend.Service.Port.Number)
				if port == 0 && path.Backend.Service.Port.Name != "" {
					// Can't resolve named ports; warn
					warnings = append(warnings,
						fmt.Sprintf("[%s/%s] path %s uses named port %q which cannot be resolved to a port number in Gateway API",
							ns, ing.Metadata.Name, path.Path, path.Backend.Service.Port.Name))
					port = 80 // fallback
				}
				hrRule.BackendRefs = []apisix.BackendRef{
					{
						Name: path.Backend.Service.Name,
						Port: port,
					},
				}
			}

			// Process annotations into filters
			filters, warns := gc.buildFilters(anns, ing, ns)
			hrRule.Filters = filters
			warnings = append(warnings, warns...)

			// Add ExtensionRef filter if a plugin config was generated
			if pluginConfigName != "" {
				hrRule.Filters = append(hrRule.Filters, apisix.HTTPRouteFilter{
					Type: "ExtensionRef",
					ExtensionRef: &apisix.ExtensionRef{
						Group: "apisix.apache.org",
						Kind:  "PluginConfig",
						Name:  pluginConfigName,
					},
				})
			}

			route.Spec.Rules = append(route.Spec.Rules, hrRule)
		}

		routes = append(routes, route)
	}

	return routes, warnings
}

// convertPathMatch converts an Ingress path/pathType to a Gateway API PathMatch.
func (gc *GatewayConverter) convertPathMatch(path string, pathType *string) *apisix.PathMatch {
	if path == "" {
		return &apisix.PathMatch{
			Type:  "PathPrefix",
			Value: "/",
		}
	}

	pt := "Prefix"
	if pathType != nil {
		pt = *pathType
	}

	// Check if path is a regex
	if pathHasRegex(path) {
		return &apisix.PathMatch{
			Type:  "RegularExpression",
			Value: path,
		}
	}

	switch pt {
	case "Exact":
		return &apisix.PathMatch{
			Type:  "Exact",
			Value: path,
		}
	case "Prefix":
		return &apisix.PathMatch{
			Type:  "PathPrefix",
			Value: path,
		}
	case "ImplementationSpecific":
		// ImplementationSpecific without regex → treat as PathPrefix
		return &apisix.PathMatch{
			Type:  "PathPrefix",
			Value: path,
		}
	default:
		return &apisix.PathMatch{
			Type:  "PathPrefix",
			Value: path,
		}
	}
}

// buildFilters creates Gateway API filters from nginx annotations.
func (gc *GatewayConverter) buildFilters(anns map[string]string, ing ingress.Ingress, ns string) ([]apisix.HTTPRouteFilter, []string) {
	if anns == nil {
		return nil, nil
	}

	var filters []apisix.HTTPRouteFilter
	var warnings []string

	// --- rewrite-target → URLRewrite filter ---
	if target, ok := getAnnotation(anns, "rewrite-target"); ok {
		if regexCapturePattern.MatchString(target) {
			// Regex rewrite - Gateway API doesn't support regex rewrites natively
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] rewrite-target=%q uses regex captures ($1, $2) which are not supported by Gateway API URLRewrite filter; manual intervention required",
					ns, ing.Metadata.Name, target))
		} else {
			filters = append(filters, apisix.HTTPRouteFilter{
				Type: "URLRewrite",
				URLRewrite: &apisix.URLRewrite{
					Path: &apisix.PathModifier{
						Type:  "ReplaceFullPath",
						Value: target,
					},
				},
			})
		}
	}

	// --- upstream-vhost → URLRewrite hostname filter ---
	if vhost, ok := getAnnotation(anns, "upstream-vhost"); ok && vhost != "" {
		filters = append(filters, apisix.HTTPRouteFilter{
			Type: "URLRewrite",
			URLRewrite: &apisix.URLRewrite{
				Hostname: vhost,
			},
		})
	}

	// --- ssl-redirect / force-ssl-redirect → RequestRedirect filter ---
	if v, ok := getAnnotation(anns, "ssl-redirect"); ok && v == "true" {
		filters = append(filters, apisix.HTTPRouteFilter{
			Type: "RequestRedirect",
			RequestRedirect: &apisix.HTTPRequestRedirect{
				Scheme:     "https",
				StatusCode: 301,
			},
		})
	}
	if hasAnnotation(anns, "force-ssl-redirect") {
		filters = append(filters, apisix.HTTPRouteFilter{
			Type: "RequestRedirect",
			RequestRedirect: &apisix.HTTPRequestRedirect{
				Scheme:     "https",
				StatusCode: 301,
			},
		})
	}

	// --- permanent-redirect → RequestRedirect filter ---
	if v, ok := getAnnotation(anns, "permanent-redirect"); ok && v != "" {
		filters = append(filters, apisix.HTTPRouteFilter{
			Type: "RequestRedirect",
			RequestRedirect: &apisix.HTTPRequestRedirect{
				Hostname:   extractHostname(v),
				StatusCode: 308,
			},
		})
	}

	// --- temporal-redirect → RequestRedirect filter ---
	if v, ok := getAnnotation(anns, "temporal-redirect"); ok && v != "" {
		filters = append(filters, apisix.HTTPRouteFilter{
			Type: "RequestRedirect",
			RequestRedirect: &apisix.HTTPRequestRedirect{
				Hostname:   extractHostname(v),
				StatusCode: 302,
			},
		})
	}

	// --- configuration-snippet with rewrite → URLRewrite filter ---
	if snippet, ok := getAnnotation(anns, "configuration-snippet"); ok {
		rewriteURIs := extractRewriteURIs(snippet)
		if len(rewriteURIs) == 2 {
			// Single rewrite: pattern → replacement
			// Gateway API doesn't support regex rewrites
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] configuration-snippet contains rewrite directive with regex pattern %q which is not supported by Gateway API URLRewrite filter; manual intervention required",
					ns, ing.Metadata.Name, rewriteURIs[0]))
		} else if len(rewriteURIs) > 2 {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] configuration-snippet contains multiple rewrite directives which are not supported by Gateway API; manual intervention required",
					ns, ing.Metadata.Name))
		}
	}

	return filters, warnings
}

// extractHostname extracts the hostname from a URL string.
func extractHostname(url string) string {
	// Remove scheme
	h := url
	if idx := strings.Index(h, "://"); idx >= 0 {
		h = h[idx+3:]
	}
	// Remove path
	if idx := strings.Index(h, "/"); idx >= 0 {
		h = h[:idx]
	}
	// Remove port
	if idx := strings.Index(h, ":"); idx >= 0 {
		h = h[:idx]
	}
	return h
}

// buildGatewayOnlyPlugins creates additional APISIX plugins for annotations
// that are normally converted to native APISIX annotations on the Ingress, but
// in Gateway API mode there's no Ingress to annotate. Instead, we generate
// APISIX plugins that will be applied via ExtensionRef.
//
// Handles: auth-* → forward-auth, proxy-*-timeout → proxy-rewrite,
// denylist/whitelist → ip-restriction, backend-protocol, etc.
func (gc *GatewayConverter) buildGatewayOnlyPlugins(ing ingress.Ingress, ns string) ([]apisix.Plugin, []string) {
	anns := ing.Metadata.Annotations
	if anns == nil {
		return nil, nil
	}

	var plugins []apisix.Plugin
	var warnings []string

	// --- Auth annotations → forward-auth plugin ---
	if v, ok := getAnnotation(anns, "auth-url"); ok && v != "" {
		faConfig := map[string]interface{}{
			"uri": v,
		}
		if method, ok := getAnnotation(anns, "auth-method"); ok && method != "" {
			faConfig["request_method"] = strings.ToUpper(method)
		}
		if headers, ok := getAnnotation(anns, "auth-request-headers"); ok && headers != "" {
			faConfig["request_headers"] = strings.Split(headers, ",")
		}
		if respHeaders, ok := getAnnotation(anns, "auth-response-headers"); ok && respHeaders != "" {
			faConfig["upstream_headers"] = strings.Split(respHeaders, ",")
		}
		if signin, ok := getAnnotation(anns, "auth-signin"); ok && signin != "" {
			faConfig["client_redirect"] = signin
		}
		if realm, ok := getAnnotation(anns, "auth-realm"); ok && realm != "" {
			faConfig["request_headers"] = appendToSlice(faConfig["request_headers"], "Authorization")
		}
		plugins = append(plugins, apisix.Plugin{
			Name:   "forward-auth",
			Enable: true,
			Config: faConfig,
		})
	}

	// --- Proxy timeouts → proxy-rewrite plugin timeout config ---
	hasTimeout := false
	proxyRewriteConfig := make(map[string]interface{})
	if v, ok := getAnnotation(anns, "proxy-connect-timeout"); ok && v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			proxyRewriteConfig["connect_timeout"] = n * 1000 // nginx uses seconds, APISIX uses ms
			hasTimeout = true
		}
	}
	if v, ok := getAnnotation(anns, "proxy-send-timeout"); ok && v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			proxyRewriteConfig["send_timeout"] = n * 1000
			hasTimeout = true
		}
	}
	if v, ok := getAnnotation(anns, "proxy-read-timeout"); ok && v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			proxyRewriteConfig["read_timeout"] = n * 1000
			hasTimeout = true
		}
	}
	if hasTimeout {
		plugins = append(plugins, apisix.Plugin{
			Name:   "proxy-rewrite",
			Enable: true,
			Config: proxyRewriteConfig,
		})
	}

	// --- denylist/whitelist → ip-restriction plugin ---
	if v, ok := getAnnotation(anns, "whitelist-source-range"); ok && v != "" {
		allowList := strings.Split(v, ",")
		for i := range allowList {
			allowList[i] = strings.TrimSpace(allowList[i])
		}
		plugins = append(plugins, apisix.Plugin{
			Name:   "ip-restriction",
			Enable: true,
			Config: map[string]interface{}{
				"whitelist": allowList,
			},
		})
	}
	if v, ok := getAnnotation(anns, "denylist-source-range"); ok && v != "" {
		denyList := strings.Split(v, ",")
		for i := range denyList {
			denyList[i] = strings.TrimSpace(denyList[i])
		}
		plugins = append(plugins, apisix.Plugin{
			Name:   "ip-restriction",
			Enable: true,
			Config: map[string]interface{}{
				"blacklist": denyList,
			},
		})
	}

	// --- backend-protocol → grpc-transcode or general protocol config ---
	if v, ok := getAnnotation(anns, "backend-protocol"); ok {
		proto := strings.ToLower(strings.TrimSpace(v))
		if proto == "grpc" || proto == "grpcs" {
			warnings = append(warnings,
				fmt.Sprintf("[%s/%s] backend-protocol=%s: Gateway API uses AppProtocol on the Service; the generated APISIX plugin config sets upstream scheme",
					ns, ing.Metadata.Name, v))
		}
	}

	// --- custom-http-errors → custom-error-page plugin ---
	if v, ok := getAnnotation(anns, "custom-http-errors"); ok && v != "" {
		codes := strings.Split(v, ",")
		var errCodes []int
		for _, c := range codes {
			c = strings.TrimSpace(c)
			if n, err := strconv.Atoi(c); err == nil {
				errCodes = append(errCodes, n)
			}
		}
		if len(errCodes) > 0 {
			plugins = append(plugins, apisix.Plugin{
				Name:   "custom-error-page",
				Enable: true,
				Config: map[string]interface{}{
					"error_codes": errCodes,
				},
			})
		}
	}

	// --- proxy-request-buffering → proxy-control plugin ---
	if v, ok := getAnnotation(anns, "proxy-request-buffering"); ok {
		if v == "off" || v == "false" {
			plugins = append(plugins, apisix.Plugin{
				Name:   "proxy-control",
				Enable: true,
				Config: map[string]interface{}{
					"request_buffering": false,
				},
			})
		}
	}

	return plugins, warnings
}

// appendToSlice safely appends a string to a slice stored as interface{}.
func appendToSlice(v interface{}, s string) []string {
	if v == nil {
		return []string{s}
	}
	if sl, ok := v.([]string); ok {
		return append(sl, s)
	}
	return []string{s}
}

// warnUnsupportedAnnotations adds warnings for annotations that have no Gateway API equivalent.
// Annotations handled via PluginConfig (ExtensionRef) or BackendTrafficPolicy are excluded.
func (gc *GatewayConverter) warnUnsupportedAnnotations(ing ingress.Ingress, result *apisix.ConversionResult) {
	if ing.Metadata.Annotations == nil {
		return
	}

	ns := ing.Metadata.Namespace
	if ns == "" {
		ns = gc.opts.DefaultNamespace
	}

	// Annotations that have no native Gateway API equivalent and are NOT handled
	// via ExtensionRef/PluginConfig/BackendTrafficPolicy.
	unsupported := map[string]string{
		"whitelist-source-range": "Gateway API has no native IP allowlist; use a gateway controller extension or policy CRD",
		"denylist-source-range":  "Gateway API has no native IP denylist; use a gateway controller extension or policy CRD",
		"enable-access-log":      "Access log configuration is implementation-specific in Gateway API",
		"backend-protocol":       "Gateway API does not have a direct backend protocol setting; use AppProtocol on the Service",
		"enable-websocket":       "WebSocket support is implementation-specific in Gateway API",
		"use-regex":              "Gateway API uses RegularExpression path type natively; no separate annotation needed",
	}

	seen := make(map[string]bool)
	for k := range ing.Metadata.Annotations {
		if !isNginxAnnotation(k) {
			continue
		}
		suffix := annotationSuffix(k)
		if suffix == "" || seen[suffix] {
			continue
		}
		seen[suffix] = true

		// Skip annotations already handled in buildFilters (native Gateway API filters)
		handledInFilters := map[string]bool{
			"rewrite-target":        true,
			"ssl-redirect":          true,
			"force-ssl-redirect":    true,
			"permanent-redirect":    true,
			"temporal-redirect":     true,
			"configuration-snippet": true,
			"upstream-vhost":        true,
		}
		if handledInFilters[suffix] {
			continue
		}

		// Skip annotations handled via PluginConfig (ExtensionRef) or BackendTrafficPolicy
		handledViaPlugins := map[string]bool{
			// Rate limiting → limit-req plugin in PluginConfig
			"limit-rps":         true,
			"limit-rpm":         true,
			"limit-connections": true,
			"limit-multiplier":  true,
			// proxy-body-size → client-control plugin in PluginConfig
			"proxy-body-size": true,
			// CORS → cors plugin in PluginConfig
			"enable-cors":            true,
			"cors-allow-origin":      true,
			"cors-allow-methods":     true,
			"cors-allow-headers":     true,
			"cors-allow-credentials": true,
			"cors-max-age":           true,
			// proxy-cookie-path → proxy-cookie-path plugin in PluginConfig
			"proxy-cookie-path": true,
			// Session cookie → session-cookie-hash plugin in PluginConfig
			"session-cookie-hash":    true,
			"session-cookie-expires": true,
			"session-cookie-max-age": true,
			"session-cookie-path":    true,
			// Real IP → real-ip plugin in PluginConfig
			"enable-real-ip":             true,
			"use-forwarded-headers":      true,
			"compute-full-forwarded-for": true,
			"forwarded-for-header":       true,
			// SSL verify → handled via PluginConfig warnings
			"ssl-verify": true,
			// upstream-hash-by → BackendTrafficPolicy with chash
			"upstream-hash-by": true,
			// Auth annotations → forward-auth plugin in PluginConfig (Gateway API only)
			"auth-url":              true,
			"auth-method":           true,
			"auth-type":             true,
			"auth-secret":           true,
			"auth-signin":           true,
			"auth-request-headers":  true,
			"auth-response-headers": true,
			// Proxy timeouts → proxy-rewrite plugin timeout in PluginConfig (Gateway API only)
			"proxy-connect-timeout": true,
			"proxy-send-timeout":    true,
			"proxy-read-timeout":    true,
			// Health checks → BackendTrafficPolicy
			"health-check-path":     true,
			"health-check-interval": true,
			"health-check-timeout":  true,
			"health-check-retries":  true,
			// Session affinity → BackendTrafficPolicy
			"affinity":            true,
			"session-cookie-name": true,
			// affinity-mode → consumed by BackendTrafficPolicy
			"affinity-mode": true,
			// proxy-request-buffering → proxy-control plugin
			"proxy-request-buffering": true,
			// auth-realm → forward-auth plugin realm config
			"auth-realm": true,
		}
		if handledViaPlugins[suffix] {
			continue
		}

		if reason, ok := unsupported[suffix]; ok {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("[%s/%s] annotation %s is not natively supported in Gateway API: %s",
					ns, ing.Metadata.Name, k, reason))
		}
	}
}

// WriteGatewayAPIResult writes Gateway API resources as multi-document YAML.
func WriteGatewayAPIResult(w io.Writer, result apisix.ConversionResult) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()

	// Write GatewayClass
	for i := range result.GatewayClasses {
		if err := enc.Encode(&result.GatewayClasses[i]); err != nil {
			return fmt.Errorf("encoding GatewayClass: %w", err)
		}
	}

	// Write Gateways
	for i := range result.Gateways {
		if err := enc.Encode(&result.Gateways[i]); err != nil {
			return fmt.Errorf("encoding Gateway %s: %w", result.Gateways[i].Metadata.Name, err)
		}
	}

	// Write HTTPRoutes
	for i := range result.HTTPRoutes {
		if err := enc.Encode(&result.HTTPRoutes[i]); err != nil {
			return fmt.Errorf("encoding HTTPRoute %s: %w", result.HTTPRoutes[i].Metadata.Name, err)
		}
	}

	// Write ApisixPluginConfig resources
	for i := range result.PluginConfigs {
		if err := enc.Encode(&result.PluginConfigs[i]); err != nil {
			return fmt.Errorf("encoding ApisixPluginConfig %s: %w", result.PluginConfigs[i].Metadata.Name, err)
		}
	}

	// Write BackendTrafficPolicy resources
	for i := range result.BackendTrafficPolicies {
		if err := enc.Encode(&result.BackendTrafficPolicies[i]); err != nil {
			return fmt.Errorf("encoding BackendTrafficPolicy %s: %w", result.BackendTrafficPolicies[i].Metadata.Name, err)
		}
	}

	// Write ApisixUpstream resources
	for i := range result.ApisixUpstreams {
		if err := enc.Encode(&result.ApisixUpstreams[i]); err != nil {
			return fmt.Errorf("encoding ApisixUpstream %s: %w", result.ApisixUpstreams[i].Metadata.Name, err)
		}
	}

	// Write ApisixTls resources
	for i := range result.ApisixTls {
		if err := enc.Encode(&result.ApisixTls[i]); err != nil {
			return fmt.Errorf("encoding ApisixTls %s: %w", result.ApisixTls[i].Metadata.Name, err)
		}
	}

	return nil
}

// FormatGatewayAPIResultSummary returns a human-readable summary of the Gateway API conversion.
func FormatGatewayAPIResultSummary(result apisix.ConversionResult) string {
	var sb strings.Builder
	sb.WriteString("Gateway API Conversion Summary:\n")

	formatStr := "single-document"
	switch result.InputFormat {
	case apisix.FormatList:
		formatStr = "IngressList"
	case apisix.FormatMultiDoc:
		formatStr = "multi-document"
	}
	sb.WriteString(fmt.Sprintf("  Input format:         %s\n", formatStr))
	sb.WriteString(fmt.Sprintf("  GatewayClass:         %d\n", len(result.GatewayClasses)))
	sb.WriteString(fmt.Sprintf("  Gateway:              %d\n", len(result.Gateways)))
	sb.WriteString(fmt.Sprintf("  HTTPRoute:            %d\n", len(result.HTTPRoutes)))
	sb.WriteString(fmt.Sprintf("  ApisixPluginConfig:   %d\n", len(result.PluginConfigs)))
	sb.WriteString(fmt.Sprintf("  BackendTrafficPolicy: %d\n", len(result.BackendTrafficPolicies)))
	sb.WriteString(fmt.Sprintf("  ApisixUpstream:       %d\n", len(result.ApisixUpstreams)))
	sb.WriteString(fmt.Sprintf("  ApisixTls:            %d\n", len(result.ApisixTls)))

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
