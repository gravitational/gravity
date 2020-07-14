module github.com/gravitational/magnet

go 1.14

replace (
	github.com/containerd/containerd => github.com/containerd/containerd v1.3.1-0.20200512144102-f13ba8f2f2fd
	github.com/docker/docker => github.com/docker/docker v17.12.0-ce-rc1.0.20200310163718-4634ce647cf2+incompatible
	github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
)

require (
	github.com/containerd/cgroups v0.0.0-20200625214700-544d8ea40afe // indirect
	github.com/containerd/console v1.0.0
	github.com/containerd/continuity v0.0.0-20200413184840-d3ef23f19fbb // indirect
	github.com/docker/docker v1.13.1 // indirect
	github.com/gofrs/flock v0.7.1 // indirect
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/gravitational/trace v1.1.11
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/magefile/mage v1.9.0
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/moby/buildkit v0.7.1
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/olekukonko/tablewriter v0.0.4
	github.com/opencontainers/go-digest v1.0.0
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/tonistiigi/fsutil v0.0.0-20200512175118-ae3a8d753069 // indirect
	go.opencensus.io v0.22.4 // indirect
	golang.org/x/net v0.0.0-20200625001655-4c5254603344 // indirect
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208 // indirect
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	golang.org/x/text v0.3.3 // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1 // indirect
	google.golang.org/genproto v0.0.0-20200626011028-ee7919e894b5 // indirect
	google.golang.org/grpc v1.30.0 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
)
