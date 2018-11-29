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
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	auditinstall "k8s.io/apiserver/pkg/apis/audit/install"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/audit"
)

var (
	events  = []auditv1.Event{}
	encoder runtime.Encoder
	decoder runtime.Decoder
)

func main() {
	scheme := runtime.NewScheme()
	auditinstall.Install(scheme)
	serializer := json.NewSerializer(json.DefaultMetaFactory, scheme, scheme, false)
	encoder = audit.Codecs.EncoderForVersion(serializer, auditv1.SchemeGroupVersion)
	decoder = audit.Codecs.UniversalDecoder(auditv1.SchemeGroupVersion)

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handler(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Fatalf("could not read request body: %v", err)
	}
	el := &auditv1.EventList{}

	if err := runtime.DecodeInto(decoder, body, el); err != nil {
		log.Fatalf("failed decoding buf: %b, apiVersion: %s", body, auditv1.SchemeGroupVersion)
	}
	defer req.Body.Close()

	// write events to stdout
	for _, event := range el.Items {
		err := encoder.Encode(&event, os.Stdout)
		if err != nil {
			log.Fatalf("could not encode audit event: %v", err)
		}
	}
	w.WriteHeader(http.StatusOK)
	return
}
