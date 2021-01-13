#!/usr/bin/env python3

import logging
import sys
import yaml

def process(path, version, image):
    with open(path, 'rt') as fh:
        csv = yaml.safe_load(fh)

    # Update CSV fields
    csv['metadata']['annotations']['containerImage'] = image
    csv['metadata']['name'] = 'ssp-operator.'+version
    csv['spec']['version'] = version[1:]

    deploymentPodSpec = csv['spec']['install']['spec']['deployments'][0]['spec']['template']['spec']
    opVerEnv = [env for env in deploymentPodSpec['containers'][0]['env'] if env['name'] == 'OPERATOR_VERSION'][0]
    opVerEnv['value'] = version

    webhookPort = [port for port in deploymentPodSpec['containers'][0]['ports'] if port['name'] == 'webhook-server'][0]

    for webhook in csv['spec']['webhookdefinitions']:
        webhook['containerPort'] = webhookPort['containerPort']

    deploymentPodSpec.pop('volumes')
    deploymentPodSpec['containers'][0].pop('volumeMounts')

    yaml.safe_dump(csv, sys.stdout)


if __name__ == '__main__':
    args = sys.argv[1:]

    csvfile = args[0]
    version = args[1]
    image   = args[2]

    try:
        process(csvfile, version, image)
    except Exception as ex:
        logging.error('error processing %r: %s', args, ex)

