module github.com/lavocatt/artemis-coredns-plugin

go 1.21

require (
	github.com/coredns/caddy v1.1.1
	github.com/coredns/coredns v1.11.1
	github.com/miekg/dns v1.1.57
	github.com/prometheus/client_golang v1.17.0
	k8s.io/api v0.28.4
	k8s.io/client-go v0.28.4
	sigs.k8s.io/controller-runtime v0.16.3
)
