#!/bin/sh
# SPDX-FileCopyrightText: 2023 Anmol Sethi <hi@nhooyr.io>
# SPDX-License-Identifier: ISC

set -eu
cd -- "$(dirname "$0")/.."

go test --run=^$ --bench=. --benchmem --memprofile ci/out/prof.mem --cpuprofile ci/out/prof.cpu -o ci/out/websocket.test "$@" .
(
  cd ./internal/thirdparty
  go test --run=^$ --bench=. --benchmem --memprofile ../../ci/out/prof-thirdparty.mem --cpuprofile ../../ci/out/prof-thirdparty.cpu -o ../../ci/out/thirdparty.test "$@" .
)
