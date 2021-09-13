Manual Kubernetes Scheduler Configuration
---------------------
To make CSI work correctly we maintain Kubernetes scheduler extender and scheduler extender patcher components.
If your Kubernetes distributive is not in the list of supported or you have third party scheduler extender deployed the
following manual steps are required on all master nodes (except OpenShift). Note that user must wait for all scheduler
pods to restart after configuration change.
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
    * To manually patch version 1.18 and below
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
    * To manually patch version 1.19 and above
        * Create configuration file _/etc/kubernetes/manifests/scheduler/config-19.yaml_
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
                path: /etc/kubernetes/manifests/scheduler/config-19.yaml
                type: File
              name: scheduler-config-19
            ```
            * Volume mount
            ```yaml
            volumeMounts:
            - mountPath: /etc/kubernetes/manifests/scheduler/config-19.yaml
              name: scheduler-config-19
              readOnly: true
            ```
            * Command parameter
            ```yaml
            spec:
              containers:
              - command:
               - kube-scheduler
               ...
               - --config=/etc/kubernetes/manifests/scheduler/config-19.yaml
            ```
* RKE2
    * Follow instructions for vanilla Kubernetes, but use the following path to the scheduler configuration file
    `/var/lib/rancher/rke2/agent/pod-manifests/`
    
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
