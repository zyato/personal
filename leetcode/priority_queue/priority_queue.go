package priority_queue

import (
	"container/heap"
)

type elem struct {
	first, second int
}

// second可选，不传等价于 type priorityQueue []int
func newPair(first int, second ...int) elem {
	p := elem{first: first}
	if len(second) > 0 {
		p.second = second[0]
	}
	return p
}

type priorityQueue []elem

func newPriorityQueue(size ...int) priorityQueue {
	var siz int
	if len(size) > 0 {
		siz = size[0]
	}
	return make(priorityQueue, 0, siz)
}

func (pq *priorityQueue) Len() int {
	return len(*pq)
}

func (pq *priorityQueue) Less(i, j int) bool {
	if (*pq)[i].first == (*pq)[j].first {
		return (*pq)[i].second < (*pq)[j].second
	}
	return (*pq)[i].first < (*pq)[j].first
}

func (pq *priorityQueue) Swap(i, j int) {
	(*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i]
}

func (pq *priorityQueue) Push(v any) {
	*pq = append(*pq, v.(elem))
}

func (pq *priorityQueue) Pop() any {
	v := (*pq)[pq.Len()-1]
	*pq = (*pq)[:pq.Len()-1]
	return v
}

func (pq *priorityQueue) Empty() bool {
	return len(*pq) == 0
}

func (pq *priorityQueue) push(v elem) {
	heap.Push(pq, v)
}

func (pq *priorityQueue) pop() elem {
	return heap.Pop(pq).(elem)
}
