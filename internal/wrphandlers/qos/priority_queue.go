// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"github.com/xmidt-org/wrp-go/v3"
)

// PriorityQueue implements heap.Interface and holds Items, using wrp.QOSValue as its priority.
// https://xmidt.io/docs/wrp/basics/#qos-description-qos
type PriorityQueue []wrp.Message

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].QualityOfService > pq[j].QualityOfService
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x any) {
	*pq = append(*pq, x.(wrp.Message))
}

func (pq *PriorityQueue) Pop() any {
	if len(*pq) == 0 {
		return nil
	}
	n := len(*pq)
	i := (*pq)[n-1]
	*pq = (*pq)[0 : n-1]

	return i
}
