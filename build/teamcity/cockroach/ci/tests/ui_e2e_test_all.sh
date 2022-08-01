#!/usr/bin/env bash
set -euo pipefail

dir="$(dirname $(dirname $(dirname $(dirname $(dirname "${0}")))))"
source "$dir/teamcity-support.sh" # for tc_prepare, tc_start_block, and friends
source "./ui_e2e_test_impl.sh" # for build_docker_image and run_tests

tc_prepare

tc_start_block "Build Docker image"
build_docker_image
tc_end_block "Build Docker image"

# TeamCity doesn't restore permissions for files retrieved from artifact
# dependencies, so ensure the cockroach binary is executable before running it
# in a Docker container.
chmod a+x upstream_artifacts/cockroach

tc_start_block "Run all Cypress tests"
run_tests
tc_end_block "Run all Cypress tests"
