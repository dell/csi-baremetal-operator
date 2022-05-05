Manual Kubernetes Scheduler Configuration
---------------------
To make CSI work correctly we maintain Kubernetes scheduler extender and scheduler extender patcher components.
If your Kubernetes distributive is not in the list of supported or you have third party scheduler extender deployed the
following manual steps are required. Note that user must wait for all scheduler pods to restart after configuration change.
* Vanilla Kubernetes
    * If you have third party scheduler extender deployed add the following sections to your configuration file
    ```yaml
    extenders:
    ...
      - urlPrefix: "http://127.0.0.1:8889"
        filterVerb: filter
        prioritizeVerb: prioritize
        weight: 1
        enableHTTPS: false
        nodeCacheCapable: false
        ignorable: true
        httpTimeout: 15s
    ```
    * To manually patch version 1.18 and below perform the following steps on all master nodes of your cluster
        * Create configuration policy file _/etc/kubernetes/manifests/scheduler/policy.yaml_
        ```yaml
        apiVersion: v1
        kind: Policy
        extenders:
          - urlPrefix: "http://127.0.0.1:8889"
            filterVerb: filter
            prioritizeVerb: prioritize
            weight: 1
            enableHttps: false
            nodeCacheCapable: false
            ignorable: true
            httpTimeout: 15000000000
        ```
        * Create configuration file _/etc/kubernetes/manifests/scheduler/config.yaml_
        ```yaml
        apiVersion: kubescheduler.config.k8s.io/v1alpha1
        kind: KubeSchedulerConfiguration
        schedulerName: default-scheduler
        algorithmSource:
          policy:
            file:
              path: /etc/kubernetes/manifests/scheduler/policy.yaml
        leaderElection:
          leaderElect: true
        clientConnection:
          kubeconfig: /etc/kubernetes/scheduler.conf
        ```
        *  Add the following sections to the _/etc/kubernetes/manifests/kube-scheduler.yaml_ configuration file
            * Volumes
            ```yaml
            volumes:
            - hostPath:
                path: /etc/kubernetes/manifests/scheduler/config.yaml
                type: File
              name: scheduler-config
            - hostPath:
                path: /etc/kubernetes/manifests/scheduler/policy.yaml
                type: File
              name: scheduler-policy
            ```
            * Volume mounts
            ```yaml
            volumeMounts:
            - mountPath: /etc/kubernetes/manifests/scheduler/config.yaml
              name: scheduler-config
              readOnly: true
            - mountPath: /etc/kubernetes/manifests/scheduler/policy.yaml
              name: scheduler-policy
              readOnly: true
            ```
            * Command parameter
            ```yaml
            spec:
              containers:
              - command:
               - kube-scheduler
               ...
               - --config=/etc/kubernetes/manifests/scheduler/config.yaml
            ```
    * To manually patch version 1.19-22 perform the following steps on all master nodes of your cluster
        * Create configuration file _/etc/kubernetes/manifests/scheduler/config.yaml_
        ```yaml
        apiVersion: kubescheduler.config.k8s.io/v1beta1
        kind: KubeSchedulerConfiguration
        extenders:
          - urlPrefix: "http://127.0.0.1:8889"
            filterVerb: filter
            prioritizeVerb: prioritize
            weight: 1
            enableHTTPS: false
            nodeCacheCapable: false
            ignorable: true
            httpTimeout: 15s
        leaderElection:
          leaderElect: true
        clientConnection:
          kubeconfig: /etc/kubernetes/scheduler.conf
        ```
        *  Add the following sections to the _/etc/kubernetes/manifests/kube-scheduler.yaml_ configuration file
            * Volume
            ```yaml
            volumes:
            - hostPath:
                path: /etc/kubernetes/manifests/scheduler/config.yaml
                type: File
              name: scheduler-config
            ```
            * Volume mount
            ```yaml
            volumeMounts:
            - mountPath: /etc/kubernetes/manifests/scheduler/config.yaml
              name: scheduler-config
              readOnly: true
            ```
            * Command parameter
            ```yaml
            spec:
              containers:
              - command:
               - kube-scheduler
               ...
               - --config=/etc/kubernetes/manifests/scheduler/config.yaml
            ```
    * To manually patch version 1.23 and above use new _apiVersion_ `kubescheduler.config.k8s.io/v1beta3` in the configuration manifest
* RKE2
    * Follow instructions for vanilla Kubernetes, but use the following path to the scheduler configuration file
    `/var/lib/rancher/rke2/agent/pod-manifests/`

* K3S
    * For manual patching follow the instructions below on all master nodes
      * Create next configuration file in directory `/var/lib/rancher/k3s/agent/pod-manifests/scheduler`:
        
        ```yaml
        apiVersion: kubescheduler.config.k8s.io/v1beta1
        kind: KubeSchedulerConfiguration
        extenders:
          - urlPrefix: "http://127.0.0.1:8889"
            filterVerb: filter
            prioritizeVerb: prioritize
            weight: 1
            enableHTTPS: false
            nodeCacheCapable: false
            ignorable: true
            httpTimeout: 15s
        leaderElection:
          leaderElect: true
        clientConnection:
          kubeconfig: /var/lib/rancher/k3s/server/cred/scheduler.kubeconfig
        ```

      *  Modify  `/etc/systemd/system/k3s.service`: add at the end of the option `ExecStart` next string `--kube-scheduler-arg=config=/var/lib/rancher/k3s/agent/pod-manifests/scheduler/config.yaml`. Service unit will look the same way:
          ```
            [Service]
            . . .
            ExecStartPre=/bin/sh -xc '! /usr/bin/systemctl is-enabled --quiet nm-cloud-setup.service'
            ExecStartPre=-/sbin/modprobe br_netfilter
            ExecStartPre=-/sbin/modprobe overlay
            ExecStart=/usr/local/k3s \
                service \  
                --kube-scheduler-arg=config=/var/lib/rancher/k3s/agent/pod-manifests/scheduler/config.yaml 
          ```

      * Manually restart k3s service with new parameter `systemctl daemon-reload && systemctl restart k3s` .

    * To manually patch version 1.23 and above use `kubescheduler.config.k8s.io/v1beta3` in the configuration manifest

    * For uninstall need to return service file to it's previous state. Delete `--kube-scheduler-arg` and restart service to apply changes.      

* OpenShift
    * If you have third party scheduler extender deployed add the following section to the config map specified in the
    cluster custom resource of scheduler CRD `oc describe scheduler cluster`
    ```json
    {
      "urlPrefix": "http://127.0.0.1:8889",
      "filterVerb": "filter",
      "prioritizeVerb": "prioritize",
      "weight": 1,
      "enableHttps": false,
      "nodeCacheCapable": false,
      "ignorable": true
    }
    ```
    * Create _policy.cfg_ file with following content:
    ```json
    {
      "kind" : "Policy",
      "apiVersion" : "v1",
      "extenders": [
        {
            "urlPrefix": "http://127.0.0.1:8889",
            "filterVerb": "filter",
            "prioritizeVerb": "prioritize",
            "weight": 1,
            "enableHttps": false,
            "nodeCacheCapable": false,
            "ignorable": true
        }
      ]
    }
    ```
    * Create _scheduler-policy_ config map in _openshift-config_ namespace:
    ```shell script
    oc create configmap -n openshift-config --from-file=policy.cfg scheduler-policy
    ```
    * Patch scheduler
    ```shell script
    oc patch scheduler cluster -p '{"spec":{"policy":{"name":"scheduler-policy"}}}' --type=merge
    ```

* Other
    * Follow instructions provided by vendor. Use the following parameters:
    ```yaml
    extenders:
    ...
      - urlPrefix: "http://127.0.0.1:8889"
        filterVerb: filter
        prioritizeVerb: prioritize
        weight: 1
        enableHTTPS: false
        nodeCacheCapable: false
        ignorable: true
        httpTimeout: 15s
    ```
