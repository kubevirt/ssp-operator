#!/usr/bin/env bash

readonly PROMTOOL="$(dirname "$0")/../bin/promtool"

function cleanup() {
    local cleanup_dir="${1:?}"
    rm -rf "${cleanup_dir}"
}

function main() {
    local prom_spec_dumper="${1:?}"
    local tests_file="${2:?}"
    local temp_dir

    temp_dir="$(mktemp --tmpdir --directory metrics_test_dir.XXXXX)"
    trap "cleanup ${temp_dir}" RETURN EXIT INT

    local rules_file="${temp_dir}/rules.json"
    local tests_copy="${temp_dir}/rules-test.yaml"

    "${prom_spec_dumper}" > "${rules_file}"
    cp "${tests_file}" "${tests_copy}"

    echo "INFO: Rules file content:"
    cat "${rules_file}"
    echo

    ${PROMTOOL} check rules "${rules_file}"
    ${PROMTOOL} test rules "${tests_copy}"
}

main "$@"
