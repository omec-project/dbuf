#!/usr/bin/env bash
# Copyright 2020-present Open Networking Foundation
# SPDX-License-Identifier: Apache-2.0

set -ex

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" > /dev/null 2>&1 && pwd)"
DBUF_ROOT_DIR=$(dirname "$DIR")

# Change to project root so that mockgen paths are correct.
cd "$DBUF_ROOT_DIR"

mockgen -package=dbuf -copyright_file=build/mockgen-license-header.txt \
    -source=dataplane_interface.go -destination=dataplane_interface_mock_test.go
mockgen -package=dbuf -copyright_file=build/mockgen-license-header.txt \
    -source=queue_manager.go -package=dbuf -destination=queue_manager_mock_test.go