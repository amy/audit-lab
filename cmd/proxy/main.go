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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	auditinstall "k8s.io/apiserver/pkg/apis/audit/install"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/audit"
	"k8s.io/klog"
)

var (
	events  = []auditv1.Event{}
	encoder runtime.Encoder
	decoder runtime.Decoder
)

// Parameters for server
type Parameters struct {
	port           int    // webhook server port
	certFile       string // path to the x509 certificate for https
	keyFile        string // path to the x509 private key matching `CertFile`
	sidecarCfgFile string // path to sidecar injector configuration file
}

// ok this proxy should have 2 informers pulled from a lib, that inform on the backend it is set to use and the class objects

func main() {
	var parameters Parameters

	// get command line parameters
	flag.IntVar(&parameters.port, "port", 443, "Webhook server port.")
	flag.StringVar(&parameters.certFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.keyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()

	scheme := runtime.NewScheme()
	auditinstall.Install(scheme)
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme, scheme)
	encoder = audit.Codecs.EncoderForVersion(serializer, auditv1.SchemeGroupVersion)
	decoder = audit.Codecs.UniversalDecoder(auditv1.SchemeGroupVersion)

	pair, err := tls.LoadX509KeyPair(parameters.certFile, parameters.keyFile)
	if err != nil {
		klog.Errorf("Filed to load key pair: %v", err)
	}

	server := &http.Server{
		Addr:      fmt.Sprintf(":%v", parameters.port),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
	}

	// define http server and server handler
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
	el := &auditv1.EventList{}

	if err := runtime.DecodeInto(decoder, body, el); err != nil {
		klog.Fatalf("failed decoding buf: %b, apiVersion: %s", body, auditv1.SchemeGroupVersion)
	}
	defer req.Body.Close()

	// write events to stdout
	for _, event := range el.Items {
		err := encoder.Encode(&event, os.Stdout)
		if err != nil {
			klog.Fatalf("could not encode audit event: %v", err)
		}
		fmt.Printf("\n----------\n\n")
	}
	w.WriteHeader(http.StatusOK)
	return
}
