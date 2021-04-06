
## Cluster Troubleshooting

### Using Host Tools from a Pod

In order to use any tool available on the host (in planet) inside a Kubernetes pod,
create a pod resource from a spec like this:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: debug
  labels:
    app: debug
spec:
  containers:
  - name: debug
    image: leader.telekube.local:5000/gravitational/debian-tall:buster
    command: ['/bin/sh', '-c', 'sleep 3600']
    volumeMounts:
    - mountPath: /rootfs
      name: rootfs
    securityContext:
      runAsUser: 0
  volumes:
  - name: rootfs
    hostPath:
      path: /
```

Then, exec into the `debug` container:

```shell
$ kubectl exec -ti debug /bin/sh
```

and use chroot to change the environment to that of the host:

```shell
pod$ chroot /rootfs
```

Now you can use any tool (`nslookup`/`dig`/`curl`/`tcpdump` etc.) from the Pod's `debug` container.
