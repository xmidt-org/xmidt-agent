#!/bin/sh
# SPDX-FileCopyrightText: 2023 Anmol Sethi <hi@nhooyr.io>
# SPDX-License-Identifier: ISC

set -eu
cd -- "$(dirname "$0")"

echo "=== fmt.sh"
./ci/fmt.sh
echo "=== lint.sh"
./ci/lint.sh
echo "=== test.sh"
./ci/test.sh "$@"
echo "=== bench.sh"
./ci/bench.sh
