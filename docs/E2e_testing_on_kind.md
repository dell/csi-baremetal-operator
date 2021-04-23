CSI Baremetal operator e2e testing
---------------------

Last update: 19.04.2021

**CSI images has to be pre-loaded to kind cluster**

1. Set CSI version

```
export CSI_VERSION=<csi version>
```

2. Build operator image and load to kind
    
```
make docker-build
make kind-load
```

3. Set Operator version

```
export OPERATOR_VERSION=<operator version>
```

4. Run e2e testing

```
make test-ci
```
