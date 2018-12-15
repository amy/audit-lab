/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	auditinstall "k8s.io/apiserver/pkg/apis/audit/install"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/audit"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	auditclientset "github.com/pbarker/audit-lab/pkg/client/clientset/versioned"
	pluginpolicy "github.com/pbarker/audit-lab/pkg/plugins/policy"
	pluginprinter "github.com/pbarker/audit-lab/pkg/plugins/printer"
)

var (
	events     = []auditv1.Event{}
	encoder    runtime.Encoder
	decoder    runtime.Decoder
	masterURL  string
	kubeconfig string
	plugin     audit.Backend
	scheme     *runtime.Scheme
)

// Parameters for server
type Parameters struct {
	port                  int    // webhook server port
	certFile              string // path to the x509 certificate for https
	keyFile               string // path to the x509 private key matching `CertFile`
	auditBackendName      string // name of the auditbackend this is implementing
	auditBackendNamespace string // namespace of the auditbackend
}

func main() {
	var parameters Parameters

	// get command line parameters
	flag.IntVar(&parameters.port, "port", 443, "Webhook server port.")
	flag.StringVar(&parameters.certFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.keyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")
	flag.StringVar(&parameters.auditBackendNamespace, "backendnamespace", "audit", "namespace of the backend")
	flag.StringVar(&parameters.auditBackendName, "backendname", "", "name of the backend")
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()

	// fetch configuration
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}
	auditclient, err := auditclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error creating audit client: %s", err.Error())
	}

	backend, err := auditclient.Audit().AuditBackends(parameters.auditBackendNamespace).Get(parameters.auditBackendName, metav1.GetOptions{})
	if err != nil {
		klog.Fatalf("could not fetch audit backend: %v", err)
	}

	pluginRules := []*pluginpolicy.ClassRule{}
	for _, classRule := range backend.Spec.Policy.ClassRules {
		class, err := auditclient.Audit().AuditClasses().Get(classRule.Name, metav1.GetOptions{})
		if err != nil {
			klog.Fatalf("could not fetch audit class: %v", err)
		}
		pluginRule := pluginpolicy.NewClassRule(class, classRule)
		pluginRules = append(pluginRules, pluginRule)
	}

	// init backends
	printerBackend := pluginprinter.NewBackend()
	enforcer := pluginpolicy.NewEnforcer(pluginRules)
	plugin = pluginpolicy.NewBackend(printerBackend, enforcer)

	// create serializers
	scheme = runtime.NewScheme()
	auditinstall.Install(scheme)
	codecs := serializer.NewCodecFactory(scheme)
	decoder = codecs.UniversalDecoder(auditinternal.SchemeGroupVersion, auditv1.SchemeGroupVersion)

	// handle ssl
	pair, err := tls.LoadX509KeyPair(parameters.certFile, parameters.keyFile)
	if err != nil {
		klog.Errorf("Filed to load key pair: %v", err)
	}

	// define http server and server handler
	server := &http.Server{
		Addr:      fmt.Sprintf(":%v", parameters.port),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	server.Handler = mux

	// start webhook server in new rountine
	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil {
			klog.Errorf("Filed to listen and serve webhook server: %v", err)
		}
	}()

	// listening OS shutdown singal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	klog.Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
	server.Shutdown(context.Background())
}

func handler(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		klog.Fatalf("could not read request body: %v", err)
	}
	el := &auditinternal.EventList{}

	if err := runtime.DecodeInto(decoder, body, el); err != nil {
		klog.Fatalf("failed decoding buf: %b, apiVersion: %s", body, auditv1.SchemeGroupVersion)
	}
	defer req.Body.Close()

	for _, event := range el.Items {
		plugin.ProcessEvents(&event)
	}
	w.WriteHeader(http.StatusOK)
	return
}
