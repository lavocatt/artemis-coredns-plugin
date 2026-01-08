// Package emptyendpoints is a CoreDNS plugin that tracks DNS queries
// to Kubernetes services with no ready endpoints, exposing a Prometheus
// metric that KEDA can use to trigger scale-from-zero.
//
// This plugin should be placed BEFORE the kubernetes plugin in the Corefile.
// It wraps the ResponseWriter to intercept responses from downstream plugins.
// When the kubernetes plugin returns NXDOMAIN (no endpoints), the wrapper
// increments the metric. No direct K8s API calls are made - we trust the
// kubernetes plugin's cached state.
package emptyendpoints

import (
	"context"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

var emptyEndpointQueries = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "coredns",
		Subsystem: "emptyendpoints",
		Name:      "queries_total",
		Help:      "DNS queries to services with no ready endpoints (NXDOMAIN from kubernetes plugin)",
	},
	[]string{"namespace", "service"},
)

func init() {
	prometheus.MustRegister(emptyEndpointQueries)
}

// EmptyEndpoints is the plugin struct.
type EmptyEndpoints struct {
	Next       plugin.Handler
	Namespaces []string // namespaces to watch (empty = all)
}

// Name returns the plugin name.
func (e EmptyEndpoints) Name() string { return "emptyendpoints" }

// ServeDNS implements the plugin.Handler interface.
// This plugin should be placed AFTER kubernetes in the plugin chain.
// It wraps the ResponseWriter to intercept NXDOMAIN responses from
// the kubernetes plugin for service queries.
func (e EmptyEndpoints) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()

	// Parse: <service>.<namespace>.svc.cluster.local.
	// or: <pod>.<service>.<namespace>.svc.cluster.local. (headless)
	parts := strings.Split(strings.TrimSuffix(qname, "."), ".")
	if len(parts) < 5 || parts[len(parts)-3] != "svc" {
		// Not a service query, pass through
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}

	namespace := parts[len(parts)-4]
	service := parts[len(parts)-5]

	// Check if we should watch this namespace
	if len(e.Namespaces) > 0 && !contains(e.Namespaces, namespace) {
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}

	// Wrap the ResponseWriter to intercept the response
	wrapper := &responseInterceptor{
		ResponseWriter: w,
		namespace:      namespace,
		service:        service,
	}

	// Call next plugin (should be kubernetes or forward)
	return plugin.NextOrFailure(e.Name(), e.Next, ctx, wrapper, r)
}

// responseInterceptor wraps dns.ResponseWriter to intercept NXDOMAIN responses
type responseInterceptor struct {
	dns.ResponseWriter
	namespace string
	service   string
}

// WriteMsg intercepts the DNS response. If it's NXDOMAIN for a service query,
// we increment the metric. This means the kubernetes plugin determined
// there are no endpoints for this service.
func (r *responseInterceptor) WriteMsg(res *dns.Msg) error {
	// Check if response is NXDOMAIN (name does not exist)
	// For headless services with no endpoints, kubernetes plugin returns NXDOMAIN
	if res.Rcode == dns.RcodeNameError {
		emptyEndpointQueries.WithLabelValues(r.namespace, r.service).Inc()
	}

	return r.ResponseWriter.WriteMsg(res)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
