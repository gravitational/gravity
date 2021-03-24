module github.com/gravitational/gravity

go 1.13

require (
	cloud.google.com/go v0.56.0
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e // indirect
	github.com/alecthomas/assert v0.0.0-20170929043011-405dbfeb8e38 // indirect
	github.com/alecthomas/colour v0.1.0 // indirect
	github.com/alecthomas/repr v0.0.0-20181024024818-d37bc2a10ba1 // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/aws/aws-sdk-go v1.37.15
	github.com/boltdb/bolt v1.3.1
	github.com/boombuler/barcode v0.0.0-20161226211916-fe0f26ff6d26 // indirect
	github.com/buger/goterm v0.0.0-20140416104154-af3f07dadc88
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cloudflare/cfssl v0.0.0-20180726162950-56268a613adf
	github.com/cloudfoundry/gosigar v1.1.1-0.20180406153506-1375283248c3
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd
	github.com/coreos/etcd v3.3.25+incompatible
	github.com/coreos/go-semver v0.3.0
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.3-0.20210216175712-646072ed6524+incompatible
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20150526203908-9cbd2a1374f4
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/emicklei/go-restful v2.11.0+incompatible // indirect
	github.com/fatih/color v1.9.0
	github.com/fsouza/go-dockerclient v1.7.2
	github.com/garyburd/redigo v0.0.0-20151029235527-6ece6e0a09f2 // indirect
	github.com/ghodss/yaml v1.0.1-0.20180820084758-c7ce16629ff4
	github.com/gizak/termui v2.3.0+incompatible
	github.com/go-ini/ini v1.30.0 // indirect
	github.com/go-openapi/runtime v0.19.15
	github.com/gofrs/flock v0.8.0
	github.com/gogo/protobuf v1.3.2
	github.com/gokyle/hotp v0.0.0-20160218004637-c180d57d286b
	github.com/google/certificate-transparency-go v0.0.0-00010101000000-000000000000 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/gorilla/handlers v0.0.0-20151124211609-e96366d97736 // indirect
	github.com/gravitational/configure v0.0.0-20191213111049-fce91dea0d0d
	github.com/gravitational/coordinate v0.0.0-20180225144834-2bc9a83f6fe2
	github.com/gravitational/form v0.0.0-20151109031454-c4048f792f70
	github.com/gravitational/go-vhost v0.0.0-20171024163855-94d0c42e3263
	github.com/gravitational/kingpin v2.1.11-0.20160205192003-785686550a08+incompatible // indirect
	github.com/gravitational/license v0.0.0-20171013193735-f3111b1818ce
	github.com/gravitational/oxy v0.0.0-20180629203109-e4a7e35311e6 // indirect
	github.com/gravitational/rigging v0.0.0-20210315200250-d036093ec3e6
	github.com/gravitational/roundtrip v1.0.0
	github.com/gravitational/satellite v0.0.9-0.20210313001929-72cbba4dc99c
	github.com/gravitational/tail v1.0.1
	github.com/gravitational/teleport v3.2.17+incompatible
	github.com/gravitational/trace v1.1.14
	github.com/gravitational/ttlmap v0.0.0-20171116003245-91fd36b9004c
	github.com/gravitational/version v0.0.2-0.20170324200323-95d33ece5ce1
	github.com/gravitational/vfsgen v0.0.0-20181202132449-6a9ea43bcacd
	github.com/jonboulle/clockwork v0.2.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kardianos/osext v0.0.0-20170510131534-ae77be60afb1 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.3 // indirect
	github.com/kylelemons/godebug v0.0.0-20170820004349-d65d576e9348
	github.com/mailgun/lemma v0.0.0-20160211003854-e8b0cd607f58
	github.com/mailgun/metrics v0.0.0-20150124003306-2b3c4565aafd // indirect
	github.com/mailgun/minheap v0.0.0-20131208021033-7c28d80e2ada // indirect
	github.com/mailgun/timetools v0.0.0-20150505213551-fd192d755b00
	github.com/mailgun/ttlmap v0.0.0-20150816203249-16b258d86efc // indirect
	github.com/maruel/panicparse v1.1.2-0.20180806203336-f20d4c4d746f // indirect
	github.com/mdp/rsc v0.0.0-20160131164516-90f07065088d // indirect
	github.com/miekg/dns v1.1.29
	github.com/mitchellh/go-ps v1.0.0
	github.com/mreiferson/go-httpclient v0.0.0-20160630210159-31f0106b4474 // indirect
	github.com/nsf/termbox-go v0.0.0-20190325093121-288510b9734e // indirect
	github.com/olekukonko/tablewriter v0.0.2
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/selinux v1.5.2
	github.com/pborman/uuid v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/pquerna/otp v0.0.0-20160912161815-54653902c20e // indirect
	github.com/prometheus/alertmanager v0.20.0
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/common v0.10.0
	github.com/russellhaering/gosaml2 v0.0.0-20170515204909-8908227c114a // indirect
	github.com/russellhaering/goxmldsig v1.1.0 // indirect
	github.com/santhosh-tekuri/jsonschema v1.2.4
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1
	github.com/tstranex/u2f v0.0.0-20160508205855-eb799ce68da4
	github.com/vulcand/oxy v0.0.0-20160623194703-40720199a16c
	github.com/vulcand/predicate v1.1.0
	github.com/xtgo/set v1.0.0
	golang.org/x/crypto v0.0.0-20200728195943-123391ffb6de
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	golang.org/x/sys v0.0.0-20210216224549-f992740a1bac
	google.golang.org/grpc v1.29.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/mgo.v2 v2.0.0-20160818020120-3f83fa500528 // indirect
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0 // indirect
	gopkg.in/yaml.v2 v2.3.0
	helm.sh/helm/v3 v3.4.2
	k8s.io/api v0.19.8
	k8s.io/apiextensions-apiserver v0.19.8
	k8s.io/apimachinery v0.19.8
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-aggregator v0.19.8
	k8s.io/kubectl v0.19.8
	k8s.io/kubernetes v1.19.8
	launchpad.net/gocheck v0.0.0-20140225173054-000000000087 // indirect
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace (
	cloud.google.com/go => cloud.google.com/go v0.21.0
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.0.1-0.20190815170712-85d9c035382e+incompatible
	github.com/PuerkitoBio/purell => github.com/PuerkitoBio/purell v1.1.0
	github.com/alecthomas/template => github.com/alecthomas/template v0.0.0-20150530000104-b867cc6ab45c
	github.com/apparentlymart/go-textseg => github.com/apparentlymart/go-textseg v1.0.0
	github.com/armon/go-metrics => github.com/armon/go-metrics v0.0.0-20190430140413-ec5e00d3c878
	github.com/asaskevich/govalidator => github.com/asaskevich/govalidator v0.0.0-20180315120708-ccb8e960c48f
	github.com/aws/aws-sdk-go => github.com/aws/aws-sdk-go v1.14.25
	github.com/beorn7/perks => github.com/beorn7/perks v0.0.0-20150223135152-b965b613227f
	github.com/boltdb/bolt => github.com/gravitational/bolt v1.3.2-gravitational
	github.com/cloudflare/cfssl => github.com/gravitational/cfssl v0.0.0-20180619163912-4b8305b36ad0
	github.com/coreos/etcd => github.com/coreos/etcd v3.3.25+incompatible
	github.com/coreos/go-oidc => github.com/gravitational/go-oidc v0.0.1
	github.com/coreos/go-semver => github.com/coreos/go-semver v0.2.0
	github.com/coreos/go-systemd => github.com/coreos/go-systemd v0.0.0-20180202092358-40e2722dffea
	github.com/coreos/prometheus-operator => github.com/gravitational/prometheus-operator v0.40.1
	github.com/davecgh/go-spew => github.com/davecgh/go-spew v1.1.0
	github.com/docker/docker => github.com/gravitational/moby v1.4.2-0.20191008111026-2adf434ca696
	github.com/docker/go-units => github.com/docker/go-units v0.3.1
	github.com/dustin/go-humanize => github.com/dustin/go-humanize v0.0.0-20150809201405-1c212aae1d02
	github.com/evanphx/json-patch => github.com/evanphx/json-patch v3.0.0+incompatible
	github.com/go-openapi/runtime => github.com/go-openapi/runtime v0.19.3
	github.com/go-openapi/strfmt => github.com/go-openapi/strfmt v0.19.2
	github.com/go-openapi/swag => github.com/go-openapi/swag v0.18.0
	github.com/golang/groupcache => github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff
	github.com/google/certificate-transparency-go => github.com/gravitational/certificate-transparency-go v0.0.0-20180803094710-99d8352410cb
	github.com/google/gofuzz => github.com/google/gofuzz v0.0.0-20150304233714-bbcb9da2d746
	github.com/google/uuid => github.com/google/uuid v1.1.0
	github.com/gophercloud/gophercloud => github.com/gophercloud/gophercloud v0.0.0-20181207171349-d3bcea3cf97e
	github.com/gorilla/mux => github.com/gorilla/mux v1.7.0
	github.com/imdario/mergo => github.com/imdario/mergo v0.0.0-20160517064435-50d4dbd4eb0e
	github.com/json-iterator/go => github.com/json-iterator/go v1.1.5
	github.com/julienschmidt/httprouter => github.com/julienschmidt/httprouter v1.1.0
	github.com/kr/pty => github.com/kr/pty v1.0.0
	github.com/kylelemons/godebug => github.com/kylelemons/godebug v0.0.0-20160406211939-eadb3ce320cb
	github.com/mattn/go-isatty => github.com/mattn/go-isatty v0.0.3
	github.com/mattn/go-runewidth => github.com/mattn/go-runewidth v0.0.2-0.20161012013512-737072b4e32b
	github.com/miekg/dns => github.com/miekg/dns v1.0.4
	github.com/mitchellh/go-homedir => github.com/mitchellh/go-homedir v1.0.0
	github.com/mitchellh/mapstructure => github.com/mitchellh/mapstructure v1.0.0
	github.com/modern-go/reflect2 => github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742
	github.com/olekukonko/tablewriter => github.com/olekukonko/tablewriter v0.0.0-20160923125401-bdcc175572fd
	github.com/pborman/uuid => github.com/pborman/uuid v0.0.0-20170612153648-e790cca94e6c
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	github.com/russross/blackfriday => github.com/russross/blackfriday v0.0.0-20151117072312-300106c228d5
	github.com/satori/go.uuid => github.com/satori/go.uuid v1.1.1-0.20170321230731-5bf94b69c6b6
	github.com/sirupsen/logrus => github.com/gravitational/logrus v1.4.3
	github.com/spf13/cobra => github.com/spf13/cobra v0.0.3
	github.com/stretchr/testify => github.com/stretchr/testify v1.2.2
	github.com/ugorji/go => github.com/ugorji/go v1.1.1
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20181025213731-e84da0312774
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20181003184128-c57b0facaced
	golang.org/x/sys => golang.org/x/sys v0.0.0-20181026203630-95b1ffbd15a5
	golang.org/x/text => golang.org/x/text v0.0.0-20161230201740-fd889fe3a20f
	golang.org/x/time => golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2
	google.golang.org/appengine => google.golang.org/appengine v1.2.0
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20170818010345-ee236bd376b0
	google.golang.org/grpc => google.golang.org/grpc v1.26.0
	gopkg.in/alecthomas/kingpin.v2 => github.com/gravitational/kingpin v2.1.11-0.20180808090833-85085db9f49b+incompatible
	gopkg.in/inf.v0 => gopkg.in/inf.v0 v0.9.0
	gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.2
	k8s.io/api => k8s.io/api v0.19.8
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.8
	k8s.io/apiserver => k8s.io/apiserver v0.19.8
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.8
	k8s.io/client-go => k8s.io/client-go v0.19.8
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.8
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.8
	k8s.io/code-generator => k8s.io/code-generator v0.19.8
	k8s.io/component-base => k8s.io/component-base v0.19.8
	k8s.io/cri-api => k8s.io/cri-api v0.19.8
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.8
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.8
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.8
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.8
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.8
	k8s.io/kubectl => k8s.io/kubectl v0.19.8
	k8s.io/kubelet => k8s.io/kubelet v0.19.8
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.8
	k8s.io/metrics => k8s.io/metrics v0.19.8
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.8
)
