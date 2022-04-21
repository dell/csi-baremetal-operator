How to enable monitoring of CSI pods on OpenShift
---------------------
Basic doc is [here](https://docs.openshift.com/container-platform/4.6/monitoring/configuring-the-monitoring-stack.html)

Necessary cmds:
```
oc apply -f deploy/monitoring/monitoring-configmap.yaml
oc apply -f deploy/monitoring/csi-pod-monitor.yaml
```