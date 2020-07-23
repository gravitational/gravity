module github.com/gravitational/magnet

go 1.14

replace (
	github.com/containerd/containerd => github.com/containerd/containerd v1.3.1-0.20200512144102-f13ba8f2f2fd
	github.com/docker/docker => github.com/docker/docker v17.12.0-ce-rc1.0.20200310163718-4634ce647cf2+incompatible
	github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
	github.com/magefile/mage => github.com/knisbet/mage v1.9.1-0.20200719045837-eabe8cda6d46
)

require (
	github.com/containerd/console v1.0.0
	github.com/gravitational/trace v1.1.11
	github.com/jaguilar/vt100 v0.0.0-20150826170717-2703a27b14ea
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/magefile/mage v1.9.0
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/morikuni/aec v1.0.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/opencontainers/go-digest v1.0.0
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/stretchr/testify v1.5.1 // indirect
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea
	golang.org/x/net v0.0.0-20200625001655-4c5254603344 // indirect
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v2 v2.2.8

)
