# SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
# SPDX-License-Identifier: Apache-2.0
#!/usr/bin/env sh
set -e

# check arguments for an option that would cause /xmidt-agent to stop
# return true if there is one
_want_help() {
    local arg
    for arg; do
        case "$arg" in
            -'?'|-h|--help|-s|--show)
                return 0
                ;;
        esac
    done
    return 1
}

_main() {
    # if command starts with an option, prepend xmidt-agent
    if [ "${1:0:1}" = '-' ]; then
        set -- /xmidt-agent "$@"
    fi

    # skip setup if they aren't running /xmidt-agent or want an option that stops /xmidt-agent
    if [ "$1" = '/xmidt-agent' ] && ! _want_help "$@"; then
        echo "Entrypoint script for xmidt-agent Client ${VERSION} started."
    fi

    exec "$@"
}

_main "$@"