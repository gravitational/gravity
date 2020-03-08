# Glossary

## [Application Bundle or Bundle](index.md#application-bundle)

A snapshot of a Kubernetes cluster, stored as a compressed `.tar` file. Application Bundles may contain nothing but pre-packaged Kubernetes for centralized management of Kubernetes resources within an organization or may also contain other application services running on Kubernetes.

An Application Bundle is a fully operational, autonomous, self-regulating
and self-updating Kubernetes cluster. All Application Bundles can optionally "phone
home" to a centralized ops center (control plane) and be remotely managed.

## [Application Manifest](index.md#application-manifest)

A YAML file which describes how an Application Bundle should be installed. The
Applicaion Manifest's purpose is to describe the infrastructure requirements and custom steps required for installing and updating the Application Bundle.

## [Ops Center](opscenter.md)

A centralized repository of your Application Bundles and its deployed instances. If an
Application Bundle is distributed via the Ops Center, it can optionally "phone home" for automating updates, remote monitoring and trouble shooting.

## [Gravity Cluster or Cluster](#)
)

## [Gravity](#)

## [tele](#)

The Gravity CLI client used for packaging applications and publishing them into the Ops Center.

## [tsh](#)

Gravity SSH client used to remotely connect to a node inside of any Gravity cluster. tsh is fully compatible with OpenSSH's ssh.

## [gravity](#)

Gravity component for managing Kubernetes. It manages Kubernetes daemons and their health, cluster updates and so on.

## [Application Resources](#)

Applications Resources include the Application Manifest and all other files that were in the same directory as the Application Manifest during the Application Bundle build process.

## [Application Hooks](pack.md#application-hooks)

Application Hooks are Kubernetes' jobs that run at different points in the application lifecycle or in
response to certain events happening in the cluster.

## [Custom Installation Screen](pack.md#custom-installation-screen)

## [Bandwagan](pack.md#custom-installation-screen)

A sample Custom Installation Screen packaged with Gravity that allows a user to create credentials and enable or disable remote access to an Application Cluster.

## [Operation Plan](cluster.md#updating-a-cluster)

## [Cluster Spec](#)

Provides the infrastructure resources that satisfy the requirements
defined by the application bundle. Remember that in case of a [manual installation](quickstart.md#installing-the-application)
of an application bundle the user is responsible to provide the same information manually
to create a cluster.
