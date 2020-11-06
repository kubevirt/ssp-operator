#!/bin/bash

olm_output=$(docker run --rm --entrypoint=/usr/bin/csv-generator quay.io/oyahud/kubevirt-ssp-operator:latest \
--csv-version=test --namespace=namespace-test  --validator-image=validator-test --dump-crds \
--kvm-info-image=kvm-test --virt-launcher-image=virt-launcher-test --node-labeller-image=node-labeller-test \
--cpu-plugin-image=cpu-plugin-test --operator-image=operator-test) 

if [ $(echo $olm_output | grep 'name: ssp-operator.vtest'| wc -l) -eq 0 ]; then
    echo "output doesn't contain correct csv-version"
    exit 1
fi

if [ $(echo $olm_output | grep 'value: \"kvm-test\"'| wc -l) -eq 0 ]; then
    echo "output doesn't contain correct kvm-info-image"
    exit 1
fi

if [ $(echo $olm_output | grep 'value: \"validator-test\"'| wc -l) -eq 0 ]; then
    echo "output doesn't contain correct validator-image"
    exit 1
fi

if [ $(echo $olm_output | grep 'value: \"virt-launcher-test\"'| wc -l) -eq 0 ]; then
    echo "output doesn't contain correct virt-launcher-image"
    exit 1
fi

if [ $(echo $olm_output | grep 'value: \"node-labeller-test\"'| wc -l) -eq 0 ]; then
    echo "output doesn't contain correct node-labeller-image"
    exit 1
fi

if [ $(echo $olm_output | grep 'value: \"cpu-plugin-test\"'| wc -l) -eq 0 ]; then
    echo "output doesn't contain correct cpu-plugin-image"
    exit 1
fi

if [ $(echo $olm_output | grep 'group: ssp.kubevirt.io'| wc -l) -eq 0 ]; then
    echo "output doesn't contain CRD definition"
    exit 1
fi

#test the case without --dump-crds flag
olm_output=$(docker run --rm --entrypoint=/usr/bin/csv-generator quay.io/oyahud/kubevirt-ssp-operator:latest \
--csv-version=test --namespace=namespace-test  --validator-image=validator-test \
--kvm-info-image=kvm-test --virt-launcher-image=virt-launcher-test --node-labeller-image=node-labeller-test \
--cpu-plugin-image=cpu-plugin-test --operator-image=operator-test) 

if [ $(echo $olm_output | grep 'group: ssp.kubevirt.io'| wc -l) -eq 1 ]; then
    echo "output contains CRD definition"
    exit 1
fi