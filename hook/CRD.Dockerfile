#
# Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# This software contains the intellectual property of Dell Inc.
# or is licensed to Dell Inc. from third parties. Use of this software
# and the intellectual property contained therein is expressly limited to the
# terms and conditions of the License Agreement under which it is provided by or
# on behalf of Dell Inc. or its subsidiaries.

# Dockerfile for csi-baremetal-pre-upgrade-crds
ARG KUBECTL_IMAGE
FROM $KUBECTL_IMAGE

COPY charts/csi-baremetal-operator/crds  /crds

USER 1000

ENTRYPOINT ["/bin/sh", "-c"]
