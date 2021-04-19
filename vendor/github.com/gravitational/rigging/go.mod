module github.com/gravitational/rigging

go 1.12

require (
	github.com/coreos/prometheus-operator v0.0.0-00010101000000-000000000000
	github.com/davecgh/go-spew v1.1.1
	github.com/ghodss/yaml v1.0.0
	github.com/gravitational/trace v1.1.6-0.20180717152918-4a5e142f3251
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/kylelemons/godebug v0.0.0-20170820004349-d65d576e9348
	github.com/sirupsen/logrus v1.6.0
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	k8s.io/api v0.19.8
	k8s.io/apiextensions-apiserver v0.19.8
	k8s.io/apimachinery v0.19.8
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-aggregator v0.19.8
)

replace (
	github.com/coreos/prometheus-operator => github.com/gravitational/prometheus-operator v0.40.1
	github.com/sirupsen/logrus => github.com/gravitational/logrus v0.10.1-0.20180402202453-dcdb95d728db
	k8s.io/client-go => k8s.io/client-go v0.19.8
)
