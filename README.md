# CoreDNS Empty Endpoints Plugin

A simple CoreDNS plugin that exposes a Prometheus metric when DNS queries are
made to Kubernetes services with no ready endpoints. KEDA can watch this metric
to trigger scale-from-zero.

## How It Works

```
Client DNS Query → CoreDNS → emptyendpoints plugin
                              ↓
                    Check if service has endpoints
                              ↓
                    No endpoints? → Increment metric
                              ↓
                    Continue to next plugin (kubernetes)
```

The plugin exposes:

```
coredns_emptyendpoints_queries_total{namespace="default",service="broker"} 5
```

## Building

CoreDNS plugins must be compiled into the CoreDNS binary. Add this plugin to
your CoreDNS build:

1. Clone CoreDNS:
   ```bash
   git clone https://github.com/coredns/coredns
   cd coredns
   ```

2. Add the plugin to `plugin.cfg` (before `kubernetes`):
   ```
   emptyendpoints:github.com/lavocatt/artemis-coredns-plugin
   ```

3. Build:
   ```bash
   go generate
   go build
   ```

## Configuration

In your Corefile:

```
.:53 {
    emptyendpoints serverless-broker  # watch only this namespace
    kubernetes cluster.local
    forward . /etc/resolv.conf
    prometheus :9153
}
```

Or watch all namespaces:

```
.:53 {
    emptyendpoints
    kubernetes cluster.local
    forward . /etc/resolv.conf
    prometheus :9153
}
```

## KEDA ScaledObject

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: broker-scaler
spec:
  scaleTargetRef:
    apiVersion: broker.amq.io/v1beta1
    kind: ActiveMQArtemis
    name: my-broker
  minReplicaCount: 0
  maxReplicaCount: 1
  triggers:
    - type: prometheus
      metadata:
        serverAddress: http://kube-dns.kube-system:9153
        query: coredns_emptyendpoints_queries_total{service="my-broker-amqp-0-svc"}
        threshold: "1"
```

## Limitations

- Requires rebuilding CoreDNS with the plugin
- Only detects DNS queries, not direct IP connections
- Metric is cumulative (consider using `rate()` in KEDA query)
