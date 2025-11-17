# Kubernetes Deployment

This directory contains Kubernetes manifests for deploying Balance in a Kubernetes cluster.

## Prerequisites

- Kubernetes cluster (1.19+)
- kubectl configured
- (Optional) Prometheus Operator for metrics

## Quick Start

1. Apply all manifests:

```bash
kubectl apply -f deployments/kubernetes/
```

2. Check deployment status:

```bash
kubectl get deployments balance
kubectl get pods -l app=balance
kubectl get svc balance
```

3. Access the service:

```bash
# Get the LoadBalancer IP
kubectl get svc balance

# Or use port-forward for testing
kubectl port-forward svc/balance 8080:80 9090:9090
```

4. Test the load balancer:

```bash
curl http://localhost:8080
```

## Components

### Deployment
- **deployment.yaml**: Main Balance deployment with 3 replicas
  - Resource limits and requests
  - Health checks (liveness and readiness probes)
  - ConfigMap volume mount
  - Prometheus scrape annotations

### Service
- **Service**: Exposes Balance on port 80 (HTTP) and 9090 (admin/metrics)
- Type: LoadBalancer (change to ClusterIP or NodePort as needed)

### ConfigMap
- **configmap.yaml**: Balance configuration
- Edit this to change load balancing behavior

### HPA (Horizontal Pod Autoscaler)
- **hpa.yaml**: Auto-scaling based on CPU and memory
- Scales between 2-10 replicas
- Targets 70% CPU and 80% memory utilization

### ServiceMonitor
- **servicemonitor.yaml**: Prometheus Operator integration
- Requires Prometheus Operator to be installed

## Configuration

### Edit Configuration

```bash
kubectl edit configmap balance-config
```

After editing, restart pods to pick up changes:

```bash
kubectl rollout restart deployment balance
```

### Scale Manually

```bash
kubectl scale deployment balance --replicas=5
```

### Update Image

```bash
kubectl set image deployment/balance balance=balance:v1.1.0
```

## Monitoring

### View Logs

```bash
# All pods
kubectl logs -l app=balance -f

# Specific pod
kubectl logs <pod-name> -f
```

### Health Checks

```bash
# Via port-forward
kubectl port-forward svc/balance 9090:9090

# Then access:
# - http://localhost:9090/health
# - http://localhost:9090/status
# - http://localhost:9090/metrics
```

### Prometheus Metrics

If Prometheus Operator is installed:

```bash
kubectl get servicemonitor balance
```

Metrics will be automatically scraped by Prometheus.

## TLS Configuration

To enable TLS, create a Secret with your certificates:

```bash
kubectl create secret tls balance-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key
```

Then update the ConfigMap to reference the certificates and mount the secret in the Deployment.

## Production Best Practices

1. **Resource Limits**: Adjust based on load testing
   ```yaml
   resources:
     requests:
       cpu: 500m
       memory: 256Mi
     limits:
       cpu: 2000m
       memory: 1Gi
   ```

2. **Pod Disruption Budget**:
   ```yaml
   apiVersion: policy/v1
   kind: PodDisruptionBudget
   metadata:
     name: balance
   spec:
     minAvailable: 2
     selector:
       matchLabels:
         app: balance
   ```

3. **Network Policies**: Restrict traffic
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: balance
   spec:
     podSelector:
       matchLabels:
         app: balance
     policyTypes:
     - Ingress
     ingress:
     - from:
       - podSelector: {}
       ports:
       - protocol: TCP
         port: 8080
   ```

4. **Affinity Rules**: Spread pods across nodes
   ```yaml
   affinity:
     podAntiAffinity:
       preferredDuringSchedulingIgnoredDuringExecution:
       - weight: 100
         podAffinityTerm:
           labelSelector:
             matchLabels:
               app: balance
           topologyKey: kubernetes.io/hostname
   ```

## Troubleshooting

### Pods not starting

```bash
kubectl describe pod <pod-name>
kubectl logs <pod-name>
```

### Configuration issues

```bash
# Validate config inside pod
kubectl exec <pod-name> -- /app/balance-validate -config /app/config/config.yaml
```

### Service not accessible

```bash
kubectl get endpoints balance
kubectl describe svc balance
```

## Cleanup

```bash
kubectl delete -f deployments/kubernetes/
```
