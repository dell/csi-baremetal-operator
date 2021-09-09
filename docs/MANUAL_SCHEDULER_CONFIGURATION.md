Manual Kubernetes Scheduler Configuration
---------------------
To make CSI work correctly we maintain Kubernetes scheduler extender and scheduler extender patcher components.
If your Kubernetes distributive is not in the list of supported or you have third party scheduler extender deployed the
following manual steps are required
* Vanilla Kubernetes
    * Version 1.18 and below
    * Version 1.19 and above
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
