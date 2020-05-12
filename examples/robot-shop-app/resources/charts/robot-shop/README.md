# Stan's Robot Shop

Use this helm chart to customise your install of Stan's Robot Shop.

### Helm v2.x

```shell
$ helm install --name robot-shop --namespace robot-shop .
```

### Helm v3.x

```bash
$ kubectl create ns robot-shop
$ helm install robot-shop --namespace robot-shop .
```

## Images

By default the images are pulled from Docker Hub. Setting `image.repo` this can be changed, for example:

```shell
$ helm install --set image.repo=eu.gcr.io/acme ...
```

Will pull images from the European Google registry project `acme`.

By default the latest version of the images is pulled. A specific version can be used:

```shell
$ helm install --set image.version=0.1.2 ...
```

It is recommened to always use the latest version.

## Pod Security Policy

If you wish to enable [PSP](https://kubernetes.io/docs/concepts/policy/pod-security-policy/)

```shell
$ helm install --set psp.enabled=true ...
```

## Payment Gateway

By default the `payment` service uses https://www.paypal.com as the pseudo payment provider. The code only does a HTTP GET against this url. You can use a different url.

```shell
$ helm install --set payment.gateway=https://foobar.com ...
```

## End User Monitoring

Optionally End User Monitoring can be enabled for the web pages. Take a look at the [documentation](https://docs.instana.io/products/website_monitoring/) to see how to get a key and an endpoint url.

```shell
$ helm install \
    --set eum.key=xxxxxxxxx \
    --set eum.url=https://eum-eu-west-1.instana.io \
    ...
```

## Use with Minis

When running on `minishift` or `minikube` set `nodeport` to true. The store will then be available on the IP address of your mini and node port of the web service.

```shell
$ mini[kube|shift] ip
192.168.66.101
$ kubectl get svc web
```

Combine the IP and port number to make the URL `http://192.168.66.101:32145`

### MiniShift

Openshift is like K8s but not K8s. Set `openshift` to true or things will break. See the notes and scripts in the OpenShift directory of this repo.

```shell
$ helm install robot-shop --set openshift=true helm
```

