/*
Copyright 2019 The Kubernetes Authors.

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

package app

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"

	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	appConf "github.com/kubernetes-sigs/kube-batch/cmd/admission/app/options"
	admissioncontroller "github.com/kubernetes-sigs/kube-batch/pkg/admission"
)

const (
	// CONTENTTYPE type of request content
	CONTENTTYPE = "Content-Type"
	// APPLICATIONJSON  json application content
	APPLICATIONJSON = "application/json"
)

// GetClient gets a clientset with in-cluster config.
func GetClient(c *appConf.Config) *kubernetes.Clientset {
	var config *rest.Config
	var err error
	if c.Master != "" || c.Kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags(c.Master, c.Kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		glog.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatal(err)
	}
	return clientset
}

// ConfigTLS configure TLs certificates
func ConfigTLS(config *appConf.Config, clientset *kubernetes.Clientset) *tls.Config {
	sCert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		glog.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
	}
}

// Serve the http Request for admission controller
func Serve(w http.ResponseWriter, r *http.Request, admit admissioncontroller.AdmitFunc) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get(CONTENTTYPE)
	if contentType != APPLICATIONJSON {
		glog.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	var reviewResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	deserializer := admissioncontroller.Codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		reviewResponse = admissioncontroller.ToAdmissionResponse(err)
	} else {
		reviewResponse = admit(ar)
	}
	glog.V(3).Infof("sending response: %v", reviewResponse)

	response := createResponse(reviewResponse, &ar)
	resp, err := json.Marshal(response)
	if err != nil {
		glog.Error(err)
	}
	if _, err := w.Write(resp); err != nil {
		glog.Error(err)
	}
}

func createResponse(reviewResponse *v1beta1.AdmissionResponse, ar *v1beta1.AdmissionReview) v1beta1.AdmissionReview {
	response := v1beta1.AdmissionReview{}
	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = ar.Request.UID
	}
	// reset the Object and OldObject, they are not needed in a response.
	ar.Request.Object = runtime.RawExtension{}
	ar.Request.OldObject = runtime.RawExtension{}

	return response
}
