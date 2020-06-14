# Introduction

This is an example TensorFlow ResNet (Deep Residual Network) server helm chart that serves machine learning (ML) model.  The default model is 
for image classification.  The model is pulled from a public URL specified in the [values.yml](./resources/charts/tensorflow-resnet/values.yml)
and can be configured to other models.  See the [TensorFlow](https://www.tensorflow.org/) website for more information on using ResNet models.

Example input
![Ostrich](./sampleImages/ostrich.jpg)

Example output
```bash
Welcome to the Bitnami tensorflow-resnet container
Subscribe to project updates by watching https://github.com/bitnami/bitnami-docker-tensorflow-resnet
Submit issues and feature requests at https://github.com/bitnami/bitnami-docker-tensorflow-resnet/issues

calling predict using file: /sampleImages/violin.jpg  ...
call predict ok
outputs size is 2
the result tensor[0] is:
[1.8476394e-10 1.69398287e-10 2.69399197e-10 1.16475674e-09 7.47527318e-10 1.34114841e-09 2.16753637e-08 1.44472723e-09 1.98581138e-10 2.0557922e-10...]...
the result tensor[1] is:
890
Done.

# Tensor classification retrieval example
Tensor result id retrieve: 890
Classification name: violin

```

## Building Cluster Image
To construct the Tensorflow-ResNet Cluster Image with a dependency-free .tar file use this command.  You can then deploy the Tensorflow-ResNet as a self-contained, truely portable application for your preferred infrastructure. 
```bash
tele build -o tensorflowresnet.tar tensorflow-resnet/resources/app.yaml
```

Further details on installing Gravity Cluster Images is available [here](https://gravitational.com/gravity/docs/installation/). 


## Building Application
In addition to Cluster Images, Gravity supports packaging application helm charts as self-contained application images. The application is then deployable in the same manner as helm charts to Gravity clusters. Further information on application packaging deployment is available [here](https://gravitational.com/gravity/docs/catalog/).
```bash
tele build -o tensorflowresnet.tar tensorflow-resnet/resources/charts/tensorflow-resnet
```

## Default
The Tensorflow-ResNet application runs in the default configuration with two ports, 8500, 8501.  The 8500 port is used for client image classification requests and configured available on a NodePort of 30090 by default.  

After deploying you will see the following Pod status of initalizing while the model is loaded.

```bash
$ kubectl get po
NAME                                READY   STATUS            RESTARTS   AGE                                                                                                                                                              
tensorflow-resnet-bf8f6f5b6-22pgd   0/1     PodInitializing   0          7s                                                                                                                                                               
tensorflow-resnet-bf8f6f5b6-59fq8   0/1     PodInitializing   0          7s                                                                                                                                                               
tensorflow-resnet-bf8f6f5b6-qbd6v   0/1     PodInitializing   0          7s    
```

Once the model is loaded these pods are available to serve image classifications.
```bash
kubectl get po                                                                                                                                                                                         
NAME                                READY   STATUS    RESTARTS   AGE                                                                                                                                                                      
tensorflow-resnet-bf8f6f5b6-22pgd   1/1     Running   0          37s                                                                                                                                                                      
tensorflow-resnet-bf8f6f5b6-59fq8   1/1     Running   0          37s                                                                                                                                                                      
tensorflow-resnet-bf8f6f5b6-qbd6v   1/1     Running   0          37s
```
```bash
$ kubectl get deployments
NAME                READY   UP-TO-DATE   AVAILABLE   AGE
tensorflow-resnet   3/3     3            3           10m
```

# Image classification

Image classification requests should be done on a machine with Docker installed. Take note of one of the server's available IPs and confirm the available NodePort.  By default it should be 30090. After invoking you will recieve a result tensor.  
The [./tensorClientResources/class.json](./tensorClientResources/class.json) has an array with the names of the results. Note that the classification in the JSON file is one number less then the result.  For example if you gave an image of a hammerhead shark you would
receive the number 5 tensor result which is number 4 below.

```json
{  
  "4": [
    "n01494475",
    "hammerhead"
  ],
  "5": [
    "n01496331",
    "electric_ray"
  ]
  }
```

## Invoking classificaton

Two example invocations are.  Invoking with a public URL or 





## Get Classification Name
