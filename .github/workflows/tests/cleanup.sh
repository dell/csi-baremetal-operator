#! /usr/bin/bash
GH_USER="dell"
GH_API_URL="https://api.github.com"
REPO="csi-baremetal-operator"
TEST_TAG="v1.1.0-test"
TEST_RELEASE_ID=$(curl $GH_API_URL/repos/$GH_USER/$REPO/releases/tags/$TEST_TAG | jq -r '.id')
CR_PAT=$(cat .github/workflows/tests/wf.secrets | awk -F "=" '{print $2}')
TEST_RELEASE_URL=$GH_API_URL/repos/$GH_USER/$REPO/releases/$TEST_RELEASE_ID
curl -u $GH_USER:$CR_PAT -X DELETE $TEST_RELEASE_URL
git tag -d $TEST_TAG
git push --delete https://$GH_USER:$CR_PAT@github.com/$GH_USER/$REPO.git $TEST_TAG