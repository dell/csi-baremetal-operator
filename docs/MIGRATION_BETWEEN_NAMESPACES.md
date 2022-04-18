Manual CSI migration into another namespace
---------------------
It may happen that CSI charts (including pods and configmaps) should be moved into another default or non-default namespace. 
`helm upgrade` is not suitable in this case due to kubernetes objects fields restrictions.
So we need to delete and install CSI charts, which is described in the steps below.

1. Delete CSI Operator and CSI Deployment charts without removing any CRs and CRDs
```yaml
helm delete csi-baremetal
helm delete csi-baremetal-operator
```
2. Reinstall CSI Operator and CSI Deployment charts into another namespace
```yaml
helm install csi-baremetal-operator <chart_path> -n $NAMESPACE <other_args>
helm install csi-baremetal <chart_path> -n $NAMESPACE <other_args>
```
3. Wait for all pods to be ready
```yaml
watch kubectl get po -n $NAMESPACE -l app=csi-baremetal
```
4. Remove unrelated resources
```yaml
kubectl get volumeattachments | grep csi-baremetal | awk '{print $1}' | xargs kubectl delete volumeattachments
```
