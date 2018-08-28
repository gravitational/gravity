### Gravity App

This is a very simple sample application which you can use to deploy into Gravity.
It has the following structure:
    
* Two pods
    - `looper` pod writes a string into its log forever.
    - `webhead` pod serves a web page.
* `gravityapp` service: exposes `webhead` pod on port #80

Two pods are needed to test cross-pod connectivity. Read on.
       
### Quick Start

Build it:
```bash
> make

Congrats!
Your sample app tarball is ready ---> build/gravity-app.tar.gz
```

Now you can deploy the tarball into Gravity. Ev uses his own `sitectl` tool for this:
```
> sitectl install build/gravity-app.tar.gz
App gravity-app is installed!
```

Make sure it's been installed:
```bash
> sitectl list
default/gravity-app:1.0.0
```

Run it:
```bash
> sitectl run default/gravity-app:1.0.0
Launched 'default/gravity-app:1.0.0'. Check its status with 'get' command
```

Indeed, lets look:
```bash
> sitectl status default/gravity-app:1.0.0
Application 'gravity-app' is running (versions: [1.0.0]). Here are its resources:
	pod: 'looper'
	pod: 'webhead'
	service: 'gravityapp'
	endpoint: 'gravityapp'
```

### Deeper Look

This section assumes you can use `kubectl` tool to examine what's running.
Lets check the status of everything real quick:
```bash
> kubectl getl all

CONTROLLER   CONTAINER(S)   IMAGE(S)   SELECTOR   REPLICAS   AGE
NAME         CLUSTER_IP    EXTERNAL_IP   PORT(S)   SELECTOR       AGE
gravityapp   10.0.161.88                 80/TCP    name=webhead   2m
kubernetes   10.0.0.1      <none>        443/TCP   <none>         13m
NAME      READY     STATUS    RESTARTS   AGE
looper    1/1       Running   0          2m
webhead   1/1       Running   0          2m
NAME      LABELS    STATUS    VOLUME    CAPACITY   ACCESSMODES   AGE
```

Everything appears to be running well. To manually nuke everything run this when you're done:
```bash
> kubectl delete pods --all
> kubectl delete services --all
```

Lets see which IP the service is available on:
```
> kubectl get service gravityapp -o yaml
```

Find `clusterIP` field, assuming it says "10.0.161.88". Keep that IP in mind.
Lets join the `looper` pod:

```bash
> kubectl exec -ti looper /bin/bash
```

Neat, heh? Now lets talk to our service:
```bash
> curl http:://10.0.161.88
<html>
<title>Web Head</title>
<body>
Hello! I am <b>the</b> webhead.
</body>
</html>
```

See? They are talking. If you have SkyDNS working, you should be able to do this
from inside other pods:

```bash
> curl http://gravityapp
```

