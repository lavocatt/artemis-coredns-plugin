# CoreDNS Empty Endpoints Plugin

A lightweight CoreDNS plugin that exposes a Prometheus metric when DNS queries
receive NXDOMAIN responses for Kubernetes service names. KEDA can watch this
metric to trigger scale-from-zero.

## How It Works

This plugin is positioned **BEFORE** the `kubernetes` plugin in the CoreDNS chain.
It wraps the ResponseWriter to intercept responses from downstream plugins. When
the kubernetes plugin returns NXDOMAIN (no ready endpoints), the wrapper detects
this and increments the metric.

```
Client DNS Query → CoreDNS
                     ↓
              emptyendpoints plugin
              (wraps ResponseWriter)
                     ↓
              kubernetes plugin
              (checks cached endpoints)
                     ↓
              NXDOMAIN response
                     ↓
              ResponseWriter intercepts
              and increments metric
                     ↓
              Return NXDOMAIN to client
```

**Key benefits:**
- **No additional K8s API calls** - trusts the kubernetes plugin's cached state
- **Minimal overhead** - just intercepts responses, no blocking operations
- **Simple implementation** - uses ResponseWriter wrapper pattern

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
   git checkout v1.11.1
   ```

2. Add the plugin to `plugin.cfg` **BEFORE** `kubernetes`:
   ```
   emptyendpoints:github.com/lavocatt/artemis-coredns-plugin
   kubernetes:kubernetes
   ```

   **Important**: The plugin MUST be before `kubernetes` to wrap the ResponseWriter.

3. Build:
   ```bash
   go generate
   go build
   ```

## Configuration

In your Corefile, place `emptyendpoints` **BEFORE** `kubernetes`:

```
.:53 {
    emptyendpoints serverless-broker  # watch only this namespace
    kubernetes cluster.local {
        pods insecure
        fallthrough in-addr.arpa ip6.arpa
    }
    forward . /etc/resolv.conf
    prometheus :9153
    cache 5
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
        serverAddress: http://prometheus:9090
        query: |
          sum(increase(coredns_emptyendpoints_queries_total{
            exported_namespace="my-namespace",
            exported_service="my-broker-hdls-svc"
          }[5m])) or vector(0)
        threshold: "1"
```

Note: Prometheus relabels `namespace` → `exported_namespace` and 
`service` → `exported_service` to avoid conflicts with its own labels.

## Why NXDOMAIN?

For **headless services** (ClusterIP: None) with no ready endpoints:
- The kubernetes plugin returns NXDOMAIN because there are no pod IPs to return
- This plugin detects that NXDOMAIN and increments the metric
- KEDA sees the metric and scales up the workload
- Once pods are ready, subsequent DNS queries succeed

For **regular ClusterIP services**:
- DNS always returns the ClusterIP (regardless of endpoints)
- This plugin won't detect the empty endpoints case
- Use headless services for scale-from-zero functionality

## Limitations

- Requires rebuilding CoreDNS with the plugin
- Only works with headless services (ClusterIP: None)
- Only detects DNS queries, not direct IP connections
- Metric is cumulative (use `increase()` in KEDA query)

## Comparison with Previous Approach

Previous implementation made a K8s API call on every DNS query:
```
DNS Query → emptyendpoints → K8s API call → kubernetes plugin
```

Current implementation trusts the kubernetes plugin's cache:
```
DNS Query → kubernetes plugin → emptyendpoints (intercept NXDOMAIN)
```

This is more efficient and doesn't add load to the Kubernetes API server.
