[![PR validation](https://github.com/dell/csi-baremetal-operator/actions/workflows/pr.yml/badge.svg)](https://github.com/dell/csi-baremetal-operator/actions/workflows/pr.yml)
[![codecov](https://codecov.io/gh/dell/csi-baremetal-operator/branch/master/graph/badge.svg)](https://codecov.io/gh/dell/csi-baremetal-operator)

Bare-metal CSI Operator
=====================

Kubernetes Operator to deploy and manage lifecycle of [Bare-Metal CSI Driver](https://github.com/dell/csi-baremetal)

Installation process
---------------------

* Add helm repository
```
helm repo add csi https://dell.github.io/csi-baremetal-operator
helm repo update
helm search repo csi --devel -l
```
* Install CSI Operator
```
helm install csi-baremetal-operator csi/csi-baremetal-operator --devel 
```
* Install CSI     
    * Vanilla Kubernetes
        ```
        helm install csi-baremetal csi/csi-baremetal-deployment --devel --set scheduler.patcher.enable=true
        ```
    * RKE2
        ```
        helm install csi-baremetal csi/csi-baremetal-deployment --devel --set scheduler.patcher.enable=true --set platform=rke
        ```
    * OpenShift
      ```
      helm install csi-baremetal csi/csi-baremetal-deployment --devel --set scheduler.patcher.enable=true --set platform=openshift
      ```
    * Not supported platform or system with third party Kubernetes scheduler extender - refer [documentation](MANUAL_SCHEDULER_CONFIGURATION.md) for manual patching of Kubernetes scheduler configuration
      ```
      helm install csi-baremetal csi/csi-baremetal-deployment --devel
      ```
        
Uninstallation process
---------------------
* Delete custom resources
```
kubectl delete pvc --all
kubectl delete volumes --all
kubectl delete lvgs --all
kubectl delete csibmnodes --all
```
* Delete helm releases
```
helm delete csi-baremetal
helm delete csi-baremetal-operator
```
* Delete custom resource definitions
```
kubectl delete crd deployments.csi-baremetal.dell.com availablecapacities.csi-baremetal.dell.com availablecapacityreservations.csi-baremetal.dell.com logicalvolumegroups.csi-baremetal.dell.com volumes.csi-baremetal.dell.com drives.csi-baremetal.dell.com nodes.csi-baremetal.dell.com
```



