#!/usr/bin/env bash
# Copyright 2020-present Open Networking Foundation
# SPDX-License-Identifier: Apache-2.0

set -ex

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" > /dev/null 2>&1 && pwd)"
DBUF_ROOT_DIR=$(dirname "$DIR")

docker run --rm -v "${DBUF_ROOT_DIR}":/dbuf -w /dbuf omecproject/reuse-verify:latest reuse lint
