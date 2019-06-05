# Introduction

These are a series of workshops typically delivered by the Gravitational services team. You can also find them with all the resources needed [on GitHub](https://github.com/gravitational/workshop).

We have published three different workshops that you can work through separately or together. They require an intermediate level of knowledge of Linux computer systems. Each workshop should take a 2 to 3 hours to complete.

* [Docker 101 workshop](#docker-101-workshop): An introduction to [Docker](https://www.docker.com/) and its basic concepts.
* [Kubernetes 101 workshop using Minikube](#kubernetes-101-workshop): An introduction to [Kubernetes](https://kubernetes.io/) and its basic concepts.
* [Kubernetes production patterns](Kubernetes-production-patterns): A review of techniques to improve the resiliency and high availability of Kubernetes deployments and some common mistakes to avoid when working with Docker and Kubernetes.

## Docker 101 workshop

An introduction to [Docker](https://www.docker.com/) and its basic concepts

### Requirements

#### Computer and OS

This workshop is written for macOS but should mostly work with Linux as well. You will need a machine with at least `7GB RAM` and `8GB free disk space` available.

#### Docker

For Linux: follow instructions provided [here](https://docs.docker.com/engine/installation/linux/).

If you have macOS (Yosemite or newer), please download Docker for Mac [here](https://download.docker.com/mac/stable/Docker.dmg).

*Older docker package for OSes older than Yosemite -- Docker Toolbox located [here](https://www.docker.com/products/docker-toolbox).*

#### Xcode and local tools

Xcode will install essential console utilities for us. You can install it from the App Store.

### Introduction

#### Hello, world

Docker is as easy as Linux! To prove that let us write classic "Hello, World" in Docker

```bsh
$ docker run busybox echo "hello world"
```

Docker containers are just as simple as Linux processes, but they also provide many more features that we are going to explore.

Let's review the structure of the command:

```bsh
docker run # executes command in a container
busybox    # container image
echo "hello world" # command to run
```

Container image supplies environment - binaries with shell for example that is running the command, so you are not using
host operating system shell, but the shell from `busybox` package when executing Docker run.

#### Sneak peek into container environment

Let's now take a look at process tree running in the container:

```bsh
$ docker run busybox ps uax
```

My terminal prints out something like this:

```bsh
    1 root       0:00 ps uax
```

*NOTE:* Oh my! Am I running this command as root? Yes, although this is not your regular root user but a very limited one. We will get back to the topic of users and security a bit later.

As you can see, the process runs in a very limited and isolated environment, and the PID of the process is 1, so it does not see all other processes
running on your machine.

#### Adding Environment Variables

Let's see what environment variables we have:

```bsh
$ docker run busybox env
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
HOSTNAME=0a0169cdec9a
```

The environment is different from your host environment.
We can extend environment by passing explicit environment variable flag to `docker run`:

```bsh
$ docker run -e HELLO=world busybox env
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
HOSTNAME=8ee8ba3443b6
HELLO=world
HOME=/root
```

#### Adding host mounts

If we look at the disks we will see the OS directories are not here, as well:

```bsh
$ docker run busybox ls -l /home
total 0
```

What if we want to expose our current directory to the container? For this we can use host mounts:

```bsh
$ docker run -v $(pwd):/home busybox ls -l /home
total 72
-rw-rw-r--    1 1000     1000         11315 Nov 23 19:42 LICENSE
-rw-rw-r--    1 1000     1000         30605 Mar 22 23:19 README.md
drwxrwxr-x    2 1000     1000          4096 Nov 23 19:30 conf.d
-rw-rw-r--    1 1000     1000          2922 Mar 23 03:44 docker.md
drwxrwxr-x    2 1000     1000          4096 Nov 23 19:35 img
drwxrwxr-x    4 1000     1000          4096 Nov 23 19:30 mattermost
-rw-rw-r--    1 1000     1000           585 Nov 23 19:30 my-nginx-configmap.yaml
-rw-rw-r--    1 1000     1000           401 Nov 23 19:30 my-nginx-new.yaml
-rw-rw-r--    1 1000     1000           399 Nov 23 19:30 my-nginx-typo.yaml
```

This command "mounted" our current working directory inside the container, so it appears to be "/home"
inside the container! All changes that we do in this repository will be immediately seen in the container's `home`
directory.

#### Network

Networking in Docker containers is isolated, as well. Let us look at the interfaces inside a running container:

```bsh
$ docker run busybox ifconfig
eth0      Link encap:Ethernet  HWaddr 02:42:AC:11:00:02
          inet addr:172.17.0.2  Bcast:0.0.0.0  Mask:255.255.0.0
          inet6 addr: fe80::42:acff:fe11:2/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:1 errors:0 dropped:0 overruns:0 frame:0
          TX packets:1 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:90 (90.0 B)  TX bytes:90 (90.0 B)

lo        Link encap:Local Loopback
          inet addr:127.0.0.1  Mask:255.0.0.0
          inet6 addr: ::1/128 Scope:Host
          UP LOOPBACK RUNNING  MTU:65536  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:0 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1
          RX bytes:0 (0.0 B)  TX bytes:0 (0.0 B)
```


We can use `-p` flag to forward a port on the host to the port 5000 inside the container:


```bsh
$ docker run -p 5000:5000 library/python:3.3 python -m http.server 5000
```

This command blocks because the server listens for requests, open a new tab and access the endpoint

```bsh
$ curl http://localhost:5000
<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">
<html>
<head>
....
```

Press `Ctrl-C` to stop the running container.


#### A bit of background

![docker-settings](images/containers.png)

A Docker container is a set of Linux processes that run isolated from the rest of the processes. Multiple Linux subsystems help to create a container concept.

**Namespaces**

Namespaces create isolated stacks of Linux primitives for a running process.

* NET namespace creates a separate networking stack for the container, with its own routing tables and devices
* PID namespace is used to assign isolated process IDs that are separate from host OS. For example, this is important if we want to send signals to a running
process.
* MNT namespace creates a scoped view of a filesystem using [VFS](http://www.tldp.org/LDP/khg/HyperNews/get/fs/vfstour.html). It lets a container
to get its own "root" filesystem and map directories from one location on the host to the other location inside container.
* UTS namespace lets container to get to its own hostname.
* IPC namespace is used to isolate inter-process communication (e.g. message queues).
* USER namespace allows container processes have different users and IDs from the host OS.

**Control groups**

Kernel feature that limits, accounts for, and isolates the resource usage (CPU, memory, disk I/O, network, etc.)

**Capabilities**

Capabilities provide enhanced permission checks on the running process, and can limit the interface configuration, even for a root user - for example (`CAP_NET_ADMIN`)

You can find a lot of additional low level detail [here](http://crosbymichael.com/creating-containers-part-1.html).


#### More container operations

**Daemons**

Our last python server example was inconvenient as it worked in foreground:

```bsh
$ docker run -d -p 5000:5000 --name=simple1 library/python:3.3 python -m http.server 5000
```

Flag `-d` instructs Docker to start the process in background. Let's see if still works:

```bsh
$ curl http://localhost:5000
```

**Inspecting a running container**

We can use `ps` command to view all running containers:

```bsh
$ docker ps
CONTAINER ID        IMAGE                COMMAND                  CREATED             STATUS              PORTS                    NAMES
eea49c9314db        library/python:3.3   "python -m http.serve"   3 seconds ago       Up 2 seconds        0.0.0.0:5000->5000/tcp   simple1
```

* Container ID - auto generated unique running id.
* Container image - image name.
* Command - Linux process running as the PID 1 in the container.
* Names - user friendly name of the container, we have named our container with `--name=simple1` flag.

We can use `logs` to view logs of a running container:

```bsh
$ docker logs simple1
```

**Attaching to a running container**

We can execute a process that joins container namespaces using `exec` command:

```bsh
$ docker exec -ti simple1 /bin/sh
```

We can look around to see the process running as PID 1:

```bsh
# ps uax
USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root         1  0.5  0.0  74456 17512 ?        Ss   18:07   0:00 python -m http.server 5000
root         7  0.0  0.0   4336   748 ?        Ss   18:08   0:00 /bin/sh
root        13  0.0  0.0  19188  2284 ?        R+   18:08   0:00 ps uax
#
```

This gives an illusion that you `SSH` in a container. However, there is no remote network connection.
The process `/bin/sh` started an instead of running in the host OS joined all namespaces of the container.

* `-t` flag attaches terminal for interactive typing.
* `-i` flag attaches input/output from the terminal to the process.

**Starting and stopping containers**

To stop and start container we can use `stop` and `start` commands:

```bsh
$ docker stop simple1
$ docker start simple1
```

**NOTE:** container names should be unique. Otherwise, you will get an error when you try to create a new container with a conflicting name!

**Interactive containers**

`-it` combination allows us to start interactive containers without attaching to existing ones:

```bsh
$ docker run -ti busybox
# ps uax
PID   USER     TIME   COMMAND
    1 root       0:00 sh
    7 root       0:00 ps uax
```

**Attaching to containers input**

To best illustrate the impact of `-i` or `--interactive` in the expanded version, consider this example:

```bsh
$ echo "hello there " | docker run busybox grep hello
```

The example above won't work as the container's input is not attached to the host stdout. The `-i` flag fixes just that:

```bsh
$ echo "hello there " | docker run -i busybox grep hello
hello there
```

#### Building Container images

So far we have been using container images downloaded from Docker's public registry.

**Starting from scratch**

`Dockerfile` is a special file that instructs `docker build` command how to build an image

```bsh
$ cd docker/scratch
$ docker build -t hello .
Sending build context to Docker daemon 3.072 kB
Step 1 : FROM scratch
 --->
Step 2 : ADD hello.sh /hello.sh
 ---> 4dce466cf3de
Removing intermediate container dc8a5b93d5a8
Successfully built 4dce466cf3de
```


The Dockerfile looks very simple:

```bsh
FROM scratch
ADD hello.sh /hello.sh
```

`FROM scratch` instructs a Docker build process to use empty image to start building the container image.
`ADD hello.sh /hello.sh` adds file `hello.sh` to the container's root path `/hello.sh`.

**Viewing images**

`docker images` command is used to display images that we have built:

```bsh
$ docker images
REPOSITORY                                    TAG                 IMAGE ID            CREATED             SIZE
hello                                         latest              4dce466cf3de        10 minutes ago      34 B
```

* Repository - a name of the local (on your computer) or remote repository. Our current repository is local and is called `hello`.
* Tag - indicates the version of our image, Docker sets `latest` tag automatically if not specified.
* Image ID - unique image ID.
* Size - the size of our image is just 34 bytes.

**NOTE:** Docker images are very different from virtual image formats. Because Docker does not boot any operating system, but simply runs
Linux process in isolation, we don't need any kernel, drivers or libraries to ship with the image, so it could be as tiny as several bytes!


**Running the image**

Trying to run it though, will result in the error:

```bsh
$ docker run hello /hello.sh
write pipe: bad file descriptor
```

This is because our container is empty. There is no shell and the script won't be able to start!
Let's fix that by changing our base image to `busybox` that contains a proper shell environment:


```bsh
$ cd docker/busybox
$ docker build -t hello .
Sending build context to Docker daemon 3.072 kB
Step 1 : FROM busybox
 ---> 00f017a8c2a6
Step 2 : ADD hello.sh /hello.sh
 ---> c8c3f1ea6ede
Removing intermediate container fa59f3921ff8
Successfully built c8c3f1ea6ede
```

Listing the image shows that image id and size have changed:

```bsh
$ docker images
REPOSITORY                                    TAG                 IMAGE ID            CREATED             SIZE
hello                                         latest              c8c3f1ea6ede        10 minutes ago      1.11 MB
```

We can run our script now:

```bsh
$ docker run hello /hello.sh
hello, world!
```

**Versioning**

Let us roll a new version of our script `v2`

```bsh
$ cd docker/busybox-v2
docker build -t hello:v2 .
```

We will now see 2 images: `hello:v2` and `hello:latest`

```bsh
$ docker images
hello                                         v2                  195aa31a5e4d        2 seconds ago       1.11 MB
hello                                         latest              47060b048841        20 minutes ago      1.11 MB
```

**NOTE:** Tag `latest` will not automatically point to the latest version, so you have to manually update it

Execute the script using `image:tag` notation:

```bsh
$ docker run hello:v2 /hello.sh
hello, world v2!
```

**Entrypoint**

We can improve our image by supplying `entrypoint`:


```bsh
$ cd docker/busybox-entrypoint
$ docker build -t hello:v3 .
```

`ENTRYPOINT` remembers the command to be executed on start, even if you don't supply the arguments:

```bsh
$ docker run hello:v3
hello, world !
```

What happens if you pass flags? They will be executed as arguments:

```bsh
$ docker run hello:v3 woo
hello, world woo!
```

This magic happens because our v3 script prints passed arguments:

```bsh
#!/bin/sh

echo "hello, world $@!"
```


**Environment variables**

We can pass environment variables during build and during runtime as well.

Here's our modified shell script:

```bsh
#!/bin/sh

echo "hello, $BUILD1 and $RUN1!"
```

Dockerfile now uses `ENV` directive to provide environment variable:

```bsh
FROM busybox
ADD hello.sh /hello.sh
ENV BUILD1 Bob
ENTRYPOINT ["/hello.sh"]
```

Let's build and run:

```bsh
cd docker/busybox-env
$ docker build -t hello:v4 .
$ docker run -e RUN1=Alice hello:v4
hello, Bob and Alice!
```

**Build arguments**

Sometimes it is helpful to supply arguments during build process
(for example, user ID to create inside the container). We can supply build arguments as flags to `docker build`:


```bsh
$ cd docker/busybox-arg
$ docker build --build-arg=BUILD1="Alice and Bob" -t hello:v5 .
$ docker run hello:v5
hello, Alice and Bob!
```

Here is our updated Dockerfile:

```bsh
FROM busybox
ADD hello.sh /hello.sh
ARG BUILD1
ENV BUILD1 $BUILD1
ENTRYPOINT ["/hello.sh"]
```

Notice how `ARG` have supplied the build argument and we have referred to it right away, exposing it as environment variable right away.

**Build layers and caching**

Let's take a look at the new build image in the `docker/cache` directory:

```bsh
$ ls -l docker/cache/
total 12
-rw-rw-r-- 1 sasha sasha 76 Mar 24 16:23 Dockerfile
-rw-rw-r-- 1 sasha sasha  6 Mar 24 16:23 file
-rwxrwxr-x 1 sasha sasha 40 Mar 24 16:23 script.sh
```

We have a file and a script that uses the file:

```bsh
$ cd docker/cache
$ docker build -t hello:v6 .

Sending build context to Docker daemon 4.096 kB
Step 1 : FROM busybox
 ---> 00f017a8c2a6
Step 2 : ADD file /file
 ---> Using cache
 ---> 6f48df47cb1d
Step 3 : ADD script.sh /script.sh
 ---> b052fd11bcc6
Removing intermediate container c555e8ab29dc
Step 4 : ENTRYPOINT /script.sh
 ---> Running in 50f057fd89cb
 ---> db7c6f36cba1
Removing intermediate container 50f057fd89cb
Successfully built db7c6f36cba1

$ docker run hello:v6
hello, hello!
```

Let's update the script.sh

```bsh
$ cp script2.sh script.sh
```

They are only different by one letter, but this makes a difference:


```bsh
$ docker build -t hello:v7 .
$ docker run hello:v7
Hello, hello!
```

Notice `Using cache` diagnostic output from the container:

```bsh
$ docker build -t hello:v7 .
Sending build context to Docker daemon  5.12 kB
Step 1 : FROM busybox
 ---> 00f017a8c2a6
Step 2 : ADD file /file
 ---> Using cache
 ---> 6f48df47cb1d
Step 3 : ADD script.sh /script.sh
 ---> b187172076e2
Removing intermediate container 7afa2631d677
Step 4 : ENTRYPOINT /script.sh
 ---> Running in 51217447e66c
 ---> d0ec3cfed6f7
Removing intermediate container 51217447e66c
Successfully built d0ec3cfed6f7
```


Docker executes every command in a special container. It detects the fact that the content has (or has not) changed,
and instead of re-executing the command, uses cached value instead. This helps to speed up builds, but sometimes introduces problems.

**NOTE:** You can always turn caching off by using the `--no-cache=true` option for the `docker build` command.

Docker images are composed of layers:

![images](https://docs.docker.com/engine/userguide/storagedriver/images/image-layers.jpg)

Every layer is the result of execution of a command in the Dockerfile.

**RUN command**

The most frequently used command is `RUN`: it executes the command in a container,
captures the output and records it as an image layer.


Let's us use existing package managers to compose our images:

```bsh
FROM ubuntu:14.04
RUN apt-get update
RUN apt-get install -y curl
ENTRYPOINT curl
```

The output of this build will look more like a real Linux install:

```bsh
$ cd docker/ubuntu
$ docker build -t myubuntu .
```

We can use our newly created ubuntu to curl pages:

```bsh
$ docker run myubuntu https://google.com
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100   220  100   220    0     0   1377      0 --:--:-- --:--:-- --:--:--  1383
<HTML><HEAD><meta http-equiv="content-type" content="text/html;charset=utf-8">
<TITLE>301 Moved</TITLE></HEAD><BODY>
<H1>301 Moved</H1>
The document has moved
<A HREF="https://www.google.com/">here</A>.
</BODY></HTML>
```

However, it all comes at a price:

```bsh
$ docker images
REPOSITORY                                    TAG                 IMAGE ID            CREATED             SIZE
myubuntu                                      latest              50928f386c70        53 seconds ago      221.8 MB
```

That is 220MB for curl! As we know, now there is no good reason to have images with all the OS inside. If you still need it though, Docker
will save you some space by re-using the base layer, so images with slightly different bases
would not repeat each other.

#### Operations with images

You are already familiar with one command, `docker images`. You can also remove images, tag and untag them.

**Removing images and containers**

Let's start with removing the image that takes too much disk space:

```bsh
$ docker rmi myubuntu
Error response from daemon: conflict: unable to remove repository reference "myubuntu" (must force) - container 292d1e8d5103 is using its referenced image 50928f386c70
```

Docker complains that there are containers using this image. How is this possible? We thought that all our containers are gone.
Actually, Docker keeps track of all containers, even those that have stopped:

```bsh
$ docker ps -a
CONTAINER ID        IMAGE                        COMMAND                   CREATED             STATUS                           PORTS                    NAMES
292d1e8d5103        myubuntu                     "curl https://google."    5 minutes ago       Exited (0) 5 minutes ago                                  cranky_lalande
f79c361a24f9        440a0da6d69e                 "/bin/sh -c curl"         5 minutes ago       Exited (2) 5 minutes ago                                  nauseous_sinoussi
01825fd28a50        440a0da6d69e                 "/bin/sh -c curl --he"    6 minutes ago       Exited (2) 5 minutes ago                                  high_davinci
95ffb2131c89        440a0da6d69e                 "/bin/sh -c curl http"    6 minutes ago       Exited (2) 6 minutes ago                                  lonely_sinoussi
```

We can now delete the container:

```bsh
$ docker rm 292d1e8d5103
292d1e8d5103
```

and the image:

```bsh
$ docker rmi myubuntu
Untagged: myubuntu:latest
Deleted: sha256:50928f386c704610fb16d3ca971904f3150f3702db962a4770958b8bedd9759b
```

**Tagging images**

`docker tag` helps us to tag images.

We have quite a lot of versions of `hello` built, but latest still points to the old `v1`.

```bsh
$ docker images | grep hello
hello                                         v7                  d0ec3cfed6f7        33 minutes ago      1.11 MB
hello                                         v6                  db7c6f36cba1        42 minutes ago      1.11 MB
hello                                         v5                  1fbecb029c8e        About an hour ago   1.11 MB
hello                                         v4                  ddb5bc88ebf9        About an hour ago   1.11 MB
hello                                         v3                  eb07be15b16a        About an hour ago   1.11 MB
hello                                         v2                  195aa31a5e4d        3 hours ago         1.11 MB
hello                                         latest              47060b048841        3 hours ago         1.11 MB
```

Let's change that by re-tagging `latest` to `v7`:

```bsh
$ docker tag hello:v7 hello:latest
$ docker images | grep hello
hello                                         latest              d0ec3cfed6f7        38 minutes ago      1.11 MB
hello                                         v7                  d0ec3cfed6f7        38 minutes ago      1.11 MB
hello                                         v6                  db7c6f36cba1        47 minutes ago      1.11 MB
hello                                         v5                  1fbecb029c8e        About an hour ago   1.11 MB
hello                                         v4                  ddb5bc88ebf9        About an hour ago   1.11 MB
hello                                         v3                  eb07be15b16a        About an hour ago   1.11 MB
hello                                         v2                  195aa31a5e4d        3 hours ago         1.11 MB
```

Both `v7` and `latest` point to the same image ID `d0ec3cfed6f7`.


**Publishing images**

Images are distributed with a special service - `docker registry`.
Let us spin up a local registry:

```bsh
$ docker run -p 5000:5000 --name registry -d registry:2
```

`docker push` is used to publish images to registries.

To instruct where we want to publish, we need to append registry address to repository name:

```bsh
$ docker tag hello:v7 127.0.0.1:5000/hello:v7
$ docker push 127.0.0.1:5000/hello:v7
```

`docker push` pushed the image to our "remote" registry.

We can now download the image using the `docker pull` command:

```bsh
$ docker pull 127.0.0.1:5000/hello:v7
v7: Pulling from hello
Digest: sha256:c472a7ec8ab2b0db8d0839043b24dbda75ca6fa8816cfb6a58e7aaf3714a1423
Status: Image is up to date for 127.0.0.1:5000/hello:v7
```

#### Wrapping up

We have learned how to start, build and publish containers and learned the containers building blocks.
However, there is much more to learn. Check out the [official docker documentation!](https://docs.docker.com/engine/userguide/) for more information.


## Kubernetes 101 Workshop

This is an introduction to Kubernetes and basic Kubernetes concepts. We will set up Kubernetes and guide you through an interactive tutorial to show you how Pods, Services, Deployments, Configuration (ConfigMaps) and Networking (Ingress) work using Kubernetes.

### Requirements

This workshop is written for macOS or Linux. You will need a machine with at least `7GB RAM` and `8GB free disk space` available.

In addition, you will need to install:

* docker
* VirtualBox
* kubectl
* minikube

#### Docker

For Linux: follow instructions provided [here](https://docs.docker.com/engine/installation/linux/).

If you have macOS (Yosemite or newer), please download Docker for Mac [here](https://download.docker.com/mac/stable/Docker.dmg).

*Older docker package for OSes older than Yosemite -- Docker Toolbox located [here](https://www.docker.com/products/docker-toolbox).*

#### VirtualBox

Let’s install VirtualBox first.

Get latest stable version from https://www.virtualbox.org/wiki/Downloads

#### Kubectl

For macOS:

```bsh
$ curl -O https://storage.googleapis.com/kubernetes-release/release/v1.3.8/bin/darwin/amd64/kubectl \
        && chmod +x kubectl && sudo mv kubectl /usr/local/bin/
```

For Linux:

```
$ curl -O https://storage.googleapis.com/kubernetes-release/release/v1.3.8/bin/linux/amd64/kubectl \
        && chmod +x kubectl && sudo mv kubectl /usr/local/biimagesn/
```

#### Xcode and local tools

Xcode will install essential console utilities for us. You can install it from the App Store.

#### Minikube

For macOS:

```
$ curl -Lo minikube https://storage.googleapis.com/minikube/releases/v0.12.2/minikube-darwin-amd64 \
        && chmod +x minikube && sudo mv minikube /usr/local/bin/
```

For Linux:

```
$ curl -Lo minikube https://storage.googleapis.com/minikube/releases/v0.12.2/minikube-linux-amd64 \
        && chmod +x minikube && sudo mv minikube /usr/local/bin/
```

Also, you can install drivers for various VM providers to optimize your minikube VM performance.
Instructions can be found here: https://github.com/kubernetes/minikube/blob/master/DRIVERS.md.

To run a cluster:

```
$ minikube start
kubectl get nodes
minikube ssh
docker run -p 5000:5000 --name registry -d registry:2
```

**Notice for macOS users:** you need to allow your docker daemon to work with your local insecure registry. It could be achieved via adding VM address to Docker for Mac.

1. Get minikube VM IP via calling `minikube ip`
2. Add obtained IP with port 5000 (specified above in `docker run` command) to Docker insecure registries:

![docker-settings](images/macos-docker-settings.jpg)
images
### Quick Example - Running NGINX

Everyone says that Kubernetes is hard. However, we'll see it's pretty easy to get started -
let's create an NGINX service.

```
$ kubectl run my-nginx --image=nginx --replicas=2 --port=80 --record
$ kubectl expose deployment my-nginx --type=LoadBalancer --port=80
```

Now let's explore what just happened.

* [Pods](http://kubernetes.io/docs/user-guide/pods/) are a building block
of the infrastructure. In essence this is a group of containers sharing the same networking and host
Linux namespaces. They are used to group related processes together. Our `run` command resulted in several running pods:


```
$ kubectl get pods

NAME                        READY     STATUS    RESTARTS   AGE
my-nginx-3800858182-auusv   1/1       Running   0          32m
my-nginx-3800858182-jzoxe   1/1       Running   0          32m
```

You can explore individual pods or group of pods using handy `kubectl describe`

```
$ kubectl describe pods

Name:		my-nginx-3800858182-auusv
Namespace:	default
Node:		172.28.128.5/172.28.128.5
Start Time:	Sun, 15 May 2016 19:37:01 +0000
Labels:		pod-template-hash=3800858182,run=my-nginx
Status:		Running
IP:		10.244.33.109
Controllers:	ReplicaSet/my-nginx-3800858182
Containers:
  my-nginx:
    Container ID:	docker://f322f42081024e8374d23765652d3abc4cb1f28d3cfd4ed37a7dd0c990c12c5f
    Image:		nginx
    Image ID:		docker://44d8b6f34ba13fdbf1da947d4bc6467eadae1cc84c2090011803f7b0862ea124
    Port:		80/TCP
    QoS Tier:
      cpu:		BestEffort
      memory:		BestEffort
    State:		Running
      Started:		Sun, 15 May 2016 19:37:36 +0000
    Ready:		True
    Restart Count:	0
    Environment Variables:
Conditions:
  Type		Status
  Ready 	True
Volumes:
  default-token-8n3l2:
    Type:	Secret (a volume populated by a Secret)
    SecretName:	default-token-8n3l2
Events:
  FirstSeen	LastSeen	Count	From			SubobjectPath			Type		Reason		Message
  ---------	--------	-----	----			-------------			--------	------		-------
  33m		33m		1	{default-scheduler }					Normal		Scheduled	Successfully assigned my-nginx-3800858182-auusv to 172.28.128.5
  33m		33m		1	{kubelet 172.28.128.5}	spec.containers{my-nginx}	Normal		Pulling		pulling image "nginx"
  32m		32m		1	{kubelet 172.28.128.5}	spec.containers{my-nginx}	Normal		Pulled		Successfully pulled image "nginx"
  32m		32m		1	{kubelet 172.28.128.5}	spec.containers{my-nginx}	Normal		Created		Created container with docker id f322f4208102
  32m		32m		1	{kubelet 172.28.128.5}	spec.containers{my-nginx}	Normal		Started		Started container with docker id f322f4208102
```

Let's see what's inside the pod.

**Pod IPs**

You can spot IP in the overlay network assigned to pod. In my case it's `10.244.33.109`. Can we access it directly?

Let's try and see!

```
$ kubectl run -i -t --rm cli --image=tutum/curl --restart=Never
$ curl http://10.244.33.109
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
...
```

Whoa! That worked! Our `sandbox` machine is joined to the cluster's overlay network, so you can access it directly, however in practice
that's rarely necessary.

### Pod Containers

In our nginx pod there's only one running container `my-nginx`, however as we've mentioned before we can have multiple
containers running in single Pod.

Our container exposes Port 80. Thanks to overlay network every container can expose the same port on the same machine, and they won't collide.

We can enter pod container using handy `kubectl exec` command:

```
$ kubectl exec -ti my-nginx-3800858182-auusv -c my-nginx -- /bin/
```

Our `kubectl exec` command specified pod id and container name within the pod. `-ti` stands for attach PTY and connect input to the container respectively.

If there's just one container, we can omit the container name within the pod:

```
$ kubectl exec -ti my-nginx-3800858182-auusv /bin/
```

Let's explore our nginx container a bit:

```
$ ps uax
root         1  0.0  0.1  31676  3020 ?        Ss   19:37   0:00 nginx: master p
nginx        5  0.0  0.0  32060  1936 ?        S    19:37   0:00 nginx: worker p
root       265  0.2  0.0  20224  1912 ?        Ss   20:24   0:00 /bin/
root       270  0.0  0.0  17492  1144 ?        R+   20:25   0:00 ps uax
```

as you see, our container has it's own separate PID namespace - nginx process is actually `PID 1`.

```
$ ls -l /var/run/secrets/kubernetes.io/serviceaccount/
```

Kubernetes also mounted special volume in our container `serviceaccount` with access credentials to talk to Kubernetes API process.
Kubernetes uses this technique a lot to mount configuration and secrets into a running container. We will explore this in more detail
a bit later.

We don't need to always run interactive sessions within container, e.g. we can execute command without attaching PTY:

```
$ kubectl exec my-nginx-3800858182-auusv -- /bin/ls -l
total 0
drwxr-xr-x.   1 root root 1190 May  3 18:53 bin
drwxr-xr-x.   1 root root    0 Mar 13 23:46 boot
drwxr-xr-x.   5 root root  380 May 15 19:37 dev
drwxr-xr-x.   1 root root 1646 May 15 19:47 etc
drwxr-xr-x.   1 root root    0 Mar 13 23:46 home
drwxr-xr-x.   1 root root  100 May  4 02:38 lib
drwxr-xr-x.   1 root root   40 May  3 18:52 lib64
drwxr-xr-x.   1 root root    0 May  3 18:52 media
drwxr-xr-x.   1 root root    0 May  3 18:52 mnt
drwxr-xr-x.   1 root root    0 May  3 18:52 opt
dr-xr-xr-x. 151 root root    0 May 15 19:37 proc
drwx------.   1 root root   56 May 15 19:46 root
drwxr-xr-x.   1 root root   48 May 15 19:37 run
drwxr-xr-x.   1 root root 1344 May  3 18:53 sbin
drwxr-xr-x.   1 root root    0 May  3 18:52 srv
dr-xr-xr-x.  13 root root    0 May 15 17:56 sys
drwxrwxrwt.   1 root root    0 May 15 19:47 tmp
drwxr-xr-x.   1 root root   70 May  4 02:38 usr
drwxr-xr-x.   1 root root   90 May  4 02:38 var
```

**Note:** when calling exec, don't forget `--`. You don't need to escape or join command arguments passed to exec really, `kubectl` will simply
send everything after `--` as is.

### Deployments and ReplicaSets

So Kubernetes created 2 Pods for us and that's it? Not really, it's a bit more advanced system and it really thought through the deployment life cycle.
Kubernetes created a deployment with ReplicaSet of 2 pods:

```
$ kubectl get deployments
NAME       DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
my-nginx   2         2         2            2           1h
```

```
$ kubectl get replicasets
NAME                  DESIRED   CURRENT   AGE
my-nginx-3800858182   2         2         1h
```

Whoa! Lot's of stuff! Let's go through it one by one:

[Deployment](http://kubernetes.io/docs/user-guide/deployments) is a special declarative state of your [Pods](http://kubernetes.io/docs/user-guide/pods) and [ReplicaSets](http://kubernetes.io/docs/user-guide/replicasets).
You simply declare the desire state of your deployment and Kubernetes converges the current state to it.

Every time you update the deployment, it kicks off the update procedure using whatever update strategy you've selected for it.

Let's dig a little deeper into this deployment:

Here we see that it manages 2 replicas of our Pod and using RollingUpdate strategy:

```
$ kubectl describe deployments/my-nginx
Name:			my-nginx
Namespace:		default
CreationTimestamp:	Sun, 15 May 2016 12:37:01 -0700
Labels:			run=my-nginx
Selector:		run=my-nginx
Replicas:		2 updated | 2 total | 2 available | 0 unavailable
StrategyType:		RollingUpdate
MinReadySeconds:	0
RollingUpdateStrategy:	1 max unavailable, 1 max surge
OldReplicaSets:		<none>
NewReplicaSet:		my-nginx-3800858182 (2/2 replicas created)
Events:
  FirstSeen	LastSeen	Count	From				SubobjectPath	Type		Reason			Message
  ---------	--------	-----	----				-------------	--------	------			-------
  1h		1h		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled up replica set my-nginx-3800858182 to 2

```


Events tell us what happened to this deployment in the past. We'll dig a little bit deeper into this deployment later and now let's move on to services!

### Services

Pods, ReplicaSets and Deployments and all done with one command! But that's not all. We need a scalable way to access our services, so k8s team came up with
[Services](http://kubernetes.io/docs/user-guide/services)

Services provide special Virtual IPs load balancing traffic to the set of pods in a replica sets.

```
$ kubectl get services
kubernetes   10.100.0.1     <none>        443/TCP   2h
my-nginx     10.100.68.75   <none>        80/TCP    1h
```

As you see there are two services - one is a system service `kubernetes` that points to Kubernetes API. Another one is `my-nginx` service, pointing to our Pods in a replica sets.

Let's dig a little deeper into services:

```
$ kubectl describe services/my-nginx
Name:			my-nginx
Namespace:		default
Labels:			<none>
Selector:		run=my-nginx
Type:			ClusterIP
IP:			10.100.68.75
Port:			<unset>	80/TCP
Endpoints:		10.244.33.109:80,10.244.40.109:80
Session Affinity:	None
No events.
```

`ClusterIP` type of the service means that it's an internal IP managed by Kubernetes and not reachable outside. You can create other types of services that play nicely with AWS/GCE and Azure `LoadBalancer`, we'll
dig into that some other time though. Meanwhile, let's notice that there are 2 endpoints:

```
Endpoints:		10.244.33.109:80,10.244.40.109:80
```

Every one of them points to appropriate Pod in the ReplicaSet. As long as pods come and go, this section will be updated, so applications don't
worry about individual Pod locations.

And finally, there's service IP

```
IP:			10.100.68.75
```

This is our VIP that never changes and provides a static piece of configuration making it easier for our components in the system to talk to each other.


```
$ kubectl run -i -t --rm cli --image=tutum/curl --restart=Never
curl http://10.100.68.75
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
    body {
        width: 35em;
        margin: 0 auto;
        font-family: Tahoma, Verdana, Arial, sans-serif;
    }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>
```

It works! Wait, so you need to hard-code this VIP in your configuration? What if it changes from environment to environment?
Thankfully, Kubernetes team thought about this as well, and we can simply do:

```
$ kubectl run -i -t --rm cli --image=tutum/curl --restart=Never
$ curl http://my-nginx
<!DOCTYPE html>
...
```

Kubernetes is integrated with [SkyDNS](https://github.com/kubernetes/kubernetes/tree/master/cluster/addons/dns) service
that watches the services and pods and sets up appropriate `A` records. Our `sandbox` local DNS server is simply configured to
point to the DNS service provided by Kubernetes.

That's very similar how Kubernetes manages discovery in containers as well. Let's login into one of the nginx boxes and
discover `/etc/resolv.conf` there:


```
$ kubectl exec -ti my-nginx-3800858182-auusv -- /bin/
root@my-nginx-3800858182-auusv:/# cat /etc/resolv.conf

nameserver 10.100.0.4
search default.svc.cluster.local svc.cluster.local cluster.local hsd1.ca.comcast.net
options ndots:5
```

As you see, `resolv.conf` is set up to point to the DNS resolution service managed by Kubernetes.

### Back to Deployments

The power of Deployments comes from ability to do smart upgrades and rollbacks in case if something goes wrong.

Let's update our deployment of nginx to the newer version.

```
$ cat my-nginx-new.yaml
```

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    run: my-nginx
  name: my-nginx
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      run: my-nginx
  template:
    metadata:
      labels:
        run: my-nginx
    spec:
      containers:
      - image: nginx:1.11.5
        name: my-nginx
        ports:
        - containerPort: 80
          protocol: TCP
```

Let's apply our deployment:

```
$ kubectl apply -f my-nginx-new.yaml
```


We can see that a new ReplicaSet has been created

```
$ kubectl get rs

NAME                  DESIRED   CURRENT   AGE
my-nginx-1413250935   2         2         50s
my-nginx-3800858182   0         0         2h
```

If we look at the events section of the deployment we will see how it performed rolling update
scaling up new ReplicaSet and scaling down old ReplicaSet:


```
$ kubectl describe deployments/my-nginx
Name:			my-nginx
Namespace:		default
CreationTimestamp:	Sun, 15 May 2016 19:37:01 +0000
Labels:			run=my-nginx
Selector:		run=my-nginx
Replicas:		2 updated | 2 total | 2 available | 0 unavailable
StrategyType:		RollingUpdate
MinReadySeconds:	0
RollingUpdateStrategy:	1 max unavailable, 1 max surge
OldReplicaSets:		<none>
NewReplicaSet:		my-nginx-1413250935 (2/2 replicas created)
Events:
  FirstSeen	LastSeen	Count	From				SubobjectPath	Type		Reason			Message
  ---------	--------	-----	----				-------------	--------	------			-------
  2h		2h		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled up replica set my-nginx-3800858182 to 2
  1m		1m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled up replica set my-nginx-1413250935 to 1
  1m		1m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled down replica set my-nginx-3800858182 to 1
  1m		1m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled up replica set my-nginx-1413250935 to 2
  1m		1m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled down replica set my-nginx-3800858182 to 0
```

And now it's `1.11.5`, let's check out in the headers:

```
$ kubectl run -i -t --rm cli --image=tutum/curl --restart=Never
$ curl -v http://my-nginx

* About to connect() to my-nginx port 80 (#0)
*   Trying 10.100.68.75...
* Connected to my-nginx (10.100.68.75) port 80 (#0)
> GET / HTTP/1.1
> User-Agent: curl/7.29.0
> Host: my-nginx
> Accept: */*
>
< HTTP/1.1 200 OK
< Server: nginx/1.9.1
```

Let's simulate a situation when a deployment fails and we need to rollback. Our deployment has a typo

```
$ cat my-nginx-typo.yaml
```

```
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    run: my-nginx
  name: my-nginx
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      run: my-nginx
  template:
    metadata:
      labels:
        run: my-nginx
    spec:
      containers:
      - image: nginx:1.91
        name: my-nginx
        ports:
        - containerPort: 80
          protocol: TCP
```

Let's apply a bad configuration:

```shell
$ kubectl apply -f my-nginx-typo.yaml
$ deployment "my-nginx" configured
```

Our new pods have crashed:

```
$ kubectl get pods
NAME                        READY     STATUS             RESTARTS   AGE
my-nginx-1413250935-rqstg   1/1       Running            0          10m
my-nginx-2896527177-8wmk7   0/1       ImagePullBackOff   0          55s
my-nginx-2896527177-cv3fd   0/1       ImagePullBackOff   0          55s
```

Our deployment shows 2 unavailable replicas:

```
$ kubectl describe deployments/my-nginx
Name:			my-nginx
Namespace:		default
CreationTimestamp:	Sun, 15 May 2016 19:37:01 +0000
Labels:			run=my-nginx
Selector:		run=my-nginx
Replicas:		2 updated | 2 total | 1 available | 2 unavailable
StrategyType:		RollingUpdate
MinReadySeconds:	0
RollingUpdateStrategy:	1 max unavailable, 1 max surge
OldReplicaSets:		my-nginx-1413250935 (1/1 replicas created)
NewReplicaSet:		my-nginx-2896527177 (2/2 replicas created)
Events:
  FirstSeen	LastSeen	Count	From				SubobjectPath	Type		Reason			Message
  ---------	--------	-----	----				-------------	--------	------			-------
  2h		2h		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled up replica set my-nginx-3800858182 to 2
  11m		11m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled up replica set my-nginx-1413250935 to 1
  11m		11m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled down replica set my-nginx-3800858182 to 1
  11m		11m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled up replica set my-nginx-1413250935 to 2
  10m		10m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled down replica set my-nginx-3800858182 to 0
  1m		1m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled up replica set my-nginx-2896527177 to 1
  1m		1m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled down replica set my-nginx-1413250935 to 1
  1m		1m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled up replica set my-nginx-2896527177 to 2
```

Our rollout has stopped. Let's view the history:

```
$ kubectl rollout history deployments/my-nginx
deployments "my-nginx":
REVISION	CHANGE-CAUSE
1		kubectl run my-nginx --image=nginx --replicas=2 --port=80 --expose --record
2		kubectl apply -f my-nginx-new.yaml
3		kubectl apply -f my-nginx-typo.yaml
```

**Note:** We used `--record` flag and now all commands are recorded!

Let's roll back the last deployment:

```
$ kubectl rollout undo deployment/my-nginx
```

We've created a new revision by doing `undo`:

```
$ kubectl rollout history deployment/my-nginx
deployments "my-nginx":
REVISION	CHANGE-CAUSE
1		kubectl run my-nginx --image=nginx --replicas=2 --port=80 --expose --record
3		kubectl apply -f my-nginx-typo.yaml
4		kubectl apply -f my-nginx-new.yaml
```

[Deployments](http://kubernetes.io/docs/user-guide/deployments/) are a very powerful tool, and we've barely scratched the surface of what they can do. Check out [docs](http://kubernetes.io/docs/user-guide/deployments/) for more detail.


### Configuration management basics

Well, our `nginx`es are up and running, let's make sure they actually do something useful by configuring them to say `hello, kubernetes!`

[ConfigMap](http://kubernetes.io/docs/user-guide/configmap/) is a special Kubernetes resource that maps to configuration files or environment variables inside a Pod.

Lets create ConfigMap from a directory. Our `conf.d` contains a `default.conf` file:

```
$ cat conf.d/default.conf
server {
    listen       80;
    server_name  localhost;

    location / {
        return 200 'hello, Kubernetes!';
    }
}
```

We can convert the whole directory into ConfigMap:

```
$ kubectl create configmap my-nginx-v1 --from-file=conf.d
$ configmap "my-nginx-v1" created
```

```
$ kubectl describe configmaps/my-nginx-v1
Name:		my-nginx-v1
Namespace:	default
Labels:		<none>
Annotations:	<none>

Data
====
default.conf:	125 bytes

```

Every file is now it's own property, e.g. `default.conf`. Now, the trick is to mount this ConfigMap in the `/etc/nginx/conf.d/`
of our nginxes. We will use new deployment for this purpose:


```
$ cat my-nginx-configmap.yaml
```

```
kind: Deployment
metadata:
  labels:
    run: my-nginx
  name: my-nginx
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      run: my-nginx
  template:
    metadata:
      labels:
        run: my-nginx
    spec:
      containers:
      - image: nginx:1.9.1
        name: my-nginx
        ports:
        - containerPort: 80
          protocol: TCP
        volumeMounts:
        - name: config-volume
          mountPath: /etc/nginx/conf.d
      volumes:
       - name: config-volume
         configMap:
           name: my-nginx-v1
```

Notice that we've introduced `volumes` section that tells Kubernetes to attach volumes to the pods. One special volume type we support is `configMap` that
is created on the fly from the ConfigMap resource `my-nginx-v1` that we've just created.

Another part of our config is `volumeMounts` that are specified for each container and tell it where to mount the volume.

Let's apply our ConfigMap:

```
$ kubectl apply -f my-nginx-configmap.yaml
```

Just as usual, new pods have been created:

```
$ kubectl get pods
NAME                        READY     STATUS    RESTARTS   AGE
my-nginx-3885498220-0c6h0   1/1       Running   0          39s
my-nginx-3885498220-9q61s   1/1       Running   0          38s
```

Out of curiosity, let's login into one of them and see ourselves the mounted ConfigMap:

```
$ kubectl exec -ti my-nginx-3885498220-0c6h0 /bin/
$ cat /etc/nginx/conf.d/default.conf
server {
    listen       80;
    server_name  localhost;

    location / {
        return 200 'hello, Kubernetes!';
    }
}
```

And finally, let's see it all in action:

```
$ kubectl run -i -t --rm cli --image=tutum/curl --restart=Never
$ curl http://my-nginx
hello, Kubernetes!
```

### Connecting services

Let's deploy a bit more complicated stack. In this exercise we will deploy [Mattermost](http://www.mattermost.org) - an alternative to Slack that can run
on your infrastructure. We will build our own containers and configuration, push it to the registry and
Mattermost stack is composed of a worker process that connects to a running PostgreSQL instance.

**Build container**

Let's build container image for our worker and push it to our local private registry:

```
minikube ip
192.168.99.100
cd mattermost/worker
sudo docker build -t $(minikube ip):5000/mattermost-worker:2.1.0 .
sudo docker push $(minikube ip):5000/mattermost-worker:2.1.0
```

**Note:** Notice the `$(minikube ip):5000` prefix. This is a private registry we've set up on our master server.

**Create configmap**

Mattermost's worker expects configuration to be mounted at:

`/var/mattermost/config/config.json`

```
$ cat mattermost/worker-config/config.json
```

If we examine config closely, we will notice that Mattermost expects a connector
string to PostgreSQL:

```
   "DataSource": "postgres://postgres:mattermost@postgres:5432/postgres?sslmode=disable"
   "DataSourceReplicas": ["postgres://postgres:mattermost@postgres:5432/postgres?sslmode=disable"]
```

Here's where Kubernetes power comes into play. We don't need to provide hard-coded IPs, we can simply make sure that
there's a `postres` service pointing to our PostgreSQL DB running somewhere in the cluster.


Let us create ConfigMap based on this file:

```
$ kubectl create configmap mattermost-v1 --from-file=mattermost/worker-config
$ kubectl describe configmaps/mattermost-v1
Name:		mattermost-v1
Namespace:	default
Labels:		<none>
Annotations:	<none>

Data
====
config.json:	2951 bytes
```

**Starting Up Postgres**

Let's create a single Pod running PostgreSQL and point our service to it:

```
$ kubectl create -f mattermost/postgres.yaml
$ kubectl get pods
NAME                        READY     STATUS    RESTARTS   AGE
mattermost-database         1/1       Running   0          12m
```

Let's check out the logs of our PostgreSQL:

```
$ kubectl logs mattermost-database
The files belonging to this database system will be owned by user "postgres".
This user must also own the server process.

The database cluster will be initialized with locale "en_US.utf8".
The default database encoding has accordingly been set to "UTF8".
The default text search configuration will be set to "english".

Data page checksums are disabled.

fixing permissions on existing directory /var/lib/postgresql/data ... ok
creating subdirectories ... ok
selecting default max_connections ... 100
selecting default shared_buffers ... 128MB
```

**Note** Our `mattermost-database` is a special snowflake, in real production systems we must create a proper ReplicaSet
for the stateful service, what is slightly more complicated than this sample.


**Creating Postgres Service**

Let's create PostgreSQL service:

```
$ kubectl create -f mattermost/postgres-service.yaml
```

Let's check out that everything is alright:

```
$ kubectl describe svc/postgres
Name:			postgres
Namespace:		default
Labels:			app=mattermost,role=mattermost-database
Selector:		role=mattermost-database
Type:			NodePort
IP:			    10.100.41.153
Port:			<unset>	5432/TCP
NodePort:		<unset>	31397/TCP
Endpoints:		10.244.40.229:5432
Session Affinity:	None
```

Seems like IP has been allocated and endpoints have been found. Last final touch:

```
$ kubectl run -i -t --rm cli --image=jess/telnet --restart=Never postgres 5432
Trying 10.100.41.153...
Connected to postgres.
Escape character is '^]'.
quit
Connection closed by foreign host.
```

Works!

**Creating Mattermost worker deployment**


```
$ cat mattermost/worker.yaml
```

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: mattermost
    role: mattermost-worker
  name: mattermost-worker
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      role: mattermost-worker
  template:
    metadata:
      labels:
        app: mattermost
        role: mattermost-worker
    spec:
      containers:
      - image: localhost:5000/mattermost-worker:2.1.0
        name: mattermost-worker
        ports:
        - containerPort: 80
          protocol: TCP
        volumeMounts:
        - name: config-volume
          mountPath: /var/mattermost/config
      volumes:
       - name: config-volume
         configMap:
           name: mattermost-v1
```

```
$ kubectl create -f mattermost/worker.yaml --record
```

Let's check out the status of the deployment to see if everything is alright:

```
$ kubectl describe deployments/mattermost-worker
Name:			mattermost-worker
Namespace:		default
CreationTimestamp:	Sun, 15 May 2016 23:56:57 +0000
Labels:			app=mattermost,role=mattermost-worker
Selector:		role=mattermost-worker
Replicas:		1 updated | 1 total | 1 available | 0 unavailable
StrategyType:		RollingUpdate
MinReadySeconds:	0
RollingUpdateStrategy:	1 max unavailable, 1 max surge
OldReplicaSets:		<none>
NewReplicaSet:		mattermost-worker-1848122701 (1/1 replicas created)
Events:
  FirstSeen	LastSeen	Count	From				SubobjectPath	Type		Reason			Message
  ---------	--------	-----	----				-------------	--------	------			-------
  3m		3m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled up replica set mattermost-worker-1932270926 to 1
  1m		1m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled up replica set mattermost-worker-1848122701 to 1
  1m		1m		1	{deployment-controller }			Normal		ScalingReplicaSet	Scaled down replica set mattermost-worker-1932270926 to 0
```

**Creating mattermost service**

Our last touch is to create mattermost service and check how it all works together:

```
$ kubectl create -f mattermost/worker-service.yaml
You have exposed your service on an external port on all nodes in your
cluster.  If you want to expose this service to the external internet, you may
need to set up firewall rules for the service port(s) (tcp:32321) to serve traffic.

See http://releases.k8s.io/release-1.2/docs/user-guide/services-firewalls.md for more details.
service "mattermost" created
```

Hey, wait a second! What was that message about? Let's inspect the service spec:

```
cat mattermost/worker-service.yaml
```

Here's what we got. Notice `NodePort` service type.

```yaml
# service for web worker
apiVersion: v1
kind: Service
metadata:
  name: mattermost
  labels:
    app: mattermost
    role: mattermost-worker
spec:
  type: NodePort
  ports:
  - port: 80
    name: http
  selector:
    role: mattermost-worker
```

`NodePort` service type exposes a static port on every node in the cluster. In this case this port
is `32321`. This is handy sometimes when you are working on-prem or locally.

**Accessing the installation**

```
$ kubectl run -i -t --rm cli --image=tutum/curl --restart=Never
$ curl http://mattermost

<!DOCTYPE html>
<html>

<head>
    <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1">
    <meta name="robots" content="noindex, nofollow">
    <meta name="referrer" content="no-referrer">

    <title>Mattermost - Signup</title>
```

Okay, okay, we need to actually access the website now. Well, that' when `NodePort` comes in handy.
Let's view it a bit closer:

```
$ kubectl describe svc/mattermost
Name:			mattermost
Namespace:		default
Labels:			app=mattermost,role=mattermost-worker
Selector:		role=mattermost-worker
Type:			NodePort
IP:			10.100.226.155
Port:			http	80/TCP
NodePort:		http	32321/TCP
Endpoints:		10.244.40.23:80
Session Affinity:	None
```

Notice this:

```
NodePort:		http	32321/TCP
```

Here we see that on my Vagrant every node in the system should have `IP:32321` resolve to the mattermost web app.
On your Vagrant the port most likely will be different!

So on my computer I can now open mattermost app using one of the nodes IP:


![mattermost](images/mattermost.png)

### Ingress

*Preparation: ingress can be enabled on already running minikube using command:*

```
$ minikube addons enable ingress
```

An Ingress is a collection of rules that allow inbound connections to reach the cluster services.
It can be configured to give services externally-reachable URLs, load balance traffic, terminate SSL, offer name based virtual hosting etc.
The difference between service and ingress (in Kubernetes terminology) is that service allows you to provide access on OSI L3, and ingress
works on L7. E.g. while accessing HTTP server service can provide only load-balancing and HA, unlike ingres which could be used to split
traffic on HTTP location basis, etc.

First, we need to create to 2 different nginx deployments, ConfigMaps and services for them:

```
$ kubectl create configmap cola-nginx --from-file=ingress/conf-cola
$ kubectl create configmap pepsi-nginx --from-file=ingress/conf-pepsi
$ kubectl apply -f ingress/cola-nginx-configmap.yaml -f ingress/pepsi-nginx-configmap.yaml
$ kubectl apply -f ingress/cola-nginx-service.yaml -f ingress/pepsi-nginx-service.yaml
```

Check if both deployments and services works:

```
$ curl $(minikube service cola-nginx --url)
Taste The Feeling. Coca-Cola.
$ curl $(minikube service pepsi-nginx --url)
Every Pepsi Refreshes The World.
```

Example ingress usage pattern is to route HTTP traffic according to location.
Now we have two different deployments and services, assume we need to route user
requests from `/cola` to `cola-nginx` service (backed by `cola-nginx` deployment)
and `/pepsi` to `pepsi-nginx` service.

This can be achieved using following ingress resource:

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: drinks-ingress
  annotations:
    ingress.kubernetes.io/rewrite-target: /
    ingress.kubernetes.io/ssl-redirect: "false"
spec:
  rules:
  - http:
      paths:
      - path: /cola
        backend:
          serviceName: cola-nginx
          servicePort: 80
      - path: /pepsi
        backend:
          serviceName: pepsi-nginx
          servicePort: 80
```

Create ingress:

```
$ kubectl apply -f ingress/drinks-ingress.yaml
```

Notice annotations:

* `ingress.kubernetes.io/rewrite-target: /` -- sets request's location to `/` instead of specified in `path`.
* `ingress.kubernetes.io/ssl-redirect: "false"` -- disables HTTP to HTTPS redirect, enabled by default.

Ingress is implemented inside `kube-system` namespace using any kind of configurable proxy. E.g. in minikube
ingress uses nginx. Simply speaking there's special server which reacts to ingress resource creation/deletion/alteration
and updates configuration of neighboured nginx. This *ingress controller* application started using
ReplicationController resource inside minikube, but could be run as usual Kubernetes application (DS, Deployment, etc),
on special set of "edge router" nodes for improved security.

```
$ kubectl --namespace=kube-system get pods -l app=nginx-ingress-lb
NAME                             READY     STATUS    RESTARTS   AGE
nginx-ingress-controller-1nzsp   1/1       Running   0          1h
```

Now we can make ingress reachable to outer world (e.g. our local host). It's not required, you're free of choice
to make it reachable only internally or via some cloud-provider using LoadBalancer.

```
$ kubectl --namespace=kube-system expose rc nginx-ingress-controller --port=80 --type=LoadBalancer
```

Finally we can check location splitting via hitting ingress-controller service with
proper location.

```
$ curl $(minikube service --namespace=kube-system nginx-ingress-controller --url)/cola
Taste The Feeling. Coca-Cola.
$ curl $(minikube service --namespace=kube-system nginx-ingress-controller --url)/pepsi
Every Pepsi Refreshes The World.
```

As you see, we're hitting one service with different locations and have different responses due
to ingress location routing.

More details on ingress features and use cases [here](https://kubernetes.io/docs/user-guide/ingress/).

### Recap

We've learned several quite important concepts like Services, Pods, ReplicaSet and
ConfigMaps. But that's just a small part of what Kubernetes can do. You can read much more on [Kubernetes website](http://kubernetes.io).


## Kubernetes Production Patterns

... and anti-patterns.

In this workshop, we are going to explore helpful techniques to improve resiliency and high availability
of Kubernetes deployments and we will take a look at some common mistakes to avoid when working with Docker and Kubernetes.

### Requirements

You will need macOS or a Linux OS with at least `7GB RAM` and `8GB free disk space` available.

* docker
* VirtualBox
* kubectl
* minikube

#### Docker

For Linux: follow instructions provided [here](https://docs.docker.com/engine/installation/linux/).

If you have macOS (Yosemite or newer), please download Docker for Mac [here](https://download.docker.com/mac/stable/Docker.dmg).

*Older docker package for OSes older than Yosemite -- Docker Toolbox located [here](https://www.docker.com/products/docker-toolbox).*

#### VirtualBox

Let’s install VirtualBox first.

Get latest stable version from https://www.virtualbox.org/wiki/Downloads

#### Kubectl

For macOS:

    curl -O https://storage.googleapis.com/kubernetes-release/release/v1.3.8/bin/darwin/amd64/kubectl \
        && chmod +x kubectl && sudo mv kubectl /usr/local/bin/

For Linux:

    curl -O https://storage.googleapis.com/kubernetes-release/release/v1.3.8/bin/linux/amd64/kubectl \
        && chmod +x kubectl && sudo mv kubectl /usr/local/biimagesn/

#### Xcode and local tools

Xcode will install essential console utilities for us. You can install it from the App Store.

#### Minikube

For macOS:

    curl -Lo minikube https://storage.googleapis.com/minikube/releases/v0.12.2/minikube-darwin-amd64 \
        && chmod +x minikube && sudo mv minikube /usr/local/bin/

For Linux:

    curl -Lo minikube https://storage.googleapis.com/minikube/releases/v0.12.2/minikube-linux-amd64 \
        && chmod +x minikube && sudo mv minikube /usr/local/bin/

Also, you can install drivers for various VM providers to optimize your minikube VM performance.
Instructions can be found here: https://github.com/kubernetes/minikube/blob/master/DRIVERS.md.

To run a cluster:

```
minikube start
kubectl get nodes
minikube ssh
docker run -p 5000:5000 --name registry -d registry:2
```

**Notice for macOS users:** you need to allow your docker daemon to work with your local insecure registry. It could be achieved via adding VM address to Docker for Mac.

1. Get minikube VM IP via calling `minikube ip`
2. Add obtained IP with port 5000 (specified above in `docker run` command) to Docker insecure registries:

![docker-settings](images/macos-docker-settings.jpg)

### Anti-Pattern: Mixing build environment and runtime environment

Let's take a look at this Dockerfile

```Dockerfile
FROM ubuntu:14.04

RUN apt-get update
RUN apt-get install gcc
RUN gcc hello.c -o /hello
```

It compiles and runs simple "Hello, World" program:

```
$ cd prod/build
$ docker build -t prod .
$ docker run prod
Hello World
```

There is a couple of problems with the resulting Dockerfile:

**Size**

```
$ docker images | grep prod
prod                                          latest              b2c197180350        14 minutes ago      293.7 MB
```

That's almost 300 megabytes to host several kilobytes of the c program! We are bringing in package manager,
C compiler and lots of other unnecessary tools that are not required to run this program.


Which leads us to the second problem:

**Security**

We distribute the whole build tool chain in addition to that we ship the source code of the image:

```
$ docker run --entrypoint=cat prod /build/hello.c
#include<stdio.h>

int main()
{
    printf("Hello World\n");
    return 0;
}
```

**Splitting build envrionment and run environment**

We are going to use "buildbox" pattern to build an image with build environment,
and we will use much smaller runtime environment to run our program


```
$ cd prod/build-fix
$ docker build -f build.dockerfile -t buildbox .
```

**NOTE:** We have used new `-f` flag to specify Dockerfile we are going to use.

Now we have a `buildbox` image that contains our build environment. We can use it to compile the C program now:

```
$ docker run -v $(pwd):/build  buildbox gcc /build/hello.c -o /build/hello
```

We have not used `docker build` this time, but mounted the source code and run the compiler directly.

**NOTE:** Docker will soon support this pattern natively by introducing [build stages](https://github.com/docker/docker/pull/32063) into the build process.


We can now use much simpler (and smaller) Dockerfile to run our image:

```Dockerfile
FROM quay.io/gravitational/debian-tall:stretch

ADD hello /hello
ENTRYPOINT ["/hello"]
```

```
$ docker build -f run.dockerfile -t prod:v2 .
$ docker run prod:v2
Hello World
$ docker images | grep prod
prod                                          v2                  ef93cea87a7c        17 seconds ago       11.05 MB
prod                                          latest              b2c197180350        45 minutes ago       293.7 MB
```

### Anti-Pattern: Zombies and orphans

**NOTICE:** this example demonstration will only work on Linux

**Orphans**

It is quite easy to leave orphaned processes running in background. Let's take an image we have build in the previous example:

```
docker run busybox sleep 10000
```

Let us open a separate terminal and locate the process

```
ps uax | grep sleep
sasha    14171  0.0  0.0 139736 17744 pts/18   Sl+  13:25   0:00 docker run busybox sleep 10000
root     14221  0.1  0.0   1188     4 ?        Ss   13:25   0:00 sleep 10000
```

As you see there are in fact two processes: `docker run` and `sleep 1000` running in a container.

Let's send kill signal to the `docker run` (just as CI/CD job would do for long running processes):

```
kill 14171
```

`docker run` process has not exited, and `sleep` process is running!

```
ps uax | grep sleep
root     14221  0.0  0.0   1188     4 ?        Ss   13:25   0:00 sleep 10000
```


Yelp engineers have a good answer for why this happens [here](https://github.com/Yelp/dumb-init):

> The Linux kernel applies special signal handling to processes which run as PID 1.
> When processes are sent a signal on a normal Linux system, the kernel will first check for any custom handlers the process has registered for that signal, and otherwise fall back to default behavior (for example, killing the process on SIGTERM).

> However, if the process receiving the signal is PID 1, it gets special treatment by the kernel; if it hasn't registered a handler for the signal, the kernel won't fall back to default behavior, and nothing happens. In other words, if your process doesn't explicitly handle these signals, sending it SIGTERM will have no effect at all.

To solve this (and other) issues, you need a simple init system that has proper signal handlers specified. Luckily `Yelp` engineers built the simple and lightweight init system, `dumb-init`

```
docker run quay.io/gravitational/debian-tall /usr/bin/dumb-init /bin/sh -c "sleep 10000"
```

Now you can simply stop `docker run` process using SIGTERM and it will handle shutdown properly

### Anti-Pattern: Direct Use Of Pods

[Kubernetes Pod](https://kubernetes.io/docs/user-guide/pods/#what-is-a-pod) is a building block that itself is not durable.

Do not use Pods directly in production. They won't get rescheduled, retain their data or guarantee any durability.

Instead, you can use `Deployment` with replication factor 1, which will guarantee that pods will get rescheduled
and will survive eviction or node loss.


### Anti-Pattern: Using background processes

```
$ cd prod/background
$ docker build -t $(minikube ip):5000/background:0.0.1 .
$ docker push $(minikube ip):5000/background:0.0.1
$ kubectl create -f crash.yaml
$ kubectl get pods
NAME      READY     STATUS    RESTARTS   AGE
crash     1/1       Running   0          5s
```

The container appears to be running, but let's check if our server is running there:

```
$ kubectl exec -ti crash /bin/
root@crash:/#
root@crash:/#
root@crash:/# ps uax
USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root         1  0.0  0.0  21748  1596 ?        Ss   00:17   0:00 /bin/ /start.sh
root         6  0.0  0.0   5916   612 ?        S    00:17   0:00 sleep 100000
root         7  0.0  0.0  21924  2044 ?        Ss   00:18   0:00 /bin/
root        11  0.0  0.0  19180  1296 ?        R+   00:18   0:00 ps uax
root@crash:/#
```

**Using Probes**

We made a mistake and the HTTP server is not running there but there is no indication of this as the parent
process is still running.

The first obvious fix is to use a proper init system and monitor the status of the web service.
However, let's use this as an opportunity to use liveness probes:


```yaml
apiVersion: v1
kind: Pod
metadata:
  name: fix
  namespace: default
spec:
  containers:
  - command: ['/start.sh']
    image: localhost:5000/background:0.0.1
    name: server
    imagePullPolicy: Always
    livenessProbe:
      httpGet:
        path: /
        port: 5000
      timeoutSeconds: 1
```

```
$ kubectl create -f fix.yaml
```

The liveness probe will fail and the container will get restarted.

```
$ kubectl get pods
NAME      READY     STATUS    RESTARTS   AGE
crash     1/1       Running   0          11m
fix       1/1       Running   1          1m
```

### Production Pattern: Logging

Set up your logs to go to stdout:

```
$ kubectl create -f logs/logs.yaml
$ kubectl logs logs
hello, world!
```

Kubernetes and Docker have a system of plugins to make sure logs sent to stdout and stderr will get
collected, forwarded and rotated.

**NOTE:** This is one of the patterns of [The Twelve Factor App](https://12factor.net/logs) and Kubernetes supports it out of the box!

### Production Pattern: Immutable containers

Every time you write something to container's filesystem, it activates [copy on write strategy](https://docs.docker.com/engine/userguide/storagedriver/imagesandcontainers/#container-and-layers).

New storage layer is created using a storage driver (devicemapper, overlayfs or others). In case of active usage,
it can put a lot of load on storage drivers, especially in case of Devicemapper or BTRFS.

Make sure your containers write data only to volumes. You can use `tmpfs` for small (as tmpfs stores everything in memory) temporary files:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pd
spec:
  containers:
  - image: busybox
    name: test-container
    volumeMounts:
    - mountPath: /tmp
      name: tempdir
  volumes:
  - name: tempdri
    emptyDir: {}
```

### Anti-Pattern: Using `latest` tag

Do not use `latest` tag in production. It creates ambiguity, as it's not clear what real version of the app this is.

It is ok to use `latest` for development purposes, although make sure you set `imagePullPolicy` to `Always`, to make sure
Kubernetes always pulls the latest version when creating a pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: always
  namespace: default
spec:
  containers:
  - command: ['/bin/sh', '-c', "echo hello, world!"]
    image: busybox:latest
    name: server
    imagePullPolicy: Always
```

### Production Pattern: Pod Readiness

Imagine a situation when your container takes some time to start. To simulate this, we are going to write a simple script:

```
#!/bin/

echo "Starting up"
sleep 30
echo "Started up successfully"
python -m http.serve 5000
```

Push the image and start service and deployment:

```yaml
$ cd prod/delay
$ docker build -t $(minikube ip):5000/delay:0.0.1 .
$ docker push $(minikube ip):5000/delay:0.0.1
$ kubectl create -f service.yaml
$ kubectl create -f deployment.yaml
```

Enter curl container inside the cluster and make sure it all works:

```
kubectl run -i -t --rm cli --image=tutum/curl --restart=Never
curl http://delay:5000
<!DOCTYPE html>
...
```

You will notice that there's a `connection refused error`, when you try to access it
for the first 30 seconds.

Update deployment to simulate deploy:

```
$ docker build -t $(minikube ip):5000/delay:0.0.2 .
$ docker push $(minikube ip):5000/delay:0.0.2
$ kubectl replace -f deployment-update.yaml
```

In the next window, let's try to to see if we got any service downtime:

```
curl http://delay:5000
curl: (7) Failed to connect to delay port 5000: Connection refused
```

We've got a production outage despite setting `maxUnavailable: 0` in our rolling update strategy!
This happened because Kubernetes did not know about startup delay and readiness of the service.

Let's fix that by using readiness probe:

```yaml
readinessProbe:
  httpGet:
    path: /
    port: 5000
  timeoutSeconds: 1
  periodSeconds: 5
```

Readiness probe indicates the readiness of the pod containers and Kubernetes will take this into account when
doing a deployment:

```
$ kubectl replace -f deployment-fix.yaml
```

This time we will get no downtime.

### Anti-Pattern: unbound quickly failing jobs

Kubernetes provides new useful tool to schedule containers to perform one-time task: [jobs](https://kubernetes.io/docs/concepts/jobs/run-to-completion-finite-workloads/)

However there is a problem:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: bad
spec:
  template:
    metadata:
      name: bad
    spec:
      restartPolicy: Never
      containers:
      - name: box
        image: busybox
        command: ["/bin/sh", "-c", "exit 1"]
```


```
$ cd prod/jobs
$ kubectl create -f job.yaml
```

You are going to observe the race to create hundreds of containers for the job retrying forever:

```
$ kubectl describe jobs
Name:		bad
Namespace:	default
Image(s):	busybox
Selector:	controller-uid=18a6678e-11d1-11e7-8169-525400c83acf
Parallelism:	1
Completions:	1
Start Time:	Sat, 25 Mar 2017 20:05:41 -0700
Labels:		controller-uid=18a6678e-11d1-11e7-8169-525400c83acf
		job-name=bad
Pods Statuses:	1 Running / 0 Succeeded / 24 Failed
No volumes.
Events:
  FirstSeen	LastSeen	Count	From			SubObjectPath	Type		Reason			Message
  ---------	--------	-----	----			-------------	--------	------			-------
  1m		1m		1	{job-controller }			Normal		SuccessfulCreate	Created pod: bad-fws8g
  1m		1m		1	{job-controller }			Normal		SuccessfulCreate	Created pod: bad-321pk
  1m		1m		1	{job-controller }			Normal		SuccessfulCreate	Created pod: bad-2pxq1
  1m		1m		1	{job-controller }			Normal		SuccessfulCreate	Created pod: bad-kl2tj
  1m		1m		1	{job-controller }			Normal		SuccessfulCreate	Created pod: bad-wfw8q
  1m		1m		1	{job-controller }			Normal		SuccessfulCreate	Created pod: bad-lz0hq
  1m		1m		1	{job-controller }			Normal		SuccessfulCreate	Created pod: bad-0dck0
  1m		1m		1	{job-controller }			Normal		SuccessfulCreate	Created pod: bad-0lm8k
  1m		1m		1	{job-controller }			Normal		SuccessfulCreate	Created pod: bad-q6ctf
  1m		1s		16	{job-controller }			Normal		SuccessfulCreate	(events with common reason combined)

```

Probably not the result you expected. Over time the load on the nodes and docker will be quite substantial,
especially if job is failing very quickly.

Let's clean up the busy failing job first:

```
$ kubectl delete jobs/bad
```

Now let's use `activeDeadlineSeconds` to limit amount of retries:


```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: bound
spec:
  activeDeadlineSeconds: 10
  template:
    metadata:
      name: bound
    spec:
      restartPolicy: Never
      containers:
      - name: box
        image: busybox
        command: ["/bin/sh", "-c", "exit 1"]
```

```
$ kubectl create -f bound.yaml
```

Now you will see that after 10 seconds, the job has failed:

```
  11s		11s		1	{job-controller }			Normal		DeadlineExceeded	Job was active longer than specified deadline
```


**NOTE:** Sometimes it makes sense to retry forever. In this case make sure to set a proper pod restart policy to protect from
accidental DDOS on your cluster.


### Production pattern: Circuit Breaker

In this example, our web application is an imaginary web server for email. To render the page,
our frontend has to make two requests to the backend:

* Talk to the weather service to get current weather
* Fetch current mail from the database

If the weather service is down, user still would like to review the email, so weather service
is auxiliary, while current mail service is critical.

Here is our frontend, weather and mail services written in python:

**Weather**

```python
from flask import Flask
app = Flask(__name__)

@app.route("/")
def hello():
    return '''Pleasanton, CA
Saturday 8:00 PM
Partly Cloudy
12 C
Precipitation: 9%
Humidity: 74%
Wind: 14 km/h
'''

if __name__ == "__main__":
    app.run(host='0.0.0.0')
    ```

**Mail**

```python
from flask import Flask,jsonify
app = Flask(__name__)

@app.route("/")
def hello():
    return jsonify([
        {"from": "<bob@example.com>", "subject": "lunch at noon tomorrow"},
        {"from": "<alice@example.com>", "subject": "compiler docs"}])

if __name__ == "__main__":
    app.run(host='0.0.0.0')
```

**Frontend**

```python
from flask import Flask
import requests
from datetime import datetime
app = Flask(__name__)

@app.route("/")
def hello():
    weather = "weather unavailable"
    try:
        print "requesting weather..."
        start = datetime.now()
        r = requests.get('http://weather')
        print "got weather in %s ..." % (datetime.now() - start)
        if r.status_code == requests.codes.ok:
            weather = r.text
    except:
        print "weather unavailable"

    print "requesting mail..."
    r = requests.get('http://mail')
    mail = r.json()
    print "got mail in %s ..." % (datetime.now() - start)

    out = []
    for letter in mail:
        out.append("<li>From: %s Subject: %s</li>" % (letter['from'], letter['subject']))


    return '''<html>
<body>
  <h3>Weather</h3>
  <p>%s</p>
  <h3>Email</h3>
  <p>
    <ul>
      %s
    </ul>
  </p>
</body>
''' % (weather, '<br/>'.join(out))

if __name__ == "__main__":
    app.run(host='0.0.0.0')
```

Let's create our deployments and services:


```
$ cd prod/cbreaker
$ docker build -t $(minikube ip):5000/mail:0.0.1 .
$ docker push $(minikube ip):5000/mail:0.0.1
$ kubectl apply -f service.yaml
deployment "frontend" configured
deployment "weather" configured
deployment "mail" configured
service "frontend" configured
service "mail" configured
service "weather" configured
```

Check that everything is running smoothly:

```
$ kubectl run -i -t --rm cli --image=tutum/curl --restart=Never
$ curl http://frontend
<html>
<body>
  <h3>Weather</h3>
  <p>Pleasanton, CA
Saturday 8:00 PM
Partly Cloudy
12 C
Precipitation: 9%
Humidity: 74%
Wind: 14 km/h
</p>
  <h3>Email</h3>
  <p>
    <ul>
      <li>From: <bob@example.com> Subject: lunch at noon tomorrow</li><br/><li>From: <alice@example.com> Subject: compiler docs</li>
    </ul>
  </p>
</body>
```

Let's introduce weather service that crashes:

```python
from flask import Flask
app = Flask(__name__)

@app.route("/")
def hello():
    raise Exception("I am out of service")

if __name__ == "__main__":
    app.run(host='0.0.0.0')
```

Build and redeploy:

```
$ docker build -t $(minikube ip):5000/weather-crash:0.0.1 -f weather-crash.dockerfile .
$ docker push $(minikube ip):5000/weather-crash:0.0.1
$ kubectl apply -f weather-crash.yaml
deployment "weather" configured
```

Let's make sure that it is crashing:

```
$ kubectl run -i -t --rm cli --image=tutum/curl --restart=Never
$ curl http://weather
<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">
<title>500 Internal Server Error</title>
<h1>Internal Server Error</h1>
<p>The server encountered an internal error and was unable to complete your request.  Either the server is overloaded or there is an error in the application.</p>
```

However our frontend should be all good:

```
$ kubectl run -i -t --rm cli --image=tutum/curl --restart=Never
curl http://frontend
<html>
<body>
  <h3>Weather</h3>
  <p>weather unavailable</p>
  <h3>Email</h3>
  <p>
    <ul>
      <li>From: <bob@example.com> Subject: lunch at noon tomorrow</li><br/><li>From: <alice@example.com> Subject: compiler docs</li>
    </ul>
  </p>
</body>
root@cli:/# curl http://frontend
<html>
<body>
  <h3>Weather</h3>
  <p>weather unavailable</p>
  <h3>Email</h3>
  <p>
    <ul>
      <li>From: <bob@example.com> Subject: lunch at noon tomorrow</li><br/><li>From: <alice@example.com> Subject: compiler docs</li>
    </ul>
  </p>
</body>
```

Everything is working as expected! There is one problem though, we have just observed the service is crashing quickly, let's see what happens
if our weather service is slow. This happens way more often in production, e.g. due to network or database overload.

To simulate this failure we are going to introduce an artificial delay:

```python
from flask import Flask
import time

app = Flask(__name__)

@app.route("/")
def hello():
    time.sleep(30)
    raise Exception("System overloaded")

if __name__ == "__main__":
    app.run(host='0.0.0.0')
```

Build and redeploy:

```
$ docker build -t $(minikube ip):5000/weather-crash-slow:0.0.1 -f weather-crash-slow.dockerfile .
$ docker push $(minikube ip):5000/weather-crash-slow:0.0.1
$ kubectl apply -f weather-crash-slow.yaml
deployment "weather" configured
```

Just as expected, our weather service is timing out:

```
curl http://weather
<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2 Final//EN">
<title>500 Internal Server Error</title>
<h1>Internal Server Error</h1>
<p>The server encountered an internal error and was unable to complete your request.  Either the server is overloaded or there is an error in the application.</p>
```

The problem though, is that every request to frontend takes 10 seconds as well

```
curl http://frontend
```

This is a much more common outage - users leave in frustration as the service is unavailable.
To fix this issue we are going to introduce a special proxy with [circuit breaker](http://vulcand.github.io/proxy.html#circuit-breakers).

![standby](http://vulcand.github.io/_images/CircuitStandby.png)

Circuit breaker is a special middleware that is designed to provide a fail-over action in case the service has degraded. It is very helpful to prevent cascading failures - where the failure of the one service leads to failure of another. Circuit breaker observes requests statistics and checks the stats against a special error condition.


![tripped](http://vulcand.github.io/_images/CircuitTripped.png)

Here is our simple circuit breaker written in python:

```python
from flask import Flask
import requests
from datetime import datetime, timedelta
from threading import Lock
import logging, sys


app = Flask(__name__)

circuit_tripped_until = datetime.now()
mutex = Lock()

def trip():
    global circuit_tripped_until
    mutex.acquire()
    try:
        circuit_tripped_until = datetime.now() + timedelta(0,30)
        app.logger.info("circuit tripped until %s" %(circuit_tripped_until))
    finally:
        mutex.release()

def is_tripped():
    global circuit_tripped_until
    mutex.acquire()
    try:
        return datetime.now() < circuit_tripped_until
    finally:
        mutex.release()


@app.route("/")
def hello():
    weather = "weather unavailable"
    try:
        if is_tripped():
            return "circuit breaker: service unavailable (tripped)"

        r = requests.get('http://localhost:5000', timeout=1)
        app.logger.info("requesting weather...")
        start = datetime.now()
        app.logger.info("got weather in %s ..." % (datetime.now() - start))
        if r.status_code == requests.codes.ok:
            return r.text
        else:
            trip()
            return "circuit brekear: service unavailable (tripping 1)"
    except:
        app.logger.info("exception: %s", sys.exc_info()[0])
        trip()
        return "circuit brekear: service unavailable (tripping 2)"

if __name__ == "__main__":
    app.logger.addHandler(logging.StreamHandler(sys.stdout))
    app.logger.setLevel(logging.DEBUG)
    app.run(host='0.0.0.0', port=6000)
```

Let's build and redeploy circuit breaker:

```
$ docker build -t $(minikube ip):5000/cbreaker:0.0.1 -f cbreaker.dockerfile .
$ docker push $(minikube ip):5000/cbreaker:0.0.1
$ kubectl apply -f weather-cbreaker.yaml
deployment "weather" configured
$  kubectl apply -f weather-service.yaml
service "weather" configured
```


Circuit breaker will detect service outage and auxiliary weather service will not bring our mail service down any more:

```
$ curl http://frontend
<html>
<body>
  <h3>Weather</h3>
  <p>circuit breaker: service unavailable (tripped)</p>
  <h3>Email</h3>
  <p>
    <ul>
      <li>From: <bob@example.com> Subject: lunch at noon tomorrow</li><br/><li>From: <alice@example.com> Subject: compiler docs</li>
    </ul>
  </p>
</body>
```

**NOTE:** There are some production level proxies that natively support circuit breaker pattern - [Vulcand](http://vulcand.github.io/) or [Nginx plus](https://www.nginx.com/products/)


### Production Pattern: Sidecar For Rate and Connection Limiting

In the previous example we have used a sidecar pattern - a special proxy local to the Pod, that adds additional logic to the service, such as error detection, TLS termination
and other features.

Here is an example of sidecar nginx proxy that adds rate and connection limits:

```
$ cd prod/sidecar
$ docker build -t $(minikube ip):5000/sidecar:0.0.1 -f sidecar.dockerfile .
$ docker push $(minikube ip):5000/sidecar:0.0.1
$ docker build -t $(minikube ip):5000/service:0.0.1 -f service.dockerfile .
$ docker push $(minikube ip):5000/service:0.0.1
$ kubectl apply -f sidecar.yaml
deployment "sidecar" configured
```

Try to hit the service faster than one request per second and you will see the rate limiting in action

```
$ kubectl run -i -t --rm cli --image=tutum/curl --restart=Never
curl http://sidecar
```
