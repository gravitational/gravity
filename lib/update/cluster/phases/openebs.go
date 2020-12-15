/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package phases

import (
	"bytes"
	"context"
	"strings"
	"text/template"

	"github.com/gravitational/gravity/lib/fsm"
	k8sutil "github.com/gravitational/gravity/lib/kubernetes"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// Upgrades OpenEBS data plane components.
// Follows the upgrade steps as described at:
// https://github.com/openebs/openebs/blob/master/k8s/upgrades/README.md

const (
	k8sJobPrefix = "cstor"
)

// PhaseUpgradePool has the info needed for a pool upgrade
type PhaseUpgradePool struct {
	// FieldLogger is used for logging
	log.FieldLogger
	// Client is an API client to the kubernetes API
	Client      *kubernetes.Clientset
	Pool        string
	FromVersion string
	ToVersion   string
}

// NewPhaseUpgradePool creates a pool upgrade phase
func NewPhaseUpgradePool(phase storage.OperationPhase, client *kubernetes.Clientset, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	poolAndVer := strings.Split(phase.Data.Data, " ")
	return &PhaseUpgradePool{
		FieldLogger: logger,
		Client:      client,
		Pool:        poolAndVer[0],
		FromVersion: poolAndVer[1],
		ToVersion:   poolAndVer[2],
	}, nil
}

// Execute runs the upgrade steps
func (p *PhaseUpgradePool) Execute(ctx context.Context) error {
	err := p.execPoolUpgradeCmd(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// PoolUpgrade holds the info needed for pool upgrade
type PoolUpgrade struct {
	Pool        string
	FromVersion string
	ToVersion   string
	JobName     string
}

func (p *PhaseUpgradePool) execPoolUpgradeCmd(ctx context.Context) error {
	jobName := k8sutil.MakeJobName(k8sJobPrefix, p.Pool)

	jobSpec, err := execTemplate(poolUpgradeJobTemplate, &PoolUpgrade{Pool: p.Pool,
		FromVersion: p.FromVersion, ToVersion: p.ToVersion, JobName: jobName})
	if err != nil {
		return trace.Wrap(err)
	}

	out, err := k8sutil.ExecJob(ctx, jobSpec, p.Client)

	p.Infof("OpenEBS pool upgrade job output: %v", out)

	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Rollback gets executed when a rollback is requested
func (p *PhaseUpgradePool) Rollback(context.Context) error {
	p.Warnf(rollbackNotSupported(), "pool", p.Pool, p.FromVersion, p.ToVersion)

	return nil
}

func rollbackNotSupported() string {
	return "Skipping rollback of OpenEBS %v %v because rollback is not supported by OpenEBS" +
		" for upgrade path: fromVersion=%v -> toVersion=%v "
}

// PreCheck gets executed before the upgrade steps
func (*PhaseUpgradePool) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck gets executed after the upgrade steps
func (*PhaseUpgradePool) PostCheck(context.Context) error {
	return nil
}

// PhaseUpgradeVolume has the info needed for a volume upgrade
type PhaseUpgradeVolume struct {
	// FieldLogger is used for logging
	log.FieldLogger
	// Client is an API client to the kubernetes API
	Client      *kubernetes.Clientset
	Volume      string
	FromVersion string
	ToVersion   string
}

// NewPhaseUpgradeVolume creates a volume upgrade phase
func NewPhaseUpgradeVolume(phase storage.OperationPhase, client *kubernetes.Clientset, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	volAndVer := strings.Split(phase.Data.Data, " ")
	return &PhaseUpgradeVolume{
		FieldLogger: logger,
		Client:      client,
		Volume:      volAndVer[0],
		FromVersion: volAndVer[1],
		ToVersion:   volAndVer[2],
	}, nil
}

// Execute runs the upgrade steps
func (p *PhaseUpgradeVolume) Execute(ctx context.Context) error {
	err := p.execVolumeUpgradeCmd(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// VolumeUpgrade holds the info needed for volume upgrade
type VolumeUpgrade struct {
	Volume      string
	FromVersion string
	ToVersion   string
	JobName     string
}

func (p *PhaseUpgradeVolume) execVolumeUpgradeCmd(ctx context.Context) error {
	jobName := k8sutil.MakeJobName(k8sJobPrefix, p.Volume)

	jobSpec, err := execTemplate(volumeUpgradeJobTemplate, &VolumeUpgrade{Volume: p.Volume,
		FromVersion: p.FromVersion, ToVersion: p.ToVersion, JobName: jobName})
	if err != nil {
		return trace.Wrap(err)
	}

	out, err := k8sutil.ExecJob(ctx, jobSpec, p.Client)

	p.Infof("OpenEBS volume upgrade job output: %v", out)

	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Rollback gets executed when a rollback is requested
func (p *PhaseUpgradeVolume) Rollback(context.Context) error {
	p.Warnf(rollbackNotSupported(), "volume", p.Volume, p.FromVersion, p.ToVersion)

	return nil
}

// PreCheck gets executed before the upgrade steps
func (*PhaseUpgradeVolume) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck gets executed after the upgrade steps
func (*PhaseUpgradeVolume) PostCheck(context.Context) error {
	return nil
}

// The upgrade jobs are taken from the following OpenEBS upgrade procedure:
// https://github.com/openebs/openebs/blob/master/k8s/upgrades/README.md
var poolUpgradeJobTemplate = template.Must(template.New("upgradePool").Parse(`
#This is an example YAML for upgrading cstor SPC.
#Some of the values below needs to be changed to
#match your openebs installation. The fields are
#indicated with VERIFY
---
apiVersion: batch/v1
kind: Job
metadata:
  #VERIFY that you have provided a unique name for this upgrade job.
  #The name can be any valid K8s string for name. 
  name: {{.JobName}}

  #VERIFY the value of namespace is same as the namespace where openebs components
  # are installed. You can verify using the command:
  # kubectl get pods -n <openebs-namespace> -l openebs.io/component-name=maya-apiserver
  # The above command should return status of the openebs-apiserver.
  namespace: openebs
spec:
  template:
    spec:
      #VERIFY the value of serviceAccountName is pointing to service account
      # created within openebs namespace. Use the non-default account.
      # by running kubectl get sa -n <openebs-namespace>
      serviceAccountName: openebs-maya-operator
      containers:
      - name:  upgrade
        args:
        - "cstor-spc"

        # --from-version is the current version of the pool
        - "--from-version={{.FromVersion}}"

        # --to-version is the version desired upgrade version
        - "--to-version={{.ToVersion}}"

        # Bulk upgrade is supported
        # To make use of it, please provide the list of SPCs
        # as mentioned below
        - "{{.Pool}}"

        #Following are optional parameters
        #Log Level
        - "--v=4"
        #DO NOT CHANGE BELOW PARAMETERS
        env:
        - name: OPENEBS_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        tty: true

        # the image version should be same as the --to-version mentioned above
        # in the args of the job
        image: registry.local:5000/openebs/m-upgrade:{{.ToVersion}}
        imagePullPolicy: Always
      restartPolicy: Never
---
`))

var volumeUpgradeJobTemplate = template.Must(template.New("upgradeVolumes").Parse(`
#This is an example YAML for upgrading cstor volume.
#Some of the values below needs to be changed to
#match your openebs installation. The fields are
#indicated with VERIFY
---
apiVersion: batch/v1
kind: Job
metadata:
  #VERIFY that you have provided a unique name for this upgrade job.
  #The name can be any valid K8s string for name. 
  name: {{.JobName}}

  #VERIFY the value of namespace is same as the namespace
  # where openebs components
  # are installed. You can verify using the command:
  # kubectl get pods -n <openebs-namespace> -l
  # openebs.io/component-name=maya-apiserver
  # The above command should return status of the openebs-apiserver.
  namespace: openebs


spec:
  template:
    spec:
      #VERIFY the value of serviceAccountName is pointing to service account
      # created within openebs namespace. Use the non-default account.
      # by running kubectl get sa -n <openebs-namespace>
      serviceAccountName: openebs-maya-operator
      containers:
        - name: upgrade
          args:
            - "cstor-volume"

            # --from-version is the current version of the volume
            - "--from-version={{.FromVersion}}"

            # --to-version is the version desired upgrade version
            - "--to-version={{.ToVersion}}"

            # Bulk upgrade is supported from 1.9
            # To make use of it, please provide the list of PVs
            # as mentioned below
            - "{{.Volume}}"

            #Following are optional parameters
            #Log Level
            - "--v=4"
          #DO NOT CHANGE BELOW PARAMETERS
          env:
            - name: OPENEBS_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
          tty: true

          # the image version should be same as the --to-version mentioned above
          # in the args of the job
          image: registry.local:5000/openebs/m-upgrade:{{.ToVersion}}
          imagePullPolicy: Always
      restartPolicy: Never
---
`))

func execTemplate(template *template.Template, templateData interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := template.Execute(&buf, templateData)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}
