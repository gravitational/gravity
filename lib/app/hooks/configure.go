/*
Copyright 2018 Gravitational, Inc.

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

package hooks

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"text/template"
	"time"

	"github.com/gravitational/gravity/lib/app/resources"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"

	"github.com/gravitational/rigging"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
)

// configureJob augments the provided job spec with the proper metadata (e.g. to ensure unique name),
// volumes and their mounts, init containers and other things to ensure that the hook is configured
// properly and has access to application resources
func configureJob(job *batchv1.Job, p Params) error {
	if err := configureMetadata(job, p); err != nil {
		return trace.Wrap(err)
	}

	if !p.SkipInitContainers {
		if err := configureInitContainer(job, p); err != nil {
			return trace.Wrap(err)
		}
	}

	configureVolumes(job, p)
	configureVolumeMounts(job, p)
	if err := configureSecurityContext(job, p); err != nil {
		return trace.Wrap(err)
	}
	configureTolerations(job)
	configureNetwork(job, p)

	return nil
}

// configureMetadata updates the provided job spec with the appropriate metadata
func configureMetadata(job *batchv1.Job, p Params) error {
	job.Kind = rigging.KindJob
	if job.APIVersion == "" {
		job.APIVersion = batchv1.SchemeGroupVersion.String()
	}

	// create the hook job in the system namespace unless specified otherwise
	if job.ObjectMeta.Namespace == "" {
		job.ObjectMeta.Namespace = defaults.KubeSystemNamespace
	}

	// make sure the name is unique
	suffix, err := teleutils.CryptoRandomHex(3)
	if err != nil {
		return trace.Wrap(err)
	}

	job.ObjectMeta.Name = fmt.Sprintf("%v-%v", job.ObjectMeta.Name, suffix)

	// specify node selector so it runs on master but keep any existing selector labels
	if job.Spec.Template.Spec.NodeSelector == nil {
		job.Spec.Template.Spec.NodeSelector = make(map[string]string)
	}
	for name, value := range p.NodeSelector {
		job.Spec.Template.Spec.NodeSelector[name] = value
	}

	// add environment variables
	for i := range job.Spec.Template.Spec.Containers {
		for name, value := range p.Env {
			job.Spec.Template.Spec.Containers[i].Env = append(
				job.Spec.Template.Spec.Containers[i].Env,
				v1.EnvVar{Name: name, Value: value})
		}
		job.Spec.Template.Spec.Containers[i].Env = append(
			job.Spec.Template.Spec.Containers[i].Env,
			v1.EnvVar{
				Name:  constants.ServiceUserEnvVar,
				Value: p.ServiceUser.UID,
			},
		)
		// set image pull policy if none specified
		if job.Spec.Template.Spec.Containers[i].ImagePullPolicy == "" {
			job.Spec.Template.Spec.Containers[i].ImagePullPolicy = v1.PullIfNotPresent
		}
	}

	// if deadline is not specified, set the default one
	if job.Spec.ActiveDeadlineSeconds == nil {
		job.Spec.ActiveDeadlineSeconds = new(int64)
	}
	if *job.Spec.ActiveDeadlineSeconds == 0 {
		*job.Spec.ActiveDeadlineSeconds = int64(time.Duration(
			defaults.HookJobDeadline).Seconds())
	}
	// deadline may have been overridden via hook request, if so, it takes precendence
	if p.JobDeadline != 0 {
		*job.Spec.ActiveDeadlineSeconds = int64(p.JobDeadline.Seconds())
	}

	// if securityContext is not specified, set the default one
	if job.Spec.Template.Spec.SecurityContext == nil {
		job.Spec.Template.Spec.SecurityContext = defaults.HookSecurityContext()
	}

	// if priorityClassName is not specified, set the default one
	if job.Spec.Template.Spec.PriorityClassName == "" {
		job.Spec.Template.Spec.PriorityClassName = defaults.HookPriorityClassName
	}

	return nil
}

// configureVolumes updates the job spec with required volumes so they are available
// to init and hook containers
func configureVolumes(job *batchv1.Job, p Params) {
	volumes := append(p.Volumes, []v1.Volume{
		{
			Name: VolumeBin,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: defaults.HostBin,
				},
			},
		},
		{
			Name: VolumeKubectlBin,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: defaults.KubectlBin,
				},
			},
		},
		{
			Name: VolumeHelmBin,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: defaults.HelmBin,
				},
			},
		},
		{
			Name: VolumeCerts,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: defaults.CertsDir,
				},
			},
		},
		{
			Name: VolumeGravity,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: defaults.LocalGravityDir,
				},
			},
		},
		{
			Name: VolumeResources,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: VolumeHelm,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: VolumeStateDir,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}...)

	job.Spec.Template.Spec.Volumes = append(
		job.Spec.Template.Spec.Volumes, volumes...)
}

// configureVolumeMounts updates all hook containers of the provided job spec with
// proper volume mounts
func configureVolumeMounts(job *batchv1.Job, p Params) {
	mounts := append(p.Mounts, []v1.VolumeMount{
		{
			Name:      VolumeBin,
			MountPath: ContainerHostBinDir,
		},
		{
			Name:      VolumeKubectlBin,
			MountPath: KubectlPath,
		},
		{
			Name:      VolumeHelmBin,
			MountPath: HelmPath,
		},
		{
			Name:      VolumeCerts,
			MountPath: defaults.CertsDir,
		},
		{
			Name:      VolumeResources,
			MountPath: ResourcesDir,
		},
		{
			Name:      VolumeHelm,
			MountPath: HelmDir,
		},
	}...)

	for i := range job.Spec.Template.Spec.Containers {
		job.Spec.Template.Spec.Containers[i].VolumeMounts = append(
			job.Spec.Template.Spec.Containers[i].VolumeMounts, mounts...)
	}

	for i := range job.Spec.Template.Spec.InitContainers {
		job.Spec.Template.Spec.InitContainers[i].VolumeMounts = append(
			job.Spec.Template.Spec.InitContainers[i].VolumeMounts, mounts...)
	}
}

// configureSecurityContext updates security contexts for job's Pod and each container
// if the contexts are using defaults.PlaceholderServiceUserID
func configureSecurityContext(job *batchv1.Job, p Params) error {
	if p.ServiceUser.IsEmpty() {
		return nil
	}
	uid, err := strconv.Atoi(p.ServiceUser.UID)
	if err != nil {
		return trace.Wrap(err)
	}
	gid, err := strconv.Atoi(p.ServiceUser.GID)
	if err != nil {
		return trace.Wrap(err)
	}
	resources.UpdateSecurityContext(&job.Spec.Template.Spec,
		systeminfo.User{Name: p.ServiceUser.Name, UID: uid, GID: gid})
	return nil
}

// configureInitContainer updates the job spec with init container that will be used to
// unpack application resources to make them available to hooks
func configureInitContainer(job *batchv1.Job, p Params) error {
	var buf bytes.Buffer
	err := initScript(&buf, p)
	if err != nil {
		return trace.Wrap(err, "failed to generate init script for %s, hook %s", p.Locator.String(), p.Hook.Type)
	}

	if len(job.Spec.Template.Spec.InitContainers) == 0 || job.Spec.Template.Spec.InitContainers[0].Name != InitContainerName {
		// prepend our init container to the beginning of the list, so that it's run first
		job.Spec.Template.Spec.InitContainers = append([]v1.Container{
			{
				Name:            InitContainerName,
				Image:           InitContainerImage,
				Command:         []string{"/bin/sh", "-c", "-e"},
				Args:            []string{buf.String()},
				ImagePullPolicy: v1.PullIfNotPresent,
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      VolumeGravity,
						MountPath: defaults.LocalGravityDir,
					},
					{
						Name:      VolumeStateDir,
						MountPath: StateDir,
					},
				},
				SecurityContext: &v1.SecurityContext{
					SELinuxOptions: &v1.SELinuxOptions{
						Type: constants.GravitySystemContainerType,
					},
				},
			},
		}, job.Spec.Template.Spec.InitContainers...)
	}

	envs := []v1.EnvVar{
		{
			Name:  ApplicationPackageEnv,
			Value: p.Locator.String(),
		},
		{
			Name: PodIPEnv,
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
	}

	for i := range job.Spec.Template.Spec.InitContainers {
		for _, env := range envs {
			updateEnvInContainer(&job.Spec.Template.Spec.InitContainers[i], env)
		}
	}
	return nil
}

func updateEnvInContainer(container *v1.Container, env v1.EnvVar) {
	// only add the environment variable if it doesn't already exist
	for _, e := range container.Env {
		if e.Name == env.Name {
			// Nothing to do
			return
		}
	}
	container.Env = append(container.Env, env)
}

// configureTolerations updates Pod spec with tolerations to allow
// pods to run on master nodes
//
// Looking at examples from the kubernetes sources, NoSchedule and NoExecute need to be applied individually
// https://github.com/kubernetes/kubernetes/blob/master/cluster/addons/fluentd-gcp/fluentd-gcp-ds.yaml
// https://github.com/kubernetes/kubernetes/pull/57122/files
func configureTolerations(job *batchv1.Job) {
	job.Spec.Template.Spec.Tolerations = append(
		job.Spec.Template.Spec.Tolerations,
		v1.Toleration{
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoSchedule,
		},
		v1.Toleration{
			Operator: v1.TolerationOpExists,
			Effect:   v1.TaintEffectNoExecute,
		},
	)
}

// configureNetwork updates the jobs pod network settings
// - Set hostnetwork=true, so that jobs can run to install kubernetes networking, or while networking is unavailable
func configureNetwork(job *batchv1.Job, p Params) {
	if p.HostNetwork {
		job.Spec.Template.Spec.HostNetwork = true
	}
}

// initScript builds a shell script used as init container entrypoint for this hook
func initScript(w io.Writer, p Params) error {
	ctx := initScriptContext{
		Package:        p.Locator.String(),
		AgentUser:      p.AgentUser,
		AgentPassword:  p.AgentPassword,
		ResourcesDir:   ResourcesDir,
		StateDir:       StateDir,
		ServiceUser:    p.ServiceUser,
		HelmDir:        HelmDir,
		HelmValuesFile: HelmValuesFile,
		HelmValues:     string(p.Values),
	}
	if !p.GravityPackage.IsEmpty() {
		ctx.GravityPackage = p.GravityPackage.String()
	}
	var script *template.Template

	switch p.Hook.Type {
	case schema.HookInstall, schema.HookInstalled, schema.HookNetworkInstall:
		// During initial installation the package should be unpacked directly from local state, in
		// other cases it will be downloaded from the running gravity site
		script = initInstallScriptTemplate
	default:
		ctx.ServiceURL = defaults.GravityServiceURL
		ctx.DirectServiceAddr = fmt.Sprintf("$%v:$%v", defaults.GravityServiceHostEnv, defaults.GravityServicePortEnv)
		script = initScriptTemplate
	}

	err := script.Execute(w, &ctx)
	return trace.Wrap(err)
}

var initScriptTemplate = template.Must(template.New("sh").Parse(`
ops_url={{.ServiceURL}};
if [ "{{.DirectServiceAddr}}" != ":" ]; then ops_url=https://{{.DirectServiceAddr}}; fi;
/opt/bin/gravity --state-dir={{.StateDir}} ops connect $ops_url {{.AgentUser}} {{.AgentPassword}};
{{if .GravityPackage}}/opt/bin/gravity --state-dir={{.StateDir}} package export \
	{{.GravityPackage}} {{.StateDir}}/gravity \
	--file-mask=0755 \
	--insecure \
	--ops-url=$ops_url
{{else}}
cp /opt/bin/gravity {{.StateDir}}/
{{end}}
TMPDIR={{.StateDir}} {{.StateDir}}/gravity --state-dir={{.StateDir}} app unpack \
	--service-uid={{.ServiceUser.UID}} \
	--insecure --ops-url=$ops_url \
	{{.Package}} {{.ResourcesDir}}
cat <<EOF > {{.HelmDir}}/{{.HelmValuesFile}}
{{.HelmValues}}
EOF
`))

var initInstallScriptTemplate = template.Must(template.New("sh").Parse(`
TMPDIR={{.StateDir}} /opt/bin/gravity app unpack --service-uid={{.ServiceUser.UID}} {{.Package}} {{.ResourcesDir}}
cat <<EOF > {{.HelmDir}}/{{.HelmValuesFile}}
{{.HelmValues}}
EOF
`))

type initScriptContext struct {
	// StateDir specifies the location to use a state directory to store
	// temporary files and login information
	StateDir string
	// ServiceURL specifies the URL to the cluster controller service
	ServiceURL string
	// DirectServiceAddr specifies the optional addres of the cluster
	// controller service as IP:Port. If specified, has priority over
	// the DNS name to avoid a resolution step.
	//
	// Only meaningful for hooks other than the install hook. The install hook
	// always uses the DNS name to resolve the cluster controller address
	DirectServiceAddr string
	// AgentUser specifies the user to use for login and fetching resources
	AgentUser string
	// AgentPassword specifies the password for the agent user (see above)
	AgentPassword string
	// ResourcesDir specifies the directory where hooks expect to find the
	// application resources
	ResourcesDir string
	// GravityPackage overrides the gravity binary that hooks use for commands
	// following login.
	// If empty, the binary from host is used
	GravityPackage string
	// Package is the application package this hook is for
	Package string
	// ServiceUser specifies the service user to use for overriding
	// the security context of the hook
	ServiceUser storage.OSUser
	// HelmDir is the directory where helm-related data is mounted
	HelmDir string
	// HelmValuesFile is the name of the file with helm values
	HelmValuesFile string
	// HelmValues are helm values in a marshaled yaml format
	HelmValues string
}
