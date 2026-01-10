package union_find

import "fmt"

type unionFind struct {
	f   []int // 父节点
	siz []int // 集合大小，当且仅当 v=f[v] 时 siz[v] 才有效
	cc  int   // 连通分量
}

// [0, n)
func newUnionFind(n int) *unionFind {
	um := &unionFind{
		f:   make([]int, n),
		siz: make([]int, n),
		cc:  n,
	}
	for i := 0; i < n; i++ {
		um.f[i] = i
		um.siz[i] = 1
	}
	return um
}

func (um *unionFind) find(v int) int {
	if um.f[v] != v {
		um.f[v] = um.find(um.f[v])
	}
	return v
}

func (um *unionFind) merge(x, y int) bool {
	a, b := um.find(x), um.find(y)
	if a == b {
		return false
	}
	// 两个连通分量合并成一个
	um.cc--
	if um.siz[b] < um.siz[a] {
		um.f[b] = a
		um.siz[a] += um.siz[b]
	} else {
		um.f[a] = b
		um.siz[b] += um.siz[a]
	}
	return true
}

func (um *unionFind) String() string {
	return fmt.Sprintf("connectedComponent[%d]\nF[%v]\nsize[%v]\n", um.cc, um.f, um.siz)
}
