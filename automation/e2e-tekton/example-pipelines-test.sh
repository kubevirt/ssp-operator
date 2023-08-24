#!/bin/bash

cp -L $KUBECONFIG /tmp/kubeconfig && export KUBECONFIG=/tmp/kubeconfig
export IMG=${CI_OPERATOR_IMG}
export VALIDATOR_IMG=${CI_VALIDATOR_IMG}

# SECRET
namespace="kubevirt"
if [[ $TARGET =~ windows10.* ]]; then
  namespace="kubevirt-os-images"
  oc create namespace ${namespace}
fi

key="/tmp/secrets/accessKeyId"
token="/tmp/secrets/secretKey"
if test -f "$key" && test -f "$token"; then
  id=$(cat $key | tr -d '\n')
  token=$(cat $token | tr -d '\n')

  oc get secret/pull-secret -n openshift-config --template='{{index .data ".dockerconfigjson" | base64decode}}' > secrets.json
  oc registry login --registry="quay.io/openshift-cnv/containerdisks" --auth-basic="$id:$token" --to=secrets.json
  oc set data secret/pull-secret -n openshift-config --from-file=.dockerconfigjson=secrets.json
fi

./hack/set-crio-permissions-command.sh


# switch to faster storage class for example pipelines tests (slower storage class is causing timeouts due 
# to not able to copy whole windows disk)
if ! oc get storageclass | grep -q 'ssd-csi (default)' > /dev/null; then
  oc annotate storageclass ssd-csi storageclass.kubernetes.io/is-default-class=true --overwrite
  oc annotate storageclass standard-csi storageclass.kubernetes.io/is-default-class- --overwrite
fi

# Deploy resources
echo "Deploying resources"
./automation/common/deploy-kubevirt-and-cdi.sh

# remove tsc node labels which causes that windows VMs could not be scheduled due to different value in tsc node selector
for node in $(oc get nodes -o name -l node-role.kubernetes.io/worker); do
  tscLabel="$(oc describe $node | grep scheduling.node.kubevirt.io/tsc-frequency- | xargs | cut -d"=" -f1)"
  # disable node labeller
  oc annotate ${node} node-labeller.kubevirt.io/skip-node=true --overwrite
  # remove tsc labels
  oc label ${node} cpu-timer.node.kubevirt.io/tsc-frequency- --overwrite
  oc label ${node} cpu-timer.node.kubevirt.io/tsc-scalable- --overwrite
  oc label ${node} ${tscLabel}- --overwrite
done

function wait_until_exists() {
  timeout 10m bash <<- EOF
  until oc get $1; do
    sleep 5
  done
EOF
}

function wait_for_pipelinerun() {
  oc wait -n kubevirt --for=condition=Succeeded=True pipelinerun -l pipelinerun=$1-run --timeout=60m &
  success_pid=$!

  oc wait -n kubevirt --for=condition=Succeeded=False pipelinerun -l pipelinerun=$1-run --timeout=60m && exit 1 &
  failure_pid=$!

  wait -n $success_pid $failure_pid

  if (( $? == 0 )); then
    echo "Pipelinerun $1 succeeded"
  else
    echo "Pipelinerun $1 failed"
    exit 1
  fi
}

# Disable smart cloning - it does not work properly on azure clusters, when this issue gets fixed we can enable it again - https://issues.redhat.com/browse/CNV-21844
oc patch cdi cdi --type merge -p '{"spec":{"cloneStrategyOverride":"copy"}}'

echo "Creating datavolume with windows iso"
oc apply -n ${namespace} -f "automation/e2e-tekton/test-files/${TARGET}-dv.yaml"

echo "Waiting for pvc to be created"
wait_until_exists "pvc -n ${namespace} iso-dv -o jsonpath='{.metadata.annotations.cdi\.kubevirt\.io/storage\.pod\.phase}'"
oc wait -n ${namespace}  pvc iso-dv --timeout=10m --for=jsonpath='{.metadata.annotations.cdi\.kubevirt\.io/storage\.pod\.phase}'='Succeeded'

echo "Create config map for http server"
oc apply -n ${namespace} -f "automation/e2e-tekton/test-files/configmap.yaml"

echo "Deploying http-server to serve iso file to pipeline"
oc apply -n ${namespace} -f "automation/e2e-tekton/test-files/http-server.yaml"

wait_until_exists "pods -n ${namespace} -l app=http-server"

echo "Waiting for http server to be ready"
oc wait -n ${namespace}  --for=condition=Ready pod -l app=http-server --timeout=10m

echo "Deploy SSP operator"
make deploy
oc wait -n kubevirt deployment ssp-operator --for condition=Available --timeout 10m

# Deploy sample SSP CR
oc apply -f "config/samples/ssp_v1beta2_ssp.yaml"
oc wait -n kubevirt ssp ssp-sample --for condition=Available --timeout 10m

wait_until_exists "pipeline windows-efi-installer -n kubevirt" wait_until_exists "pipeline windows-customize -n kubevirt"

# Run windows10/11/2022-installer pipeline
echo "Running ${TARGET}-installer pipeline"
oc create -n kubevirt -f "automation/e2e-tekton/test-files/${TARGET}-installer-pipelinerun.yaml"
wait_until_exists "pipelinerun -n kubevirt -l pipelinerun=${TARGET}-installer-run"

# Wait for pipeline to finish
echo "Waiting for pipeline to finish"
wait_for_pipelinerun "${TARGET}-installer"

# Run windows-customize pipeline
echo "Running windows-customize pipeline"
oc create -n kubevirt -f "automation/e2e-tekton/test-files/${TARGET}-customize-pipelinerun.yaml"
wait_until_exists "pipelinerun -n kubevirt -l pipelinerun=${TARGET}-customize-run"

# Wait for pipeline to finish
echo "Waiting for pipeline to finish"
wait_for_pipelinerun "${TARGET}-customize"
