package kmp

/*
给你一个下标从 0 开始长度为 n 的整数数组 nums ，和一个下标从 0 开始长度为 m 的整数数组 pattern ，pattern 数组只包含整数 -1 ，0 和 1 。

大小为 m + 1 的子数组 nums[i..j] 如果对于每个元素 pattern[k] 都满足以下条件，那么我们说这个子数组匹配模式数组 pattern ：
    - 如果 pattern[k] == 1 ，那么 nums[i + k + 1] > nums[i + k]
    - 如果 pattern[k] == 0 ，那么 nums[i + k + 1] == nums[i + k]
    - 如果 pattern[k] == -1 ，那么 nums[i + k + 1] < nums[i + k]

请你返回匹配 pattern 的 nums 子数组的 数目
*/

func countMatchingSubarrays(nums []int, pattern []int) int {
	text := make([]int, 0, len(nums)-1)
	for i := 1; i < len(nums); i++ {
		v := nums[i] - nums[i-1]
		if v > 0 {
			v = 1
		}
		if v < 0 {
			v = -1
		}
		text = append(text, v)
	}
	return NewKMP(pattern).SearchCount(text)
}

type KMP struct {
	pattern []int
	next    []int
}

// NewKMP 生成 KMP 工具，并且计算出 next 数组
func NewKMP(pattern []int) KMP {
	i, j, n := 0, -1, len(pattern)
	next := make([]int, n+1)
	next[0] = -1
	for i < n {
		if j == -1 || pattern[i] == pattern[j] {
			i, j = i+1, j+1
			next[i] = j
		} else {
			j = next[j]
		}
	}
	return KMP{
		pattern: pattern,
		next:    next,
	}
}

// SearchCount pattern 在 text 中出现的次数(可以重叠)
func (kmp KMP) SearchCount(text []int) (count int) {
	pattern, next := kmp.pattern, kmp.next
	tLen, pLen := len(text), len(pattern)
	i, j := 0, 0
	for i < tLen && j < pLen {
		if j == -1 || text[i] == pattern[j] {
			i, j = i+1, j+1
		} else {
			j = next[j]
		}
		if j == pLen {
			count++
			j = next[j]
		}
	}
	return count
}

// Search pattern 在 text 中首次出现的下标
func (kmp KMP) Search(text []int) int {
	pattern, next := kmp.pattern, kmp.next
	tLen, pLen := len(text), len(pattern)
	i, j := 0, 0
	for i < tLen && j < pLen {
		if j == -1 || text[i] == pattern[j] {
			i, j = i+1, j+1
		} else {
			j = next[j]
		}
	}
	if j == pLen {
		return i - j
	}
	return -1
}
