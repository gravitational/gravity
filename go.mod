module github.com/gravitational/gravity

go 1.13

require (
	cloud.google.com/go v0.38.0
	github.com/DATA-DOG/go-sqlmock v1.4.1 // indirect
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e // indirect
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.4.2 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible // indirect
	github.com/alecthomas/assert v0.0.0-20170929043011-405dbfeb8e38 // indirect
	github.com/alecthomas/colour v0.1.0 // indirect
	github.com/alecthomas/repr v0.0.0-20181024024818-d37bc2a10ba1 // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
	github.com/aokoli/goutils v1.0.1 // indirect
	github.com/apparentlymart/go-cidr v1.0.0 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/aws/aws-sdk-go v1.25.41
	github.com/beevik/etree v0.0.0-20170418002358-cda1c0026246 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/boltdb/bolt v1.3.1
	github.com/boombuler/barcode v0.0.0-20161226211916-fe0f26ff6d26 // indirect
	github.com/buger/goterm v0.0.0-20140416104154-af3f07dadc88
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/cloudflare/cfssl v0.0.0-20180726162950-56268a613adf
	github.com/cloudfoundry/gosigar v1.1.1-0.20180406153506-1375283248c3
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd
	github.com/containerd/continuity v0.0.0-20200107194136-26c1120b8d41 // indirect
	github.com/coreos/etcd v3.3.13+incompatible
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/prometheus-operator v0.0.0-00010101000000-000000000000 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.4.2-0.20190327010347-be7ac8be2ae0
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20150526203908-9cbd2a1374f4
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/emicklei/go-restful v2.11.0+incompatible // indirect
	github.com/fatih/color v1.7.0
	github.com/fsouza/go-dockerclient v1.0.0
	github.com/garyburd/redigo v0.0.0-20151029235527-6ece6e0a09f2 // indirect
	github.com/ghodss/yaml v1.0.1-0.20180820084758-c7ce16629ff4
	github.com/gizak/termui v2.3.0+incompatible
	github.com/go-ini/ini v1.30.0 // indirect
	github.com/go-openapi/runtime v0.19.4
	github.com/gobuffalo/packr v1.30.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/godbus/dbus v4.0.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.0
	github.com/gokyle/hotp v0.0.0-20160218004637-c180d57d286b
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/google/gops v0.3.8 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/gorilla/handlers v0.0.0-20151124211609-e96366d97736 // indirect
	github.com/gosimple/slug v1.9.0 // indirect
	github.com/gravitational/bandwagon v0.0.0-20200215230242-8a67c7595376 // indirect
	github.com/gravitational/configure v0.0.0-20191213111049-fce91dea0d0d
	github.com/gravitational/coordinate v0.0.0-20180225144834-2bc9a83f6fe2
	github.com/gravitational/form v0.0.0-20151109031454-c4048f792f70
	github.com/gravitational/go-vhost v0.0.0-20171024163855-94d0c42e3263
	github.com/gravitational/kingpin v2.1.11-0.20160205192003-785686550a08+incompatible // indirect
	github.com/gravitational/license v0.0.0-20171013193735-f3111b1818ce
	github.com/gravitational/oxy v0.0.0-20180629203109-e4a7e35311e6 // indirect
	github.com/gravitational/rigging v0.0.0-20191021212636-83b2e9505286
	github.com/gravitational/roundtrip v1.0.0
	github.com/gravitational/satellite v0.0.9-0.20200720191657-4877c91ae81f
	github.com/gravitational/tail v1.0.1
	github.com/gravitational/teleport v3.2.15-0.20200309221853-bebf7a500543+incompatible
	github.com/gravitational/trace v1.1.11
	github.com/gravitational/ttlmap v0.0.0-20171116003245-91fd36b9004c
	github.com/gravitational/version v0.0.2-0.20170324200323-95d33ece5ce1
	github.com/gravitational/vfsgen v0.0.0-20181202132449-6a9ea43bcacd
	github.com/hashicorp/go-getter v0.0.0-20180809191950-4bda8fa99001 // indirect
	github.com/hashicorp/go-hclog v0.0.0-20180828044259-75ecd6e6d645 // indirect
	github.com/hashicorp/go-plugin v0.0.0-20180814222501-a4620f9913d1 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/hcl2 v0.0.0-20180822193130-ed8144cda141 // indirect
	github.com/hashicorp/hil v0.0.0-20170627220502-fa9f258a9250 // indirect
	github.com/hashicorp/terraform v0.11.7 // indirect
	github.com/hashicorp/yamux v0.0.0-20180826203732-cc6d2ea263b2 // indirect
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/jonboulle/clockwork v0.2.0
	github.com/julienschmidt/httprouter v1.2.0
	github.com/kardianos/osext v0.0.0-20170510131534-ae77be60afb1
	github.com/konsorten/go-windows-terminal-sequences v1.0.3 // indirect
	github.com/kylelemons/godebug v0.0.0-20170820004349-d65d576e9348
	github.com/lib/pq v1.2.0 // indirect
	github.com/magefile/mage v1.10.0 // indirect
	github.com/mailgun/holster v1.9.1-0.20191129074427-2296d2fb30b1 // indirect
	github.com/mailgun/lemma v0.0.0-20160211003854-e8b0cd607f58
	github.com/mailgun/metrics v0.0.0-20150124003306-2b3c4565aafd // indirect
	github.com/mailgun/minheap v0.0.0-20131208021033-7c28d80e2ada // indirect
	github.com/mailgun/timetools v0.0.0-20150505213551-fd192d755b00
	github.com/mailgun/ttlmap v0.0.0-20150816203249-16b258d86efc // indirect
	github.com/maruel/panicparse v1.1.2-0.20180806203336-f20d4c4d746f // indirect
	github.com/mdp/rsc v0.0.0-20160131164516-90f07065088d // indirect
	github.com/miekg/dns v1.1.26
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/go-ps v1.0.0
	github.com/mitchellh/hashstructure v1.0.0 // indirect
	github.com/mreiferson/go-httpclient v0.0.0-20160630210159-31f0106b4474 // indirect
	github.com/nsf/termbox-go v0.0.0-20190325093121-288510b9734e // indirect
	github.com/olekukonko/tablewriter v0.0.1
	github.com/onsi/ginkgo v1.11.0 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/selinux v1.4.0
	github.com/pborman/uuid v1.2.0
	github.com/petar/GoLLRB v0.0.0-20130427215148-53be0d36a84c // indirect
	github.com/posener/complete v1.1.2 // indirect
	github.com/pquerna/otp v0.0.0-20160912161815-54653902c20e // indirect
	github.com/prometheus/alertmanager v0.20.0
	github.com/prometheus/client_golang v1.2.1
	github.com/prometheus/common v0.6.0
	github.com/rubenv/sql-migrate v0.0.0-20190902133344-8926f37f0bc1 // indirect
	github.com/russellhaering/gosaml2 v0.0.0-20170515204909-8908227c114a // indirect
	github.com/russellhaering/goxmldsig v0.0.0-20170515183101-605161228693 // indirect
	github.com/santhosh-tekuri/jsonschema v1.2.2
	github.com/shurcooL/sanitized_anchor_name v0.0.0-20170918181015-86672fcb3f95 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/stevvooe/resumable v0.0.0-20150521211217-51ad44105773 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/tstranex/u2f v0.0.0-20160508205855-eb799ce68da4
	github.com/ugorji/go v1.1.7 // indirect
	github.com/ulikunitz/xz v0.5.4 // indirect
	github.com/vulcand/oxy v0.0.0-20160623194703-40720199a16c
	github.com/vulcand/predicate v1.1.0
	github.com/xeipuuv/gojsonpointer v0.0.0-20151027082146-e0fe6f683076 // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20150808065054-e02fc20de94c // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20151204154511-3988ac14d6f6 // indirect
	github.com/xtgo/set v1.0.0
	github.com/zclconf/go-cty v0.0.0-20180829180805-c2393a5d54f2 // indirect
	github.com/ziutek/mymysql v1.5.4 // indirect
	go.etcd.io/bbolt v1.3.5 // indirect
	golang.org/x/crypto v0.0.0-20200709230013-948cd5f35899
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae
	gonum.org/v1/gonum v0.6.1 // indirect
	google.golang.org/grpc v1.26.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/gorp.v1 v1.7.2 // indirect
	gopkg.in/mgo.v2 v2.0.0-20160818020120-3f83fa500528 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver v0.0.0
	k8s.io/apimachinery v0.17.6
	k8s.io/client-go v2.0.0-alpha.0.0.20181121191925-a47917edff34+incompatible
	k8s.io/gengo v0.0.0-20191120174120-e74f70b9b27e // indirect
	k8s.io/helm v2.15.2+incompatible
	k8s.io/kube-aggregator v0.0.0
	k8s.io/kubectl v0.17.6
	k8s.io/kubernetes v1.15.7
	k8s.io/utils v0.0.0-20200324210504-a9aa75ae1b89 // indirect
	launchpad.net/gocheck v0.0.0-20140225173054-000000000087 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
	vbom.ml/util v0.0.0-20180919145318-efcd4e0f9787 // indirect
)

replace (
	cloud.google.com/go => cloud.google.com/go v0.21.0
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v11.1.0+incompatible
	github.com/Microsoft/go-winio => github.com/Microsoft/go-winio v0.4.9
	github.com/PuerkitoBio/purell => github.com/PuerkitoBio/purell v1.1.0
	github.com/alecthomas/template => github.com/alecthomas/template v0.0.0-20150530000104-b867cc6ab45c
	github.com/apparentlymart/go-textseg => github.com/apparentlymart/go-textseg v1.0.0
	github.com/armon/go-metrics => github.com/armon/go-metrics v0.0.0-20190430140413-ec5e00d3c878
	github.com/asaskevich/govalidator => github.com/asaskevich/govalidator v0.0.0-20180315120708-ccb8e960c48f
	github.com/aws/aws-sdk-go => github.com/aws/aws-sdk-go v1.14.25
	github.com/beorn7/perks => github.com/beorn7/perks v0.0.0-20150223135152-b965b613227f
	github.com/boltdb/bolt => github.com/gravitational/bolt v1.3.2-gravitational
	github.com/cloudflare/cfssl => github.com/gravitational/cfssl v0.0.0-20180619163912-4b8305b36ad0
	github.com/coreos/etcd => github.com/coreos/etcd v3.3.10+incompatible
	github.com/coreos/go-oidc => github.com/gravitational/go-oidc v0.0.1
	github.com/coreos/go-semver => github.com/coreos/go-semver v0.2.0
	github.com/coreos/go-systemd => github.com/coreos/go-systemd v0.0.0-20180202092358-40e2722dffea
	github.com/coreos/prometheus-operator => github.com/coreos/prometheus-operator v0.32.0
	github.com/davecgh/go-spew => github.com/davecgh/go-spew v1.1.0
	github.com/docker/docker => github.com/gravitational/moby v1.4.2-0.20191008111026-2adf434ca696
	github.com/docker/go-units => github.com/docker/go-units v0.3.1
	github.com/dustin/go-humanize => github.com/dustin/go-humanize v0.0.0-20150809201405-1c212aae1d02
	github.com/evanphx/json-patch => github.com/evanphx/json-patch v3.0.0+incompatible
	github.com/fsouza/go-dockerclient => github.com/fsouza/go-dockerclient v0.0.0-20160213172103-9184fe7a2d7c
	github.com/go-openapi/runtime => github.com/go-openapi/runtime v0.19.3
	github.com/go-openapi/strfmt => github.com/go-openapi/strfmt v0.19.2
	github.com/go-openapi/swag => github.com/go-openapi/swag v0.18.0
	github.com/golang/groupcache => github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff
	github.com/google/certificate-transparency-go => github.com/gravitational/certificate-transparency-go v0.0.0-20180803094710-99d8352410cb
	github.com/google/gofuzz => github.com/google/gofuzz v0.0.0-20150304233714-bbcb9da2d746
	github.com/google/uuid => github.com/google/uuid v1.1.0
	github.com/gophercloud/gophercloud => github.com/gophercloud/gophercloud v0.0.0-20181207171349-d3bcea3cf97e
	github.com/gorilla/mux => github.com/gorilla/mux v1.7.0
	github.com/hashicorp/go-cleanhttp => github.com/hashicorp/go-cleanhttp v0.5.0
	github.com/hashicorp/go-uuid => github.com/hashicorp/go-uuid v1.0.0
	github.com/hashicorp/go-version => github.com/hashicorp/go-version v1.0.0
	github.com/hashicorp/golang-lru => github.com/hashicorp/golang-lru v0.5.0
	github.com/hashicorp/hcl => github.com/hashicorp/hcl v1.0.0
	github.com/hashicorp/memberlist => github.com/hashicorp/memberlist v0.1.4
	github.com/imdario/mergo => github.com/imdario/mergo v0.0.0-20160517064435-50d4dbd4eb0e
	github.com/json-iterator/go => github.com/json-iterator/go v1.1.5
	github.com/julienschmidt/httprouter => github.com/julienschmidt/httprouter v1.1.0
	github.com/kr/pty => github.com/kr/pty v1.0.0
	github.com/kylelemons/godebug => github.com/kylelemons/godebug v0.0.0-20160406211939-eadb3ce320cb
	github.com/mattn/go-isatty => github.com/mattn/go-isatty v0.0.3
	github.com/mattn/go-runewidth => github.com/mattn/go-runewidth v0.0.2-0.20161012013512-737072b4e32b
	github.com/matttproud/golang_protobuf_extensions => github.com/matttproud/golang_protobuf_extensions v0.0.0-20151011102529-d0c3fe89de86
	github.com/miekg/dns => github.com/miekg/dns v1.0.4
	github.com/mitchellh/go-homedir => github.com/mitchellh/go-homedir v1.0.0
	github.com/mitchellh/mapstructure => github.com/mitchellh/mapstructure v1.0.0
	github.com/modern-go/reflect2 => github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742
	github.com/olekukonko/tablewriter => github.com/olekukonko/tablewriter v0.0.0-20160923125401-bdcc175572fd
	github.com/pborman/uuid => github.com/pborman/uuid v0.0.0-20170612153648-e790cca94e6c
	github.com/prometheus/alertmanager => github.com/prometheus/alertmanager v0.18.0
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/client_model => github.com/prometheus/client_model v0.0.0-20170216185247-6f3806018612
	github.com/prometheus/common => github.com/prometheus/common v0.0.0-20170731114204-61f87aac8082
	github.com/prometheus/procfs => github.com/prometheus/procfs v0.0.5
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
	gopkg.in/alecthomas/kingpin.v2 => github.com/gravitational/kingpin v2.1.11-0.20180808090833-85085db9f49b+incompatible
	gopkg.in/inf.v0 => gopkg.in/inf.v0 v0.9.0
	gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.2
	k8s.io/api => k8s.io/api v0.15.7
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.15.7
	k8s.io/apimachinery => k8s.io/apimachinery v0.15.7
	k8s.io/apiserver => k8s.io/apiserver v0.15.7
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.15.7
	k8s.io/client-go => k8s.io/client-go v0.15.7
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.15.7
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.15.7
	k8s.io/code-generator => k8s.io/code-generator v0.15.7
	k8s.io/component-base => k8s.io/component-base v0.15.7
	k8s.io/cri-api => k8s.io/cri-api v0.15.7
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.15.7
	k8s.io/klog => k8s.io/klog v0.1.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.15.7
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.15.7
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.15.7
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.15.7
	k8s.io/kubectl => k8s.io/kubectl v0.17.6
	k8s.io/kubelet => k8s.io/kubelet v0.15.7
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.15.7
	k8s.io/metrics => k8s.io/metrics v0.15.7
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.15.7
)
