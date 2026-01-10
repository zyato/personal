package fenwick

type fwt = int
type fenwick []fwt

func newFenwick(n int) fenwick {
	// f[1:n] 存储数据，避免 pre(0) 特判
	return make(fenwick, n+1)
}

// [可选]如果有初始元素，使用 with 初始化可以降低时间复杂度
// 循环 update 复杂度：O(n log n)
// withInit 复杂度: O(n)
func (f fenwick) withInit(nums []fwt) fenwick {
	for i, v := range nums {
		i++
		f[i] += v
		if t := i + i&-i; t < len(f) {
			f[t] += f[i]
		}
	}
	return f
}

// a[i] 增加 delta，1 <= i <= n。时间复杂度 O(log n)
func (f fenwick) update(i int, delta fwt) {
	// lowbit(i) = i & -i = i - i & (i - 1)
	for ; i < len(f); i += i & -i {
		f[i] += delta
	}
}

// sum(a[1]+...+a[i])，1 <= i <= n。时间复杂度 O(log n)
func (f fenwick) pre(i int) (v fwt) {
	// i - lowbit(i) = i & (i - 1)
	for ; i > 0; i = i & (i - 1) {
		v += f[i]
	}
	return v
}

// sum(a[l] + ... + a[r]), 1 <= l <= r <= n。时间复杂度 O(log n)
func (f fenwick) query(l, r int) fwt {
	if l > r {
		return 0
	}
	return f.pre(r) - f.pre(l-1)
}
