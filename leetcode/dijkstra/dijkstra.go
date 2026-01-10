package dijkstra

import (
	"container/heap"
)

type dijkstra struct {
	edges [][]elem // edges[i]: 从i结点出发可达的所有结点elem.second和边权elem.first
}

// n个结点，分别为0到n-1
func newDijkstra(n int) *dijkstra {
	return &dijkstra{
		edges: make([][]elem, n),
	}
}

func (d *dijkstra) addEdge(x, y, v int) {
	// 优先队列按照first、second优先级排序，所以y和v需要反着写
	d.edges[x] = append(d.edges[x], newPair(v, y))
}

func (d *dijkstra) run(start int) []int {
	n := len(d.edges)
	dist := make([]int, n)
	for i := range dist {
		dist[i] = -1
	}
	vis := make([]bool, n)
	dist[start] = 0
	pq := newPriorityQueue(n)
	pq.push(newPair(0, start))
	for !pq.Empty() {
		cur := pq.pop().second
		if vis[cur] {
			continue
		}
		vis[cur] = true
		for _, e := range d.edges[cur] {
			next, nextDist := e.second, e.first
			if !vis[next] && (dist[next] == -1 || dist[next] > dist[cur]+nextDist) {
				dist[next] = dist[cur] + nextDist
				pq.push(newPair(dist[next], next))
			}
		}
	}
	return dist
}

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
