#!/usr/bin/env python3

import logging
import sys
import yaml


_ANNOTATIONS = {
    'categories': 'Openshift Optional',
    'description': \
        'Manages KubeVirt addons for Scheduling, Scale, Performance',
    'containerImage': 'REPLACE_IMAGE:TAG',
}
_VERSIONED_NAME = 'ssp-operator.vPLACEHOLDER_CSV_VERSION'
_DESCRIPTION = "KubeVirt Schedule, Scale and Performance Operator"
_NAMESPACE = 'kubevirt'
_SPEC = {
    'description': _DESCRIPTION,
    'provider': {
        'name': 'KubeVirt project'
    },
    'maintainers': [{
        'name': 'KubeVirt project',
        'email': 'kubevirt-dev@googlegroups.com',
    }],
    'keywords': [
        'KubeVirt', 'Virtualization', 'Template', 'Performance',
        'VirtualMachine', 'Node', 'Labels',
    ],
    'links': [{
        'name': 'KubeVirt',
        'url': 'https://kubevirt.io',
    }, {
        'name': 'Source Code',
        'url': 'https://github.com/kubevirt/ssp-operator'
    }],
    'labels': {
        'alm-owner-kubevirt': 'ssp-operator',
        'operated-by': 'ssp-operator',
    },
    'selector': {
        'matchLabels': {
            'alm-owner-kubevirt': 'ssp-operator',
            'operated-by': 'ssp-operator',
        },
    },
    'version': 'vPLACEHOLDER_CSV_VERSION'
}



def process(path):
    with open(path, 'rt') as fh:
        manifest = yaml.safe_load(fh)

    manifest['metadata']['namespace'] = _NAMESPACE
    manifest['metadata']['annotations'].update(_ANNOTATIONS)

    manifest['spec'].update(_SPEC)
    manifest['metadata']['name'] = _VERSIONED_NAME

    yaml.safe_dump(manifest, sys.stdout)


if __name__ == '__main__':
    for arg in sys.argv[1:]:
        try:
            process(arg)
        except Exception as ex:
            logging.error('error processing %r: %s', arg, ex)
            # keep going!
