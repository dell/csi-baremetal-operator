Manual Kubernetes Scheduler Configuration
---------------------
To make CSI work correctly we maintain Kubernetes scheduler extender and scheduler extender patcher components.
If your Kubernetes distributive is not in the list of supported or you have third party scheduler extender deployed the
following manual steps are required
* Vanilla Kubernetes
    * If you have third party scheduler extender deployed just add the following sections to your configuration file
    ```
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
        ```
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
        ```
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
            ```
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
            ```
            volumeMounts:
            - mountPath: /etc/kubernetes/manifests/scheduler/config.yaml
              name: scheduler-config
              readOnly: true
            - mountPath: /etc/kubernetes/manifests/scheduler/policy.yaml
              name: scheduler-policy
              readOnly: true
            ```
            * Command parameter
            ```
            spec:
              containers:
              - command:
               - kube-scheduler
               ...
               - --config=/etc/kubernetes/manifests/scheduler/config.yaml
            ```
    * To manually patch version 1.19 and above
        * Create configuration file _/etc/kubernetes/manifests/scheduler/config-19.yaml_
        ```
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
            ```
            volumes:
            - hostPath:
                path: /etc/kubernetes/manifests/scheduler/config-19.yaml
                type: File
              name: scheduler-config-19
            ```
            * Volume mount
            ```
            volumeMounts:
            - mountPath: /etc/kubernetes/manifests/scheduler/config-19.yaml
              name: scheduler-config-19
              readOnly: true
            ```
            * Command parameter
            ```
            spec:
              containers:
              - command:
               - kube-scheduler
               ...
               - --config=/etc/kubernetes/manifests/scheduler/config-19.yaml
            ```
* RKE2
    
* OpenShift

* Other
