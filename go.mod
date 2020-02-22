module github.com/gravitational/gravity

go 1.12

require (
	cloud.google.com/go v0.34.0
	github.com/DATA-DOG/go-sqlmock v1.4.1 // indirect
	github.com/MakeNowJust/heredoc v0.0.0-20171113091838-e9091a26100e // indirect
	github.com/Masterminds/semver v1.4.2 // indirect
	github.com/Masterminds/sprig v2.16.0+incompatible // indirect
	github.com/alecthomas/assert v0.0.0-20170929043011-405dbfeb8e38 // indirect
	github.com/alecthomas/colour v0.1.0 // indirect
	github.com/alecthomas/repr v0.0.0-20181024024818-d37bc2a10ba1 // indirect
	github.com/alecthomas/template v0.0.0-20160405071501-a0175ee3bccc
	github.com/aokoli/goutils v1.0.1 // indirect
	github.com/apparentlymart/go-cidr v1.0.0 // indirect
	github.com/armon/go-metrics v0.0.0-20190430140413-ec5e00d3c878 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/aws/aws-sdk-go v1.16.26
	github.com/beevik/etree v0.0.0-20170418002358-cda1c0026246 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/boltdb/bolt v1.3.1
	github.com/boombuler/barcode v0.0.0-20161226211916-fe0f26ff6d26 // indirect
	github.com/buger/goterm v0.0.0-20140416104154-af3f07dadc88
	github.com/cenkalti/backoff v1.1.0
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/cloudflare/cfssl v0.0.0-20180726162950-56268a613adf
	github.com/cloudfoundry/gosigar v0.0.0-20170815193638-f4030c18ce1a
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd
	github.com/containerd/continuity v0.0.0-20200107194136-26c1120b8d41 // indirect
	github.com/coreos/etcd v3.3.13+incompatible
	github.com/coreos/go-semver v0.2.0
	github.com/coreos/prometheus-operator v0.29.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/docker/distribution v0.0.0-20170726174610-edc3ab29cdff
	github.com/docker/docker v0.7.3-0.20190327010347-be7ac8be2ae0
	github.com/docker/libtrust v0.0.0-20150526203908-9cbd2a1374f4
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/emicklei/go-restful v2.11.0+incompatible // indirect
	github.com/evanphx/json-patch v4.1.0+incompatible // indirect
	github.com/fatih/color v1.7.0
	github.com/fsouza/go-dockerclient v1.0.0
	github.com/garyburd/redigo v0.0.0-20151029235527-6ece6e0a09f2 // indirect
	github.com/ghodss/yaml v0.0.0-20180820084758-c7ce16629ff4
	github.com/gizak/termui v2.3.0+incompatible
	github.com/go-openapi/analysis v0.19.4 // indirect
	github.com/go-openapi/runtime v0.19.3
	github.com/go-openapi/strfmt v0.19.2 // indirect
	github.com/go-sql-driver/mysql v1.4.1 // indirect
	github.com/gobuffalo/packr v1.30.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/godbus/dbus v4.0.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.1
	github.com/gokyle/hotp v0.0.0-20160218004637-c180d57d286b
	github.com/golang/groupcache v0.0.0-20181024230925-c65c006176ff // indirect
	github.com/golang/protobuf v1.3.1
	github.com/google/btree v1.0.0 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/googleapis/gnostic v0.1.0 // indirect
	github.com/gophercloud/gophercloud v0.0.0-20190301152420-fca40860790e // indirect
	github.com/gorilla/handlers v0.0.0-20151124211609-e96366d97736 // indirect
	github.com/gravitational/configure v0.0.0-20180808141939-c3428bd84c23
	github.com/gravitational/coordinate v0.0.0-20180225144834-2bc9a83f6fe2
	github.com/gravitational/form v0.0.0-20151109031454-c4048f792f70
	github.com/gravitational/go-vhost v0.0.0-20171024163855-94d0c42e3263
	github.com/gravitational/kingpin v0.0.0-20160205192003-785686550a08 // indirect
	github.com/gravitational/license v0.0.0-20171013193735-f3111b1818ce
	github.com/gravitational/log v0.0.0-20200127200505-fdffa14162b0 // indirect
	github.com/gravitational/oxy v0.0.0-20180629203109-e4a7e35311e6 // indirect
	github.com/gravitational/rigging v0.0.0-20191021212636-83b2e9505286
	github.com/gravitational/roundtrip v1.0.0
	github.com/gravitational/satellite v0.0.0-20200205225625-d2c1e14945c2
	github.com/gravitational/tail v1.0.1
	github.com/gravitational/teleport v0.0.0-20200110233851-f4445fa60013
	github.com/gravitational/trace v0.0.0-20200129130229-dd5b2e8eae86
	github.com/gravitational/ttlmap v0.0.0-20171116003245-91fd36b9004c
	github.com/gravitational/version v0.0.0-20160322215120-ffae935cafba
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v0.0.0-20181025070259-68e3a13e4117 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.7.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.1 // indirect
	github.com/hashicorp/go-getter v0.0.0-20180809191950-4bda8fa99001 // indirect
	github.com/hashicorp/go-hclog v0.0.0-20180828044259-75ecd6e6d645 // indirect
	github.com/hashicorp/go-immutable-radix v1.1.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-plugin v0.0.0-20180814222501-a4620f9913d1 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/go-version v1.1.0 // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/hashicorp/hcl2 v0.0.0-20180822193130-ed8144cda141 // indirect
	github.com/hashicorp/hil v0.0.0-20170627220502-fa9f258a9250 // indirect
	github.com/hashicorp/memberlist v0.1.4 // indirect
	github.com/hashicorp/serf v0.8.3 // indirect
	github.com/hashicorp/terraform v0.11.7
	github.com/hashicorp/yamux v0.0.0-20180826203732-cc6d2ea263b2 // indirect
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/jmoiron/sqlx v1.2.0 // indirect
	github.com/jonboulle/clockwork v0.0.0-20190114141812-62fb9bc030d1
	github.com/julienschmidt/httprouter v1.2.0
	github.com/kardianos/osext v0.0.0-20170510131534-ae77be60afb1
	github.com/kylelemons/godebug v0.0.0-20170820004349-d65d576e9348
	github.com/lib/pq v1.2.0 // indirect
	github.com/mailgun/lemma v0.0.0-20160211003854-e8b0cd607f58
	github.com/mailgun/metrics v0.0.0-20150124003306-2b3c4565aafd // indirect
	github.com/mailgun/minheap v0.0.0-20131208021033-7c28d80e2ada // indirect
	github.com/mailgun/timetools v0.0.0-20150505213551-fd192d755b00
	github.com/mailgun/ttlmap v0.0.0-20150816203249-16b258d86efc // indirect
	github.com/maruel/panicparse v0.0.0-20180806203336-f20d4c4d746f // indirect
	github.com/mattn/go-isatty v0.0.6 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/mattn/go-sqlite3 v1.13.0 // indirect
	github.com/mdp/rsc v0.0.0-20160131164516-90f07065088d // indirect
	github.com/miekg/dns v1.1.8
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/go-ps v0.0.0-20170309133038-4fdf99ab2936
	github.com/mitchellh/go-testing-interface v1.0.0 // indirect
	github.com/mitchellh/hashstructure v1.0.0 // indirect
	github.com/nsf/termbox-go v0.0.0-20190325093121-288510b9734e // indirect
	github.com/olekukonko/tablewriter v0.0.1
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/pborman/uuid v1.2.0
	github.com/pkg/errors v0.9.1 // indirect
	github.com/posener/complete v1.1.2 // indirect
	github.com/pquerna/otp v0.0.0-20160912161815-54653902c20e // indirect
	github.com/prometheus/alertmanager v0.18.0
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/common v0.4.1
	github.com/prometheus/procfs v0.0.5 // indirect
	github.com/rubenv/sql-migrate v0.0.0-20190902133344-8926f37f0bc1 // indirect
	github.com/russellhaering/gosaml2 v0.0.0-20170515204909-8908227c114a // indirect
	github.com/russellhaering/goxmldsig v0.0.0-20170515183101-605161228693 // indirect
	github.com/santhosh-tekuri/jsonschema v1.2.2
	github.com/sirupsen/logrus v1.4.2
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/stevvooe/resumable v0.0.0-20150521211217-51ad44105773 // indirect
	github.com/stretchr/testify v1.4.0
	github.com/tstranex/u2f v0.0.0-20160508205855-eb799ce68da4
	github.com/ulikunitz/xz v0.5.4 // indirect
	github.com/vulcand/oxy v0.0.0-20160623194703-40720199a16c
	github.com/vulcand/predicate v1.1.0
	github.com/xeipuuv/gojsonpointer v0.0.0-20151027082146-e0fe6f683076 // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20150808065054-e02fc20de94c // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20151204154511-3988ac14d6f6 // indirect
	github.com/xtgo/set v1.0.0
	github.com/zclconf/go-cty v0.0.0-20180829180805-c2393a5d54f2 // indirect
	github.com/ziutek/mymysql v1.5.4 // indirect
	go.mongodb.org/mongo-driver v1.0.4 // indirect
	golang.org/x/crypto v0.0.0-20190621222207-cc06ce4a13d4
	golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/sys v0.0.0-20200120151820-655fe14d7479
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2 // indirect
	google.golang.org/genproto v0.0.0-20190307195333-5fe7a883aa19 // indirect
	google.golang.org/grpc v1.19.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127
	gopkg.in/gorp.v1 v1.7.2 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/mgo.v2 v2.0.0-20160818020120-3f83fa500528 // indirect
	gopkg.in/square/go-jose.v2 v2.2.0 // indirect
	gopkg.in/yaml.v2 v2.2.4
	k8s.io/api v0.15.7
	k8s.io/apiextensions-apiserver v0.15.7
	k8s.io/apimachinery v0.15.7
	k8s.io/client-go v0.15.7
	k8s.io/helm v2.15.2+incompatible
	k8s.io/kube-aggregator v0.15.7
	k8s.io/kubernetes v1.15.7
	k8s.io/utils v0.0.0-20191010214722-8d271d903fe4 // indirect
	launchpad.net/gocheck v0.0.0-20140225173054-000000000087 // indirect
	vbom.ml/util v0.0.0-20180919145318-efcd4e0f9787 // indirect
)

replace (
	github.com/coreos/go-oidc => github.com/gravitational/go-oidc v0.0.1
	github.com/julienschmidt/httprouter => github.com/julienschmidt/httprouter v1.1.0
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	github.com/sirupsen/logrus => github.com/gravitational/logrus v0.10.1-0.20180402202453-dcdb95d728db
	gopkg.in/alecthomas/kingpin.v2 => github.com/gravitational/kingpin v2.1.11-0.20180808090833-85085db9f49b+incompatible
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
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.15.7
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.15.7
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.15.7
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.15.7
	k8s.io/kubelet => k8s.io/kubelet v0.15.7
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.15.7
	k8s.io/metrics => k8s.io/metrics v0.15.7
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.15.7
)
