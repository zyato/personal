package yanghui_triangle

/*
   leetcode: https://leetcode.cn/problems/find-the-n-th-value-after-k-seconds/description/
   name: K秒后第N个元素的值
   tag: 杨辉三角、乘法逆元、快速幂、扩展欧几里得
*/

const mod = 1e9 + 7

func valueAfterKSeconds(n int, k int) int {
	// 杨辉三角第n行第k列公式：C(n, k)，其中n和k都是从0开始
	// C(k + n - 1, n - 1)
	res := 1
	for i := 1; i <= n-1; i++ {
		// res = res * (k + n - i) / i
		res = (res * (k + n - i) % mod) * modInverse(i) % mod
	}
	return res
}

// 1. 扩展欧几里得求逆元( ax + by = gcd(a, b) )，要求：gcd(a, b) == 1，其中 b = mod
// modInverse = (x + mod) % mod
func extendedGCD(a, b int) (gcd int, x int, y int) {
	if b == 0 {
		return a, 1, 0
	}
	gcd, x, y = extendedGCD(b, a%b)
	return gcd, y, x - (a/b)*y
}
func modInverse(a int) int {
	gcd, x, _ := extendedGCD(a, mod)
	if gcd != 1 { // not exist
		return -1
	}
	return (x + mod) % mod
}

// 2. 快速幂求乘法逆元，要求：a > 0 && mod > 0 && mod是质数
// modInverse = quickPower(a, mod - 2)
func quickPower(a, pow int) int {
	res := 1
	for pow > 0 {
		if pow&1 == 1 {
			res = (res * a) % mod
		}
		a = (a * a) % mod
		pow >>= 1
	}
	return res
}
