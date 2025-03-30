# Connectivity operator package

This repo contains the source code and instructions to build the connectivity operator. Note that it is not intended to
work as a go project by itself, so if you try to open it in GoLand in its current form, it will show errors.


## Prerequisites

* `sed`
* `go`
* `docker`
* `kubebuilder`

```
curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
chmod +x kubebuilder && mv kubebuilder /usr/local/bin/
```


### My config

```
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube_latest_amd64.deb
sudo dpkg -i minikube_latest_amd64.deb
```

I am using `go latest` and `minikube` for kubernetes. I assume it should be working just OK in other kubernetes environments,
but the install and run instructions will need to be replaced and modified accordingly.




## Install and run

* Install the listener first from https://github.com/khzs/connectivity-listener

* Clone this repo

* Execute the following:


```
minikube start
alias kubectl='minikube kubectl --'

mkdir k8s-connectivity-operator
cd k8s-connectivity-operator

go mod init github.com/khzs/k8s-connectivity-operator
kubebuilder init --repo github.com/khzs/k8s-connectivity-operator
kubebuilder create api --group monitoring --version v1 --kind Connectivity

sed -i 's|^KUBECTL ?= kubectl$|KUBECTL ?= minikube kubectl --|' Makefile
sed -i 's|golang:[^ ]*|golang:latest|' Dockerfile

cp -r ../copystack/* ./
go mod tidy


# CRD
make manifests
kubectl apply -f config/crd/bases/monitoring.my.domain_connectivities.yaml

# operator image
make docker-build IMG=connectivity-operator:latest
minikube image load connectivity-operator:latest
make deploy IMG=connectivity-operator:latest
kubectl patch deployment k8s-connectivity-operator-controller-manager -n k8s-connectivity-operator-system --type json -p '[{"op": "replace", "path": "/spec/template/spec/containers/0/imagePullPolicy", "value": "IfNotPresent"}]'

# Connectivity resource
kubectl apply -f ../sample-connectivity.yaml

# Check output
kubectl get pods -A
kubectl logs -n k8s-connectivity-operator-system deployment/k8s-connectivity-operator-controller-manager -c manager
kubectl get connectivity sample-connectivity -o yaml
```
