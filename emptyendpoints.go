// Package emptyendpoints is a CoreDNS plugin that tracks DNS queries
// to Kubernetes services with no ready endpoints, exposing a Prometheus
// metric that KEDA can use to trigger scale-from-zero.
package emptyendpoints

import (
	"context"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var emptyEndpointQueries = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "coredns",
		Subsystem: "emptyendpoints",
		Name:      "queries_total",
		Help:      "DNS queries to services with no ready endpoints",
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
	client     client.Client
}

// Name returns the plugin name.
func (e EmptyEndpoints) Name() string { return "emptyendpoints" }

// ServeDNS implements the plugin.Handler interface.
func (e EmptyEndpoints) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()

	// Parse: <service>.<namespace>.svc.cluster.local.
	// or: <pod>.<service>.<namespace>.svc.cluster.local.
	parts := strings.Split(strings.TrimSuffix(qname, "."), ".")
	if len(parts) < 5 || parts[len(parts)-3] != "svc" {
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}

	namespace := parts[len(parts)-4]
	service := parts[len(parts)-5]

	// Check if we should watch this namespace
	if len(e.Namespaces) > 0 && !contains(e.Namespaces, namespace) {
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}

	// Check if service has ready endpoints
	if !e.hasReadyEndpoints(ctx, namespace, service) {
		emptyEndpointQueries.WithLabelValues(namespace, service).Inc()
		// Let the query continue - it will fail naturally, but we've recorded the metric
	}

	return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
}

func (e EmptyEndpoints) hasReadyEndpoints(ctx context.Context, namespace, service string) bool {
	if e.client == nil {
		return true // fail open if no client
	}

	endpoints := &corev1.Endpoints{}
	err := e.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: service}, endpoints)
	if err != nil {
		return true // fail open on error
	}

	for _, subset := range endpoints.Subsets {
		if len(subset.Addresses) > 0 {
			return true
		}
	}
	return false
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// NewClient creates a Kubernetes client for the plugin.
func NewClient() (client.Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	_ = clientset // we use controller-runtime client instead
	return client.New(config, client.Options{})
}
