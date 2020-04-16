# Ingress

Most applications deployed in Kubernetes clusters will require network access.
To ease managing access to all those resources, Gravity provides an 
out-of-the-box solution based on the [nginx Ingress](https://github.com/kubernetes/ingress-nginx).

The official nginx Ingress provides the following benefits:

* very simple configuration and management
* very good performance levels without any additional tuning
* multiple integrations with external tools (eg: cert-manager)
* based on the reliable nginx web server

The nginx Ingress is among the officially supported ones by [CNCF](https://www.cncf.io/).
[documentation](https://kubernetes.github.io/ingress-nginx/) for more 
information.

!!! note "Supported version"
    nginx Ingress built-in integration is offered and supported starting from 
    Gravity 7.1

## Enable nginx Ingress

By default nginx Ingress integration is disabled. It can be enabled by setting 
the following field in a cluster image manifest file:

```yaml
ingress:
  nginx:
    enabled: true
```

When nginx Ingress is enabled, it will be packaged in the cluster image tarball
alongside other dependencies during the `tele build` process. During the
cluster installation, nginx Ingress will be installed in the `kube-system` 
namespace via `helm`.

### Enable nginx Ingress During Upgrade

nginx Ingress can be enabled for existing Gravity clusters when upgrading to a 
new version that supports nginx Ingress.

To enable it in the existing cluster:

* Update your cluster image manifest to enable nginx Ingress integration like shown above.
* Build a new version of the cluster image using `tele build`. See [Building a Cluster Image](pack.md#building-a-cluster-image) for details.
* Upgrade the existing cluster to this new version. See [Upgrading a Cluster](cluster.md#updating-a-cluster) for details.

nginx Ingress will be installed and configured during the upgrade operation.

## Configure nginx Ingress

### HTTP

```yaml
---
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: test-ingress
  annotations:
    kubernetes.io/ingress.class: "nginx"
spec:
  rules:
  - http:
      paths:
      - path: /
        backend:
          serviceName: nginx-catchall
          servicePort: 80
      - path: /test
        backend:
          serviceName: nginx-test
          servicePort: 80
```

```yaml
---
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: test-ingress
  annotations:
    kubernetes.io/ingress.class: "nginx"
spec:
  rules:
  - host: example.gravitational.com
    http:
      paths:
      - path: /
        backend:
          serviceName: nginx-with-host
          servicePort: 80
      - path: /status
        backend:
          serviceName: nginx-with-host
          servicePort: 80
```

### HTTPs


```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret-tls
  namespace: default
data:
  tls.crt: base64 encoded cert
  tls.key: base64 encoded key
type: kubernetes.io/tls
```

```yaml
---
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: tls-test-ingress
  tls:
  - hosts:
    - ssl-example.gravitational.com
    secretName: test-secret-tls
  rules:
    - host: ssl-example.gravitational.com
      http:
        paths:
        - path: /
          backend:
            serviceName: service1
            servicePort: 80
```



```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-test
  labels:
    run: nginx-test
spec:
  ports:
  - port: 80
    protocol: TCP
  selector:
    run: nginx-test
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-test
spec:
  selector:
    matchLabels:
      run: nginx-test
  replicas: 2
  template:
    metadata:
      labels:
        run: nginx-test
    spec:
      containers:
      - name: nginx
        image: nginx
        ports:
        - containerPort: 80
```