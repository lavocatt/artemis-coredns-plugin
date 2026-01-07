package emptyendpoints

import (
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
)

var log = clog.NewWithPlugin("emptyendpoints")

func init() {
	plugin.Register("emptyendpoints", setup)
}

func setup(c *caddy.Controller) error {
	e := EmptyEndpoints{}

	// Parse config: emptyendpoints [namespace1 namespace2 ...]
	for c.Next() {
		e.Namespaces = c.RemainingArgs()
	}

	// Create Kubernetes client
	client, err := NewClient()
	if err != nil {
		log.Warningf("emptyendpoints: failed to create k8s client: %v", err)
	} else {
		e.client = client
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		e.Next = next
		return e
	})

	return nil
}
