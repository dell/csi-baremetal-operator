CSI Baremetal operator e2e testing
---------------------

Last update: 19.04.2021

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

3. Set Operator version

```
export operatorVersion=...
```

4. Copy actual charts from csi-baremetal-operator repo

```
cp -r ./charts /tmp/charts
```

5. Run e2e testing

```
CI=true go test -v test/e2e/csi_operator_e2e_test.go -ginkgo.v -ginkgo.progress -kubeconfig={$HOME}/.kube/config -operatorVersion={$operatorVersion} -csiVersion={$csiVersion}
```
