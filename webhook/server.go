package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog"
)

// ServerParameters Webhook Server parameters
type ServerParameters struct {
	port       int    // webhook server port
	certFile   string // path to the x509 certificate for https
	keyFile    string // path to the x509 private key matching `CertFile`
	envCfgFile string // path to setenv configuration file
}

// WebhookServer webhook server construct
type WebhookServer struct {
	//sidecarConfig *Config
	setenvConfig *setEnvConfig
	server       *http.Server
}

type setEnvConfig struct {
	EnvVars []corev1.EnvVar `yaml:"env"`
}

var scheme = runtime.NewScheme()
var codecs = serializer.NewCodecFactory(scheme)

// admissionResponseError is a helper function to create an AdmissionResponse
// with an embedded error
func admissionResponseError(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

// admitFunc is the type we use for all of our validators and mutators
type admitFunc func(v1beta1.AdmissionReview) *v1beta1.AdmissionResponse

// serve handles the http portion of a request prior to handing to an admit function
func serve(w http.ResponseWriter, r *http.Request, admit admitFunc) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		klog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	klog.V(2).Info(fmt.Sprintf("handling request: %s", body))

	// The AdmissionReview that was sent to the webhook
	adminReviewRequest := v1beta1.AdmissionReview{}

	// The AdmissionReview that will be returned
	adminReviewRespond := v1beta1.AdmissionReview{}

	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &adminReviewRequest); err != nil {
		klog.Errorf("Can't decode body: %v", err)
		adminReviewRespond.Response = admissionResponseError(err)
	} else {
		// pass to admitFunc
		adminReviewRespond.Response = admit(adminReviewRequest)
	}

	// Return the same UID
	//adminReviewRespond.Response.UID = adminReviewRequest.Request.UID

	//klog.V(2).Info(fmt.Sprintf("sending response: %v", adminReviewRespond.Response))
	//klog.Infof("sending response: %v", adminReviewRespond)

	respBytes, err := json.Marshal(adminReviewRespond)
	if err != nil {
		klog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	klog.Infof("Ready to write reponse ...")
	if _, err := w.Write(respBytes); err != nil {
		klog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func serveMutatePods(w http.ResponseWriter, r *http.Request) {
	serve(w, r, mutatePods)
}

func main() {
	var parameters ServerParameters
	// get command line parameters
	flag.IntVar(&parameters.port, "port", 8443, "Webhook server port.")
	flag.StringVar(&parameters.certFile, "tlsCertFile", "/etc/certs/tls.crt", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.keyFile, "tlsKeyFile", "/etc/certs/tls.key", "File containing the x509 private key to --tlsCertFile.")
	//flag.StringVar(&parameters.envCfgFile, "envCfgFile", "/etc/webhook/config/setenvconfig.yaml", "File containing the environment variables we want to inject.")
	flag.Parse()

	certs, err := tls.LoadX509KeyPair(parameters.certFile, parameters.keyFile)
	if err != nil {
		klog.Errorf("Failed to load key pair: %v", err)
	}

	klog.Infoln("Credential is being injected by Mutating Webhook ...")
	//fmt.Println("Credential is being injected by Mutating Webhook ...")

	server := &WebhookServer{
		//setenvConfig: setEnvConfig,
		server: &http.Server{
			Addr:      fmt.Sprintf(":%v", parameters.port),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{certs}},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", serveMutatePods)
	server.server.Handler = mux

	// start webhook server in new rountine
	go func() {
		if err := server.server.ListenAndServeTLS("", ""); err != nil {
			klog.Errorf("Failed to listen and serve webhook server: %v", err)
		}
	}()

	// listening OS shutdown singal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	klog.Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
	server.server.Shutdown(context.Background())
}
