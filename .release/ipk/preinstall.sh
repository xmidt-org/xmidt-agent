#!/bin/bash
# SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
# SPDX-License-Identifier: Apache-2.0

getent group  xmidt-agent > /dev/null || groupadd -r xmidt-agent
getent passwd xmidt-agent > /dev/null || \
    useradd \
        -d /var/run/xmidt-agent \
        -r \
        -g xmidt-agent \
        -s /sbin/nologin \
        -c "Xmidt-Agent Client" \
        xmidt-agent