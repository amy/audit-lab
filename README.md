# Audit Lab

The Audit Lab holds prototypes for kubernetes audit upstream enhancements

## Audit Classes
Audit Classes are a prototype that aims to make policy more composable.

The [AuditClass object](config/install/crds/audit_v1alpha1_auditclass.yaml) classifies requests. Then the 
[AuditBackend object](config/install/crds/audit_v1alpha1_auditbackend.yaml), which is a superset of the 
[AuditSink object](https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/api/auditregistration/v1alpha1/types.go#L65) provides a means 
of applying rules to the audit classes. 

### Creating a Test Cluster
```term
go get -d k8s.io/kubernetes
cd $(go env GOPATH)/src/k8s.io/kubernetes
git checkout release-1.13
cd -
kind build node-image
kind create cluster --image kindest/node:latest --config kind-config.yaml
export KUBECONFIG="$(kind get kubeconfig-path --name="1")"
```

### Installing the Controller
```term
kubectl apply -Rf config/install/
```

### Creating Classes and Backends
Create a sample class
```term
kubectl apply -f config/samples/classes/
```

Create a sample backend
```term
kubectl apply -f config/samples/audit_v1alpha1_auditbackend.yaml
```

You can then view the container running in the `audit` namespace
```term
kubectl -n audit get po
```

Tail the logs of the container to see the audit logs printed. Change up the class or backend to see 
how it affects the output.

### Removing Backends
```term
kubectl -n audit delete auditbackend sample
```