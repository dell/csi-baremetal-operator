[![PR validation](https://github.com/dell/csi-baremetal-operator/actions/workflows/pr.yml/badge.svg)](https://github.com/dell/csi-baremetal-operator/actions/workflows/pr.yml)
[![codecov](https://codecov.io/gh/dell/csi-baremetal-operator/branch/master/graph/badge.svg)](https://codecov.io/gh/dell/csi-baremetal-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/dell/csi-baremetal-operator)](https://goreportcard.com/report/github.com/dell/csi-baremetal-operator)
![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.18--v1.23-success?logo=kubernetes&logoColor=lightgrey)

Bare-metal CSI Operator
=====================

Kubernetes Operator to deploy and manage lifecycle of [Bare-Metal CSI Driver](https://github.com/dell/csi-baremetal)

Supported environments
----------------------
- **OpenShift**: 4.6
- **RKE2r1**: 1.21-1.23
- **Node OS**:
  - Ubuntu 18.04 / 20.04 LTS
  - Red Hat Enterprise Linux 7.7 / 8.x
  - Red Hat Enterprise Linux CoreOS 4.6
  - CentOS Linux 7 / 8
- **Helm**: 3.0

Installation process
---------------------
* Prerequisites
    * *lvm2* packet installed on the Kubernetes nodes
    * *helm v3+*
    * *kubectl v1.16+*

* Add helm repository
    ```shell script
    helm repo add csi https://dell.github.io/csi-baremetal-operator
    helm repo update
    helm search repo csi -l
    ```
* Setup environment variables
    ```shell script
    export REGISTRY=docker.io/objectscale
    export DOCKER_REGISTRY_SECRET=dockerhub-pull-secret
    ```
* Create docker registry secret
    ```shell script
    kubectl create secret docker-registry $DOCKER_REGISTRY_SECRET --docker-username=<USER NAME> \
  --docker-password=<PASSWORD> --docker-email=<EMAIL>
    ```
* Install CSI Operator
    ```shell script
    helm install csi-baremetal-operator csi/csi-baremetal-operator --set global.registry=$REGISTRY \
  --set global.registrySecret=$DOCKER_REGISTRY_SECRET
    ```
* Install CSI
    * Vanilla Kubernetes
        ```
        helm install csi-baremetal csi/csi-baremetal-deployment --set scheduler.patcher.enable=true \
      --set driver.drivemgr.type=halmgr --set global.registry=$REGISTRY --set global.registrySecret=$DOCKER_REGISTRY_SECRET
        ```
    * RKE2
        ```
        helm install csi-baremetal csi/csi-baremetal-deployment --set scheduler.patcher.enable=true \
      --set driver.drivemgr.type=halmgr --set platform=rke --set global.registry=$REGISTRY \
      --set global.registrySecret=$DOCKER_REGISTRY_SECRET
        ```
    * OpenShift
        ```
        helm install csi-baremetal csi/csi-baremetal-deployment --set scheduler.patcher.enable=true \
      --set driver.drivemgr.type=halmgr --set platform=openshift --set global.registry=$REGISTRY \
      --set global.registrySecret=$DOCKER_REGISTRY_SECRET
        ```
      **Note:** In case of CSI deployment at Openshift platform in non default namespace, CSI ServiceAccounts \
      csi-node-sa and csi-baremetal-extender-sa will require Privileged SCC. In case there are no such permissions \
      CSI Operator will not deploy node and scheduler extender daemonsets. You can see the following events in such case:
      ```
      65m         Warning   SecurityContextConstraintsVerificationFailed       deployment/csi-baremetal                              ServiceAccount csi-node-sa has insufficient securityContextConstraints, should have privileged
      65m         Warning   SecurityContextConstraintsVerificationFailed       deployment/csi-baremetal                              ServiceAccount csi-baremetal-extender-sa has insufficient securityContextConstraints, should have privileged
      ```
      To make them deployable - create the following role and rolebinding in deployed namespace:
      ```yaml
      apiVersion: rbac.authorization.k8s.io/v1
      kind: Role
      metadata:
        name: $ROLENAME
        namespace: $NAMESPACE
      rules:
      - apiGroups:
        - security.openshift.io
        resourceNames:
        - privileged
        resources:
        - securitycontextconstraints
        verbs:
        - use
      ```
      ```yaml
      apiVersion: rbac.authorization.k8s.io/v1
      kind: RoleBinding
      metadata:
        name: role-binding
        namespace: $NAMESPACE
      roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: Role
        name: $ROLENAME
      subjects:
      - kind: ServiceAccount
        name: csi-baremetal-extender-sa
        namespace: $NAMESPACE
      - kind: ServiceAccount
        name: csi-node-sa
        namespace: $NAMESPACE
      ```
      After created the below roles and rolebindings CSI Operator will automatically catch the changes and deploy
      node and scheduler extender daemonsets.
    * [Kind](https://kind.sigs.k8s.io/) (for testing purposes only)
      ```
      helm install csi-baremetal csi/csi-baremetal-deployment --set scheduler.patcher.enable=true \
      --set driver.drivemgr.type=loopbackmgr --set driver.drivemgr.deployConfig=true --set global.registry=$REGISTRY \
      --set global.registrySecret=$DOCKER_REGISTRY_SECRET
      ```
    * [K3S](https://k3s.io/)
      ```
      helm install csi-baremetal csi/csi-baremetal-deployment --set driver.drivemgr.type=halmgr \
      --set global.registry=$REGISTRY --set global.registrySecret=$DOCKER_REGISTRY_SECRET
      ```
      **Note:** Currently supports only manual patching -  refer [documentation](MANUAL_SCHEDULER_CONFIGURATION.md) 

    * Not supported platform or system with third party Kubernetes scheduler extender - refer [documentation](MANUAL_SCHEDULER_CONFIGURATION.md) for manual patching of Kubernetes scheduler configuration
      ```
      helm install csi-baremetal csi/csi-baremetal-deployment --set driver.drivemgr.type=halmgr \
      --set global.registry=$REGISTRY --set global.registrySecret=$DOCKER_REGISTRY_SECRET
      ```
Feature Supporting
------
### CIS hardening
For supporting CIS Hardening we need to add the following settings while installing CSI and CSI Baremetal Operator:
* While installing CSI Baremetal Operator set SecurityContext:
  ```
  --set securityContext.enable=true --set securityContext.runAsNonRoot=true --set securityContext.runAsUser=1000
  ```
* While installing CSI:
  * Set the following SecurityContexts at deployments.csi-baremetal.dell.com for Controller, NodeController components:
    ```
    --set driver.controller.securityContext.enable=true --set driver.controller.securityContext.runAsNonRoot=true --set driver.controller.securityContext.runAsUser=1000 \
    --set driver.securityContext.securityContext.enable=true --set driver.securityContext.securityContext.runAsNonRoot=true --set driver.securityContext.securityContext.runAsUser=1000
    ```
  * Create (if not existed) privileged PodSecurityPolicy:
    ```
    apiVersion: policy/v1beta1
    kind: PodSecurityPolicy
    metadata:
      name: privileged
      annotations:
        seccomp.security.alpha.kubernetes.io/allowedProfileNames: '*'
    spec:
      privileged: true
      allowPrivilegeEscalation: true
      allowedCapabilities:
      - '*'
      volumes:
      - '*'
      hostNetwork: true
      hostPorts:
      - min: 0
        max: 65535
      hostIPC: true
      hostPID: true
      runAsUser:
        rule: 'RunAsAny'
      seLinux:
        rule: 'RunAsAny'
      supplementalGroups:
        rule: 'RunAsAny'
      fsGroup:
        rule: 'RunAsAny'
    ```
  * Create Role for this policy usage:
    ```
    apiVersion: rbac.authorization.k8s.io/v1
    kind: Role
    metadata:
      name: manager-role
      namespace: test-namespace
    rules:
    - apiGroups:
      - policy
        resourceNames:
      - privileged
        resources:
      - podsecuritypolicies
        verbs:
      - use
    ```
  * Create RoleBinding for this policy usage:
    ```
    apiVersion: rbac.authorization.k8s.io/v1
    kind: RoleBinding
    metadata:
      name: manager-rolebinding
      namespace: test-namespace
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: Role
      name: manager-role
    subjects:
    - kind: ServiceAccount
      name: csi-node-sa
      namespace: test-namespace
    - kind: ServiceAccount
      name: csi-baremetal-extender-sa
      namespace: test-namespace
    ```
  * Set the following SecurityContexts at deployments.csi-baremetal.dell.com for SchedulerExtender, SchedulerExtenderPatcher components:
    ```
    --set scheduler.securityContext.enable=true --set scheduler.securityContext.privileged=true
    ```
  * Enable podSecurityPolicy at deployments.csi-baremetal.dell.com for Controller, Node, SchedulerExtender, SchedulerExtenderPatcher components:
    ```
    --set driver.controller.securityContext.enable=true --set driver.controller.securityContext.runAsNonRoot=true --set driver.controller.securityContext.runAsUser=1000 \
    --set driver.node.podSecurityPolicy.enable=true --set driver.node.podSecurityPolicy.resourceName=privileged \
    --set scheduler.podSecurityPolicy.enable=true --set scheduler.podSecurityPolicy.resourceName=privileged
    ```
Usage
------

* Storage classes

    * Use storage class without `lvg` postfix if you need to provision PV bypassing LVM. Size of the resulting PV will
    be equal to the size of underlying physical drive.

    * Use storage class with `lvg` postfix if you need to provision PVC based on the logical volume. Size of the
    resulting PV will be equal to the size of PVC.

* To obtain information about:

    * Node IDs assigned by CSI - `kubectl get nodes.csi-baremetal.dell.com`

    * Local Drives discovered by CSI - `kubectl get drives.csi-baremetal.dell.com`

    * Capacity available for allocation - `kubectl get  availablecapacities.csi-baremetal.dell.com`

    * Provisioned logical volume groups - `kubectl get logicalvolumegroups.csi-baremetal.dell.com`

    * Provisioned volumes - `kubectl get volumes.csi-baremetal.dell.com`

Upgrade process
---------------------
To upgrade please reference _Installation process_ section but replace `helm install` by `helm upgrade` command
 
Uninstallation process
---------------------
* Delete custom resources
    ```
    kubectl delete pvc --all
    kubectl delete volumes --all -A
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
    kubectl delete crd deployments.csi-baremetal.dell.com availablecapacities.csi-baremetal.dell.com \
  availablecapacityreservations.csi-baremetal.dell.com logicalvolumegroups.csi-baremetal.dell.com \
  volumes.csi-baremetal.dell.com drives.csi-baremetal.dell.com nodes.csi-baremetal.dell.com
    ```
