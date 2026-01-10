package next_permutation

import (
	"sort"
)

// 给定一个可包含重复数字的序列 nums ，按任意顺序 返回所有不重复的全排列。
func permuteUnique(nums []int) [][]int {
	sort.Ints(nums)
	n := 1
	for i := len(nums); i > 1; i-- {
		n *= i
	}
	res := make([][]int, 0, n)
	res = append(res, append([]int{}, nums...))
	for nextPermutation(nums) {
		res = append(res, append([]int{}, nums...))
	}
	return res
}

// return hasNext
func nextPermutation(nums []int) bool {
	// 逆序找到第一个数值下降的下标记为idx，需要把此位置数值变大，以获取下一个排列
	// 显然，需要把idx之后的第一个大于nums[idx]的数移过来，才能满足下一个排列的条件(最小的大于nums排列的排列)，并交换两个位置数值
	// 交换之后同样满足nums[idx+1:]是非增序列，当nums[idx]变大后，需要让nums[idx+1:]变成非减的，即为下一个排列
	for i := len(nums) - 2; i >= 0; i-- {
		// 为了去重，这里需要取等于
		if nums[i] >= nums[i+1] {
			continue
		}
		idx := search(nums, i+1, len(nums)-1, nums[i])
		nums[i], nums[idx] = nums[idx], nums[i]
		reverse(nums, i+1, len(nums)-1)
		return true
	}
	// 走到这意味着nums逆序，没有下一个排列，或者从升序重新开始
	reverse(nums, 0, len(nums)-1)
	return false
}

func reverse(nums []int, i, j int) {
	for i < j {
		nums[i], nums[j] = nums[j], nums[i]
		i++
		j--
	}
}

// 找非增数组最后一个大于target的下标
func search(nums []int, l, r, v int) int {
	for l <= r {
		m := l + (r-l)/2
		if nums[m] <= v {
			r = m - 1
		} else {
			l = m + 1
		}
	}
	return r
}

// 全排列递归解法
func fullPermutation(nums []int) [][]int {
	// 如果需要去重，只需要保证cur位置针对同一个数只放一次即可。实现起来可以先升序排序nums，这样后面的位置和i位置一样的数直接跳过即可
	sort.Ints(nums)
	n := 1
	for i := len(nums); i > 1; i-- {
		n *= i
	}
	data := make([]int, len(nums))
	res := make([][]int, 0, n)
	vis := make([]bool, len(nums))
	var dfs func(int)
	dfs = func(cur int) {
		if cur == len(nums) {
			res = append(res, append([]int(nil), data...))
			return
		}
		for i := 0; i < len(nums); i++ {
			// 去重需要加上(i > 0 && !vis[i - 1] && nums[i] == nums[i - 1])这个条件，没有此条件表示不去重
			// 表示如果在i-1位置放置过nums[i]，这次就直接跳过。
			// 注意一定要加上!vis[i-1]，如果在i-1没有放置nums[i]，本次是第一次放置nums[i]不应该跳过，跳过会遗漏答案
			if vis[i] || (i > 0 && !vis[i-1] && nums[i] == nums[i-1]) {
				continue
			}
			data[cur] = nums[i]
			vis[i] = true
			dfs(cur + 1)
			vis[i] = false
		}
	}
	dfs(0)
	return res
}
