#!/usr/bin/env python3

import logging
import sys
import yaml

_VERSIONED_NAME = 'ssp-operator.vPLACEHOLDER_CSV_VERSION'
_SPEC = {
    'version': 'vPLACEHOLDER_CSV_VERSION'
}

def process(path):
    with open(path, 'rt') as fh:
        manifest = yaml.safe_load(fh)

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
