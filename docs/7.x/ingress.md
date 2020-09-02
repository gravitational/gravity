---
title: Kubernetes Ingress
description: How to configure Kubernetes ingress in air-gapped and on-premise environments
---

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

In order to be able to route network requests to Kubernetes Pods inside Gravity,
you should follow the usual Ingress configuration pattern.

This usually includes creating a new resource of `kind: Ingress` which specifies
how to route requests to Services that address running Pods.

Here's an example of an HTTP Ingress, configured to listen to requests sent
toward anyway hostname, and send them to two different services based on the
path of the request.

```yaml
# HTTP Ingress
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

Here's a slightly different of an Ingress which only receives requests sent 
toward the hostname example.gravitational.com hostname and also enables the
'/status' path sending it to a different pod.

```yaml
---
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: test-ingress-host
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
          serviceName: nginx-status-with-host
          servicePort: 80
```


### HTTPS enabled Ingress

In order to create an Ingress which supports SSL/TLS encrypted traffic via HTTPs
you'll have to create a certificate containing the certificate itself.

Usually this involves the use of a dynamic cert-manager, but that goes beyond
the scope of this example.

In this case we'll assume that you already have a certificate saved in two files
called `tls.crt` for the certificate and `tls.key` for the private key file.

!!!NOTE: please note that since the web server underlying this the Ingress is
nginx you will have to include your entire CA anchor chain inside the tls.crt file

To create the certificate, please create the two files explained above and then
run the following command:

```bash
$ kubectl create secret tls example-gravitational-com-cert --cert=tls.crt --key=tls.key
```

Alternatively you could manually create your certificate following the template
below. Remember to base64 encode the data records' content.

```yaml
---
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: example-gravitational-com-cert 
  namespace: default
data:
  tls.crt: base64 encoded cert
  tls.key: base64 encoded key
```

Now you should be able to dd a new Ingress which uses that certificate to enable
HTTPs traffic as showcased below:

```yaml
---
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: tls-test-ingress
  tls:
  - hosts:
    - ssl-example.gravitational.com
    secretName: example-gravitational-com-cert
  rules:
    - host: ssl-example.gravitational.com
      http:
        paths:
        - path: /
          backend:
            serviceName: service1
            servicePort: 80
```

### Testing your Ingress

In case you need a quick way to test our Ingress deployment, we suggest using
the example below, which will create a quick nginx deployment and service that
can then be used to test if the Ingress itself is working fine.

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
