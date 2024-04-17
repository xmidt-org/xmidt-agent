// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"github.com/xmidt-org/wrp-go/v3"
)

// Item contains a wrp Message pointer and is managed by PriorityQueue.
type Item struct {
	value    *wrp.Message
	priority wrp.QOSValue
	index    int
}

// PriorityQueue implements heap.Interface and holds Items, using wrp.QOSValue as its priority.
// https://xmidt.io/docs/wrp/basics/#qos-description-qos
type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].priority > pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	i := x.(*Item)
	i.index = n
	i.priority = i.value.QualityOfService
	if i.value.QualityOfService > 99 {
		i.priority = i.value.QualityOfService
	}

	*pq = append(*pq, i)
}

func (pq *PriorityQueue) Pop() any {
	if len(*pq) == 0 {
		return nil
	}

	old := *pq
	n := len(old)
	i := old[n-1]
	// avoid memory leak
	old[n-1] = nil
	// for safety (no longer managed by PriorityQueue)
	i.index = -1
	*pq = old[0 : n-1]

	return i
}
