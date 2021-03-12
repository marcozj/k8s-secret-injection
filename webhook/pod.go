package main

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

type myPod struct {
	self                  *corev1.Pod
	injectEnvs            map[string]string
	initContainerImage    string
	sideCarContainerImage string
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func (p *myPod) addInitContainer() (patch []patchOperation) {

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      secretVolumeName,
			MountPath: secretsFilesPath,
			ReadOnly:  false,
		},
		{
			Name:      binVolumeName,
			MountPath: binPath,
			ReadOnly:  false,
		},
		{
			Name:      oauthTokenVolumeName,
			MountPath: oauthTokenPath,
			ReadOnly:  true,
		},
	}

	// Add environment variables for communicating with the tenant
	var envVars []corev1.EnvVar
	for key, value := range p.injectEnvs {
		var envVar corev1.EnvVar
		envVar.Name = key
		envVar.Value = value
		envVars = append(envVars, envVar)
	}

	klog.Infof("initContainerImage: %s\n", p.initContainerImage)
	//arg := "echo '#!/bin/sh\nexport MYSQL_ROOT_PASSWORD=testdata' > /centrifyvault/injectenv.sh && chmod +x /centrifyvault/injectenv.sh"
	//arg := "/tmp/inject.sh"
	newContainer := corev1.Container{
		Name:            "centrifyk8s-init",
		Image:           p.initContainerImage,
		ImagePullPolicy: "IfNotPresent",
		Env:             envVars,
		VolumeMounts:    volumeMounts,
	}

	return addContainers(p.self.Spec.InitContainers, []corev1.Container{newContainer}, "/spec/initContainers")
}

func (p *myPod) addSidecarContainer() (patch []patchOperation) {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      secretVolumeName,
			MountPath: secretsFilesPath,
			ReadOnly:  false,
		},
		{
			Name:      binVolumeName,
			MountPath: binPath,
			ReadOnly:  false,
		},
		{
			Name:      oauthTokenVolumeName,
			MountPath: oauthTokenPath,
			ReadOnly:  true,
		},
	}

	// Add environment variables for communicating with the tenant
	var envVars []corev1.EnvVar
	for key, value := range p.injectEnvs {
		var envVar corev1.EnvVar
		envVar.Name = key
		envVar.Value = value
		envVars = append(envVars, envVar)
	}

	klog.Infof("sideCarContainerImage: %s\n", p.sideCarContainerImage)
	t := true
	sc := corev1.SecurityContext{Privileged: &t}
	newContainer := corev1.Container{
		Name:            "centrifyk8s-sidecar",
		Image:           p.sideCarContainerImage,
		ImagePullPolicy: "IfNotPresent",
		Env:             envVars,
		VolumeMounts:    volumeMounts,
		//Command:         []string{"/bin/sh", "-c"},
		//Args:            []string{arg},
		// We will run CentrifyCC client in sidecar container and it requires the container to run in privileged mode
		SecurityContext: &sc,
	}

	return addContainers(p.self.Spec.Containers, []corev1.Container{newContainer}, "/spec/containers")
}

func (p *myPod) addVolume() (patch []patchOperation) {
	secretVolume := corev1.Volume{
		Name: secretVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: "Memory",
			},
		},
	}
	binVolume := corev1.Volume{
		Name: binVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: "Memory",
			},
		},
	}

	return addVolumes(p.self.Spec.Volumes, []corev1.Volume{secretVolume, binVolume}, "/spec/volumes")
}

func (p *myPod) addSecretVolume() (patch []patchOperation) {
	secretName, ok := p.self.Annotations[annotationOauthSecretName]
	if ok && secretName != "" {
		secretVolume := corev1.Volume{
			Name: secretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		patch = addVolumes(p.self.Spec.Volumes, []corev1.Volume{secretVolume}, "/spec/volumes")
	}
	return patch
}

func (p *myPod) addVolumeMount() (patch []patchOperation) {
	secretVolumeMount := corev1.VolumeMount{
		Name:      secretVolumeName,
		MountPath: secretsFilesPath,
		ReadOnly:  false,
	}
	binVolumeMount := corev1.VolumeMount{
		Name:      binVolumeName,
		MountPath: binPath,
		ReadOnly:  false,
	}

	for i, container := range p.self.Spec.Containers {
		patch = append(patch, addVolumeMounts(
			container.VolumeMounts,
			[]corev1.VolumeMount{secretVolumeMount, binVolumeMount},
			fmt.Sprintf("/spec/containers/%d/volumeMounts", i))...)
	}

	return patch
}

func addSecretVolumeMount(target []corev1.Container, secretName string) (patch []patchOperation) {
	secretVolumeMount := corev1.VolumeMount{
		Name:      secretName,
		MountPath: "/var/secrets",
		ReadOnly:  true,
	}

	for i, container := range target {
		patch = append(patch, addVolumeMounts(
			container.VolumeMounts,
			[]corev1.VolumeMount{secretVolumeMount},
			fmt.Sprintf("/spec/containers/%d/volumeMounts", i))...)
	}

	return patch
}

func (p *myPod) mutateCommand(launcherPath string) (patch []patchOperation) {
	for i, container := range p.self.Spec.Containers {
		// https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#notes
		args := container.Command
		// the container has no explicitly specified command
		if len(args) == 0 {
			// Get container image entrypoint
			envs := container.Env
			for _, env := range envs {
				if env.Name == "CFYVAULT_CONTAINER_ENTRYPOINT" {
					args = append(args, env.Value)
				} else if env.Name == "CFYVAULT_CONTAINER_CMD" && len(container.Args) == 0 {
					// If no Args are defined we can use the Docker CMD from the image
					args = append(args, env.Value)
				}
			}
		}

		args = append(args, container.Args...)
		container.Command = []string{launcherPath}
		container.Args = args
		klog.Infof("Final container command and args: %v %v", container.Command, container.Args)

		patch = append(patch, patchOperation{
			Op:    "replace",
			Path:  fmt.Sprintf("/spec/containers/%d/command", i),
			Value: append(container.Command, container.Args...),
		})
	}

	return patch
}

//////////////////////////////
///// Inject EnvVars  ////////
//////////////////////////////

func addEnv(target []corev1.Container, envs map[string]string) (patch []patchOperation) {
	var basePath string = "/spec/containers/"
	var addenvdef bool

	var envVars []corev1.EnvVar

	for key, value := range envs {
		var envVar corev1.EnvVar
		envVar.Name = key
		envVar.Value = value
		envVars = append(envVars, envVar)
	}

	for x := 0; x < (len(target)); x++ {
		klog.Infof("Processing Container %v With Existing EnvVars:%v", x, target[x].Env)

		if target[x].Env == nil {
			addenvdef = true
		} else {
			addenvdef = false
		}

		var value interface{}
		path := basePath

		if addenvdef {
			path = path + strconv.Itoa(x) + "/env"
			value = envVars
			klog.Infof("No EnvVars Set ... adding array to PATH === %v  &&  VALUE =======:%v", path, value)
			patch = append(patch, patchOperation{
				Op:    "add",
				Path:  path,
				Value: value,
			})
		} else {
			path = path + strconv.Itoa(x) + "/env/-"
			for _, add := range envVars {
				value = add
				glog.Infof("Injecting PATH === %v  &&  VALUE =======:%v", path, value)
				patch = append(patch, patchOperation{
					Op:    "add",
					Path:  path,
					Value: value,
				})
			}
		}
	}
	return patch
}

func addContainers(target, containers []corev1.Container, base string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, v := range containers {
		value = v
		path := base
		if first {
			first = false
			value = []corev1.Container{v}
		} else {
			path = path + "/-"
		}

		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}

	return patch
}

func addVolumes(target, volumes []corev1.Volume, base string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, v := range volumes {
		value = v
		path := base
		if first {
			first = false
			value = []corev1.Volume{v}
		} else {
			path = path + "/-"
		}

		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func addVolumeMounts(target, mounts []corev1.VolumeMount, base string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, v := range mounts {
		value = v
		path := base
		if first {
			first = false
			value = []corev1.VolumeMount{v}
		} else {
			path = path + "/-"
		}

		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func (p *myPod) updateAnnotation(added map[string]string) (patch []patchOperation) {
	// Initialise annotation if it is empty
	if p.self.Annotations == nil {
		p.self.Annotations = map[string]string{}
	}

	for key, value := range added {
		//if target == nil || target[key] == "" {
		if p.self.Annotations[key] == "" {
			//target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + key,
				Value: value,
			})
		}
	}
	return patch
}
