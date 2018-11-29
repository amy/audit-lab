/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"time"

	v1alpha1 "github.com/pbarker/audit-lab/pkg/apis/audit/v1alpha1"
	scheme "github.com/pbarker/audit-lab/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// AuditBackendsGetter has a method to return a AuditBackendInterface.
// A group's client should implement this interface.
type AuditBackendsGetter interface {
	AuditBackends(namespace string) AuditBackendInterface
}

// AuditBackendInterface has methods to work with AuditBackend resources.
type AuditBackendInterface interface {
	Create(*v1alpha1.AuditBackend) (*v1alpha1.AuditBackend, error)
	Update(*v1alpha1.AuditBackend) (*v1alpha1.AuditBackend, error)
	UpdateStatus(*v1alpha1.AuditBackend) (*v1alpha1.AuditBackend, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.AuditBackend, error)
	List(opts v1.ListOptions) (*v1alpha1.AuditBackendList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.AuditBackend, err error)
	AuditBackendExpansion
}

// auditBackends implements AuditBackendInterface
type auditBackends struct {
	client rest.Interface
	ns     string
}

// newAuditBackends returns a AuditBackends
func newAuditBackends(c *AuditV1alpha1Client, namespace string) *auditBackends {
	return &auditBackends{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the auditBackend, and returns the corresponding auditBackend object, and an error if there is any.
func (c *auditBackends) Get(name string, options v1.GetOptions) (result *v1alpha1.AuditBackend, err error) {
	result = &v1alpha1.AuditBackend{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("auditbackends").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of AuditBackends that match those selectors.
func (c *auditBackends) List(opts v1.ListOptions) (result *v1alpha1.AuditBackendList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.AuditBackendList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("auditbackends").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested auditBackends.
func (c *auditBackends) Watch(opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("auditbackends").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a auditBackend and creates it.  Returns the server's representation of the auditBackend, and an error, if there is any.
func (c *auditBackends) Create(auditBackend *v1alpha1.AuditBackend) (result *v1alpha1.AuditBackend, err error) {
	result = &v1alpha1.AuditBackend{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("auditbackends").
		Body(auditBackend).
		Do().
		Into(result)
	return
}

// Update takes the representation of a auditBackend and updates it. Returns the server's representation of the auditBackend, and an error, if there is any.
func (c *auditBackends) Update(auditBackend *v1alpha1.AuditBackend) (result *v1alpha1.AuditBackend, err error) {
	result = &v1alpha1.AuditBackend{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("auditbackends").
		Name(auditBackend.Name).
		Body(auditBackend).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *auditBackends) UpdateStatus(auditBackend *v1alpha1.AuditBackend) (result *v1alpha1.AuditBackend, err error) {
	result = &v1alpha1.AuditBackend{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("auditbackends").
		Name(auditBackend.Name).
		SubResource("status").
		Body(auditBackend).
		Do().
		Into(result)
	return
}

// Delete takes name of the auditBackend and deletes it. Returns an error if one occurs.
func (c *auditBackends) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("auditbackends").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *auditBackends) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("auditbackends").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched auditBackend.
func (c *auditBackends) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.AuditBackend, err error) {
	result = &v1alpha1.AuditBackend{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("auditbackends").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}