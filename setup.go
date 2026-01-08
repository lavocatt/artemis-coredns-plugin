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

	if len(e.Namespaces) > 0 {
		log.Infof("emptyendpoints: watching namespaces: %v", e.Namespaces)
	} else {
		log.Info("emptyendpoints: watching all namespaces")
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		e.Next = next
		return e
	})

	return nil
}
