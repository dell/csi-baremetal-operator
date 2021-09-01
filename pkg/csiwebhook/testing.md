```bash
# Deploy Operator and CSI
helm install csi-baremetal-operator ./charts/csi-baremetal-operator/ --set image.tag=0.4.0-49.ca6cbe4 --set image.pullPolicy=IfNotPresent
NAME: csi-baremetal-operator
LAST DEPLOYED: Wed Sep 1 10:58:37 2021
NAMESPACE: default
STATUS: deployed
REVISION: 1

helm install csi-baremetal ./charts/csi-baremetal-deployment/ --set image.tag=0.4.0-440.fb41e71 --set image.pullPolicy=IfNotPresent --set scheduler.patcher.enable=true --set driver.drivemgr.type=loopbackmgr --set driver.drivemgr.deployConfig=true --set driver.log.level=debug --set scheduler.log.level=debug --set nodeController.log.level=debug
NAME: csi-baremetal
LAST DEPLOYED: Wed Sep 1 11:00:47 2021
NAMESPACE: default
STATUS: deployed
REVISION: 1

# We can't install new csi-baremetal due to same names
helm install csi-baremetal2 ./csi-baremetal-operator/charts/csi-baremetal-deployment/ --set image.tag=0.4.0-440.fb41e71 --set image.pullPolicy=IfNotPresent --set scheduler.patcher.enable=true --set driver.drivemgr.type=loopbackmgr --set driver.drivemgr.deployConfig=true --set driver.log.level=debug --set scheduler.log.level=debug --set nodeController.log.level=debug
Error: rendered manifests contain a resource that already exists. Unable to continue with install: ServiceAccount "csi-controller-sa" in namespace "default" exists and cannot be imported into the current release: invalid ownership metadata; annotation validation error: key "meta.helm.sh/release-name" must equal "csi-baremetal2": current value is "csi-baremetal"

# Save chart as template
helm template csi-baremetal2 --output-dir=csi-templ ./csi-baremetal-operator/charts/csi-baremetal-deployment/ --set image.tag=0.4.0-440.fb41e71 --set image.pullPolicy=IfNotPresent --set scheduler.patcher.enable=true --set driver.drivemgr.type=loopbackmgr --set driver.drivemgr.deployConfig=true --set driver.log.level=debug --set scheduler.log.level=debug --set nodeController.log.level=debug
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/csibm-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/node-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/scheduler-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/configmap/loopbackmgr-config.yaml
wrote csi-templ/csi-baremetal-deployment/templates/storageclass/default-sc.yaml
wrote csi-templ/csi-baremetal-deployment/templates/storageclass/hdd-sc.yaml
wrote csi-templ/csi-baremetal-deployment/templates/storageclass/hddlvg-sc.yaml
wrote csi-templ/csi-baremetal-deployment/templates/storageclass/nvme-sc.yaml
wrote csi-templ/csi-baremetal-deployment/templates/storageclass/ssd-sc.yaml
wrote csi-templ/csi-baremetal-deployment/templates/storageclass/ssdlvg-sc.yaml
wrote csi-templ/csi-baremetal-deployment/templates/storageclass/syslvg-sc.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/csibm-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/node-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/node-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/scheduler-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/csibm-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/node-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/node-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/node-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/scheduler-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/rbac/controller-rbac.yaml
wrote csi-templ/csi-baremetal-deployment/templates/csi-baremetal_v1_deployment.yaml
wrote csi-templ/csi-baremetal-deployment/templates/csidriver/csidriver.yaml

# Install CSI Deployment manifest only
user@user-vm ~/g/s/g/dell> ka csi-templ/csi-baremetal-deployment/templates/csi-baremetal_v1_deployment.yaml
Error from server (deployment ... already exists): error when creating "csi-templ/csi-baremetal-deployment/templates/csi-baremetal_v1_deployment.yaml": admission webhook "mapplication.kb.io" denied the request: deployment ... already exists

# We received error, which specified in code
```