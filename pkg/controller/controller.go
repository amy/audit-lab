/*
Copyright 2017 The Kubernetes Authors.

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

package controller

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	auditregv1alpha1 "k8s.io/api/auditregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	auditreginformers "k8s.io/client-go/informers/auditregistration/v1alpha1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	auditreglisters "k8s.io/client-go/listers/auditregistration/v1alpha1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	auditcrdv1alpha1 "github.com/pbarker/audit-lab/pkg/apis/audit/v1alpha1"
	clientset "github.com/pbarker/audit-lab/pkg/client/clientset/versioned"
	samplescheme "github.com/pbarker/audit-lab/pkg/client/clientset/versioned/scheme"
	informers "github.com/pbarker/audit-lab/pkg/client/informers/externalversions/audit/v1alpha1"
	listers "github.com/pbarker/audit-lab/pkg/client/listers/audit/v1alpha1"
)

const controllerAgentName = "audit-backend-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Backend is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Backend fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Backend"
	// MessageResourceSynced is the message used for an Event fired when a Backend
	// is synced successfully
	MessageResourceSynced = "Backend synced successfully"
)

// Controller is the controller implementation for Backend resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// auditcrdclientset is a clientset for our own API group
	auditcrdclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced

	serviceLister corelisters.ServiceLister
	serviceSynced cache.InformerSynced

	auditSinkLister auditreglisters.AuditSinkLister
	auditSinkSynced cache.InformerSynced

	auditBackendLister listers.AuditBackendLister
	auditBackendSynced cache.InformerSynced

	auditClassLister listers.AuditClassLister
	auditClassSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	auditcrdclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	serviceInformer coreinformers.ServiceInformer,
	auditSinkInformer auditreginformers.AuditSinkInformer,
	auditBackendInformer informers.AuditBackendInformer,
	auditClassInformer informers.AuditClassInformer,
) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:      kubeclientset,
		auditcrdclientset:  auditcrdclientset,
		deploymentsLister:  deploymentInformer.Lister(),
		deploymentsSynced:  deploymentInformer.Informer().HasSynced,
		serviceLister:      serviceInformer.Lister(),
		serviceSynced:      serviceInformer.Informer().HasSynced,
		auditSinkLister:    auditSinkInformer.Lister(),
		auditSinkSynced:    auditSinkInformer.Informer().HasSynced,
		auditBackendLister: auditBackendInformer.Lister(),
		auditBackendSynced: auditBackendInformer.Informer().HasSynced,
		auditClassLister:   auditClassInformer.Lister(),
		auditClassSynced:   auditClassInformer.Informer().HasSynced,
		workqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Backends"),
		recorder:           recorder,
	}

	klog.Info("Setting up event handlers")

	// Set up an event handler for when Sink resources change
	auditSinkInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newSink := new.(*auditregv1alpha1.AuditSink)
			oldSink := old.(*auditregv1alpha1.AuditSink)
			if newSink.ResourceVersion == oldSink.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	// Set up an event handler for when Backend resources change
	auditBackendInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueAuditBackend,
		UpdateFunc: func(old, new interface{}) {
			oldBackend := old.(*auditcrdv1alpha1.AuditBackend)
			newBackend := new.(*auditcrdv1alpha1.AuditBackend)
			if oldBackend.ResourceVersion == newBackend.ResourceVersion {
				return
			}
			controller.enqueueAuditBackend(new)
		},
		DeleteFunc: controller.enqueueAuditBackend,
	})

	// Set up an event handler for when Class resources change
	auditClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.handleClassDelta(obj.(*auditcrdv1alpha1.AuditClass))
		},
		UpdateFunc: func(old, new interface{}) {
			oldClass := old.(*auditcrdv1alpha1.AuditClass)
			newClass := new.(*auditcrdv1alpha1.AuditClass)
			if oldClass.ResourceVersion == newClass.ResourceVersion {
				return
			}
			controller.handleClassDelta(new.(*auditcrdv1alpha1.AuditClass))
		},
		DeleteFunc: func(obj interface{}) {
			// handle tombstones
			controller.handleClassDelta(obj.(*auditcrdv1alpha1.AuditClass))
		},
	})

	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Backend resource will enqueue that Backend resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newService := new.(*corev1.Service)
			oldService := old.(*corev1.Service)
			if newService.ResourceVersion == oldService.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Backend controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.auditBackendSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Backend resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Backend resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) handleClassDelta(class *auditcrdv1alpha1.AuditClass) {
	klog.Infof("handling class delta for %q", class.Name)
	// list all backends
	backends, err := c.auditBackendLister.AuditBackends(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		runtime.HandleError(err)
	}
	// see which ones have the class
	for _, backend := range backends {
		if hasClass(backend, class) {
			c.enqueueAuditBackend(backend)
		}
	}
}

func hasClass(backend *auditcrdv1alpha1.AuditBackend, class *auditcrdv1alpha1.AuditClass) bool {
	for _, classRule := range backend.Spec.Policy.ClassRules {
		if classRule.Name == class.Name {
			return true
		}
	}
	return false
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Backend resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	klog.Infof("syncing backend name: %q namespace: %q", name, namespace)
	// Get the Backend resource with this namespace/name
	backend, err := c.auditBackendLister.AuditBackends(namespace).Get(name)
	if err != nil {
		// If the backend no longer exists, then ensure the deployment has been deleted
		if errors.IsNotFound(err) {
			klog.Infof("backend %s/%s no longer exists, deleting any leftover resources", namespace, name)
			// Get the deployment with the name specified in Backend.spec
			deployment, err := c.deploymentsLister.Deployments(namespace).Get(name)
			// if the error is something other than not found, handle it
			if !errors.IsNotFound(err) && err != nil {
				return err
			}
			// if the deployment is not nil, delete it
			if deployment != nil {
				err := c.kubeclientset.AppsV1().Deployments(namespace).Delete(name, nil)
				if err != nil {
					return err
				}
			}
			// Get the service with the name specified in Backend.spec
			service, err := c.serviceLister.Services(namespace).Get(name)
			// if the error is something other than not found, handle it
			if !errors.IsNotFound(err) && err != nil {
				return err
			}
			// if the service is not nil, delete it
			if service != nil {
				err := c.kubeclientset.CoreV1().Services(namespace).Delete(name, nil)
				if err != nil {
					return err
				}
			}
			// Get the sink with the name specified in Backend.spec
			sink, err := c.auditSinkLister.Get(name)
			// if the error is something other than not found, handle it
			if !errors.IsNotFound(err) && err != nil {
				return err
			}
			// if the sink is not nil, delete it
			if sink != nil {
				err := c.kubeclientset.AuditregistrationV1alpha1().AuditSinks().Delete(name, nil)
				if err != nil {
					return err
				}
			}
			klog.Infof("successfully cleaned up backend %s/%s", namespace, name)
			return nil
		}
		klog.Errorf("could not get backend in sync: %v", err)
		return err
	}

	classes, err := c.auditClassLister.List(labels.Everything())
	if err != nil {
		return err
	}
	// check that classes exist for backend
	for _, classRule := range backend.Spec.Policy.ClassRules {
		if !containsClass(classes, classRule.Name) {
			klog.Infof("class %q does not exist, aborting provisioning", classRule.Name)
			c.recorder.Event(backend, corev1.EventTypeWarning, "invalid-class", fmt.Sprintf("class %q does not exist", classRule.Name))
			return nil
		}
	}

	// Get the deployment with the name specified in Backend.spec
	deployment, err := c.deploymentsLister.Deployments(backend.Namespace).Get(backend.Name)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		klog.Infof("creating deployment for backend %s/%s", namespace, name)
		deployment, err = c.kubeclientset.AppsV1().Deployments(backend.Namespace).Create(newDeployment(backend))
	} else if err == nil {
		// roll in all other cases so that config will be reloaded
		klog.Infof("rolling deployment for backend %s/%s", namespace, name)
		deployment, err = c.kubeclientset.AppsV1().Deployments(backend.Namespace).Update(newDeployment(backend))
	}
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Get the sink with the name specified in Backend.spec
	sink, err := c.auditSinkLister.Get(backend.Name)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		klog.Infof("creating sink for backend %s/%s", namespace, name)
		sink, err = c.newAuditSink(backend)
		if err != nil {
			return err
		}
		_, err = c.kubeclientset.AuditregistrationV1alpha1().AuditSinks().Create(sink)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}

	// Get the service with the name specified in Backend.spec
	service, err := c.serviceLister.Services(backend.Namespace).Get(backend.Name)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		klog.Infof("creating service for backend %s/%s", namespace, name)
		service = newService(backend)
		_, err = c.kubeclientset.CoreV1().Services(backend.Namespace).Create(service)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this Backend resource, update the deployment
	if !metav1.IsControlledBy(deployment, backend) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(backend.Namespace).Update(newDeployment(backend))
		if err != nil {
			return err
		}
	}

	// If the Sink is not controlled by this Backend resource, update the deployment
	if !metav1.IsControlledBy(sink, backend) {
		sink, err = c.newAuditSink(backend)
		if err != nil {
			return err
		}
		sink, err = c.kubeclientset.AuditregistrationV1alpha1().AuditSinks().Update(sink)
		if err != nil {
			return err
		}
	}
	// If the Service is not controlled by this Backend resource, update the deployment
	if !metav1.IsControlledBy(service, backend) {
		service, err = c.kubeclientset.CoreV1().Services(backend.Namespace).Update(newService(backend))
		if err != nil {
			return err
		}
	}

	// Finally, we update the status block of the Backend resource to reflect the
	// current state of the world
	err = c.updateStatus(backend, deployment)
	if err != nil {
		return err
	}

	klog.Infof("update complete for backend %s/%s", namespace, name)
	c.recorder.Event(backend, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func containsClass(classes []*auditcrdv1alpha1.AuditClass, className string) bool {
	for _, class := range classes {
		if class.Name == className {
			return true
		}
	}
	return false
}

func (c *Controller) updateStatus(backend *auditcrdv1alpha1.AuditBackend, deployment *appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	backendCopy := backend.DeepCopy()
	// backendCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Backend resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.auditcrdclientset.AuditV1alpha1().AuditBackends(backend.Namespace).Update(backendCopy)
	return err
}

// enqueueAuditBackend takes a Backend resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Backend.
func (c *Controller) enqueueAuditBackend(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Backend resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Backend resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Backend, we should not do anything more
		// with it.
		if ownerRef.Kind != "Backend" {
			return
		}

		b, err := c.auditBackendLister.AuditBackends(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of Backend '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueAuditBackend(b)
		return
	}
}

func (c *Controller) newAuditSink(backend *auditcrdv1alpha1.AuditBackend) (*auditregv1alpha1.AuditSink, error) {
	// export CA_BUNDLE=$(kubectl get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' | base64 | tr -d '\n')

	// needd to grab the CABundle stored in kube-system -- could this be retrieved earlier?
	cm, err := c.kubeclientset.CoreV1().ConfigMaps("kube-system").Get("extension-apiserver-authentication", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	cafile := cm.Data["client-ca-file"]
	return &auditregv1alpha1.AuditSink{
		ObjectMeta: metav1.ObjectMeta{
			Name: backend.Name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(backend, schema.GroupVersionKind{
					Group:   auditcrdv1alpha1.SchemeGroupVersion.Group,
					Version: auditcrdv1alpha1.SchemeGroupVersion.Version,
					Kind:    "AuditBackend",
				}),
			},
		},
		Spec: auditregv1alpha1.AuditSinkSpec{
			Policy: auditregv1alpha1.Policy{
				Level: auditregv1alpha1.LevelRequestResponse,
				Stages: []auditregv1alpha1.Stage{
					auditregv1alpha1.StageRequestReceived,
					auditregv1alpha1.StageResponseStarted,
					auditregv1alpha1.StageResponseComplete,
					auditregv1alpha1.StagePanic,
				},
			},
			Webhook: auditregv1alpha1.Webhook{
				ClientConfig: auditregv1alpha1.WebhookClientConfig{
					Service: &auditregv1alpha1.ServiceReference{
						Name:      backend.Name,
						Namespace: backend.Namespace,
					},
					CABundle: []byte(cafile),
				},
			},
		},
	}, nil
}

func newService(backend *auditcrdv1alpha1.AuditBackend) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backend.Name,
			Namespace: backend.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(backend, schema.GroupVersionKind{
					Group:   auditcrdv1alpha1.SchemeGroupVersion.Group,
					Version: auditcrdv1alpha1.SchemeGroupVersion.Version,
					Kind:    "AuditBackend",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"controller": backend.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       int32(443),
					TargetPort: intstr.IntOrString{IntVal: 443},
				},
			},
		},
	}
}

// newDeployment creates a new Deployment for a Backend resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Backend resource that 'owns' it.
func newDeployment(backend *auditcrdv1alpha1.AuditBackend) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "audit-proxy",
		"controller": backend.Name,
	}
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backend.Name,
			Namespace: backend.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(backend, schema.GroupVersionKind{
					Group:   auditcrdv1alpha1.SchemeGroupVersion.Group,
					Version: auditcrdv1alpha1.SchemeGroupVersion.Version,
					Kind:    "AuditBackend",
				}),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:    "audit-proxy-init",
							Image:   "aunem/audit-proxy-init:latest",
							Command: []string{"sh", "/create-cert"},
							Args:    []string{"--service", backend.Name, "--secret", backend.Name, "--namespace", backend.Namespace},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "audit-proxy",
							Image: "aunem/audit-proxy:latest",
							Args:  []string{"--backendname", backend.Name, "--backendnamespace", backend.Namespace},
							VolumeMounts: []corev1.VolumeMount{
								{
									ReadOnly:  true,
									Name:      "certs",
									MountPath: "/etc/webhook/certs",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "delta",
									Value: fmt.Sprintf("%d", time.Now().Unix()),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: backend.Name,
								},
							},
						},
					},
				},
			},
		},
	}
}
