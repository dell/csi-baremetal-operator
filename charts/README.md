Deploy CSI Baremetal to kind-cluster using operator
---------------------

Last update: 26.03.2021

! Kind-cluster has pre-loaded csi images

1. Set CSI version

```
export csiVersion=...
```

2. Build operator image and load to kind
    
```
make docker-build
make kind-load
```

3. Deploy operator

```
helm install csi-baremetal-operator ./charts/csi-baremetal-operator/
```

4. Deploy csi-baremetal

```
helm install csi-baremetal ./charts/csi-baremetal-deployment/ --set image.tag=${csiVersion} --set image.pullPolicy=IfNotPresent --set driver.drivemgr.type=loopbackmgr --set driver.drivemgr.deployConfig=true --set scheduler.patcher.enable=true
```
