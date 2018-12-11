# Audit Lab

The Audit Lab holds prototypes for kubernetes audit upstream enhancements

## Creating a Test Cluster
```term
go get -d k8s.io/kubernetes
cd $(go env GOPATH)/src/k8s.io/kubernetes
git checkout release-1.13
git fetch && git merge --ff-only
cd -
kind build node-image
kind create cluster --image kindest/node:latest --config kind-config.yaml
export KUBECONFIG="$(kind get kubeconfig-path --name="1")"
```