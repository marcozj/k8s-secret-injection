package main

import (
	"encoding/json"
	"strings"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

const (
	binPath                    = "/centrify/bin"
	secretsFilesPath           = "/centrify/secrets"
	secretVolumeName           = "vault-secret-volume"
	binVolumeName              = "vault-bin-volume"
	oauthTokenVolumeName       = "vault-token"
	oauthTokenPath             = "/var/secrets"
	annotationPrefix           = "vault.centrify.com/"
	annotationMutate           = annotationPrefix + "mutate"
	annotationStatus           = annotationPrefix + "status"
	annotationAppLauncher      = annotationPrefix + "app-launcher"
	annotationTenanturl        = annotationPrefix + "tenant-url"
	annotationAppID            = annotationPrefix + "appid"
	annotationScope            = annotationPrefix + "scope"
	annotationToken            = annotationPrefix + "token"
	annotationOauthSecretName  = annotationPrefix + "oauth-secret-name"
	annotationEnrollmentCode   = annotationPrefix + "enrollment-code"
	annotationAuthType         = annotationPrefix + "auth-type"
	annotationSecretPrefix     = annotationPrefix + "vaultsecret_"
	annotationInitContainer    = annotationPrefix + "init-container"
	annotationSidecarContainer = annotationPrefix + "sidecar-container"
	annotationInitImage        = annotationPrefix + "init-image"
	annotationSidecarImage     = annotationPrefix + "sidecar-image"
)

var ignoredNamespaces = []string{
	metav1.NamespaceSystem,
	metav1.NamespacePublic,
}

func mutatePods(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	klog.Info("mutating pods")
	/*
		podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
		if ar.Request.Resource != podResource {
			return admissionResponseError(fmt.Errorf("expect resource to be %s", podResource))
		}
	*/
	req := ar.Request
	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		klog.Errorf("Could not unmarshal raw object: %v", err)
		return admissionResponseError(err)
	}

	klog.Infof("AdmissionReview for Kind=%v, Namespace=%v (%v) Name=%v UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo)
	klog.Infof("Unmarshal pod: %v", pod)

	// Basic admission response without mutation
	resp := &v1beta1.AdmissionResponse{
		Allowed: true,
		UID:     req.UID,
	}

	thisPod := myPod{}
	thisPod.self = &pod
	thisPod.injectEnvs = thisPod.convertEnv()
	thisPod.initContainerImage = "centrify/secret-injector-oauth"
	thisPod.sideCarContainerImage = "centrify/secret-injector-dmc"
	// Use custom init image if defined
	initimage, ok := thisPod.self.Annotations[annotationInitImage]
	if ok && initimage != "" {
		thisPod.initContainerImage = initimage
	}
	// Use custom sidecar image if defined
	sidecardimage, ok := thisPod.self.Annotations[annotationSidecarImage]
	if ok && sidecardimage != "" {
		thisPod.sideCarContainerImage = sidecardimage
	}
	// determine whether to perform mutation
	inject, err := thisPod.mutateRequired(ignoredNamespaces)
	if err != nil {
		return admissionResponseError(err)
	}
	if !inject {
		klog.Infof("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name)
		return resp
	}

	//annotations := map[string]string{annotationStatus: "injected"}
	patchBytes, err := thisPod.createPatch()
	if err != nil {
		return admissionResponseError(err)
	}

	klog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
	resp.Patch = patchBytes
	patchType := v1beta1.PatchTypeJSONPatch
	resp.PatchType = &patchType

	return resp
}

// Check whether the target resoured need to be mutated
func (p *myPod) mutateRequired(ignoredList []string) (bool, error) {
	klog.Info("Determing if mutation is required...")
	// skip special kubernete system namespaces
	for _, namespace := range ignoredList {
		if p.self.Namespace == namespace {
			klog.Infof("Skip mutation for %v for it' in special namespace:%v", p.self.Name, p.self.Namespace)
			return false, nil
		}
	}

	var mutate bool
	status, ok := p.self.Annotations[annotationStatus]
	if ok && strings.ToLower(status) == "injected" {
		// status is defined and value is injected, ignore
		mutate = false
	} else {
		raw, ok := p.self.Annotations[annotationMutate]
		if !ok {
			mutate = false
		} else {
			klog.Infof("Raw mutate key: %v", raw)
			switch strings.ToLower(raw) {
			case "y", "yes", "true", "on":
				mutate = true
			default:
				mutate = false
			}
		}
	}

	klog.Infof("Mutation policy for %v/%v: status: %q required:%v", p.self.Namespace, p.self.Name, status, mutate)
	return mutate, nil
}

// Check whether the target resoured need to be mutated
func mutateRequired(ignoredList []string, pod *corev1.Pod) (bool, error) {
	klog.Info("Determing if mutation is required...")
	// skip special kubernete system namespaces
	for _, namespace := range ignoredList {
		if pod.Namespace == namespace {
			klog.Infof("Skip mutation for %v for it' in special namespace:%v", pod.Name, pod.Namespace)
			return false, nil
		}
	}

	var mutate bool
	status, ok := pod.Annotations[annotationStatus]
	if ok && strings.ToLower(status) == "injected" {
		// status is defined and value is injected, ignore
		mutate = false
	} else {
		raw, ok := pod.Annotations[annotationMutate]
		if !ok {
			mutate = false
		} else {
			klog.Infof("Raw mutate key: %v", raw)
			switch strings.ToLower(raw) {
			case "y", "yes", "true", "on":
				mutate = true
			default:
				mutate = false
			}
		}
	}

	klog.Infof("Mutation policy for %v/%v: status: %q required:%v", pod.Namespace, pod.Name, status, mutate)
	return mutate, nil
}

///////////////////////////////////////
// create mutation patch for resoures//
///////////////////////////////////////
func (p *myPod) createPatch() ([]byte, error) {
	var patch []patchOperation

	// Create init container. Unless specifically indicates no, it should always be created to at least copy app launcher binary
	init, ok := p.self.Annotations[annotationInitContainer]
	if !(ok && strings.ToLower(init) == "no") {
		patch = append(patch, p.addInitContainer()...)
	}

	// Add volumes amd mounts to mutated containers for copying injector binary and save secret files
	patch = append(patch, p.addVolume()...)
	patch = append(patch, p.addVolumeMount()...)

	// Add volume for mounting oauth token from k8s secret
	// Don't need to add volume mount here. It is added in init and sidecar container calls directly
	patch = append(patch, p.addSecretVolume()...)

	//patch = append(patch, addEnv(pod.Spec.Containers, envs)...)

	// Mutate container command so that it is launched by app launcher that "inserts" secrets into environment variables within the process
	applauncher, ok := p.self.Annotations[annotationAppLauncher]
	if ok && strings.ToLower(applauncher) != "" {
		patch = append(patch, p.mutateCommand(applauncher)...)
	}

	// Create sidecar container
	sidecar, ok := p.self.Annotations[annotationSidecarContainer]
	if ok && strings.ToLower(sidecar) == "yes" {
		patch = append(patch, p.addSidecarContainer()...)
	}

	// Finally, insert annotation to indicate mutation is completed
	annotations := map[string]string{annotationStatus: "injected"}
	patch = append(patch, p.updateAnnotation(annotations)...)

	return json.Marshal(patch)
}

// convertEnv converts certain annotation into environment variables that to be injected
func (p *myPod) convertEnv() map[string]string {
	envs := make(map[string]string)
	for key, value := range p.self.Annotations {
		if strings.HasPrefix(key, annotationSecretPrefix) {
			envs[strings.TrimPrefix(key, annotationSecretPrefix)] = value
		} else {
			switch key {
			case annotationTenanturl:
				envs["VAULT_URL"] = value
			case annotationAppID:
				envs["VAULT_APPID"] = value
			case annotationScope:
				envs["VAULT_SCOPE"] = value
			case annotationToken:
				envs["VAULT_TOKEN"] = value
			case annotationAuthType:
				envs["VAULT_AUTHTYPE"] = value
			case annotationEnrollmentCode:
				envs["VAULT_ENROLLMENTCODE"] = value
			}
		}
	}

	return envs
}
