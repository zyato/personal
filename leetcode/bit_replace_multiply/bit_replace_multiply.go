package bit_replace_multiply

// 加法、减法：采用补码进行运算，加、减无差别计算
func bitAdd(a int, b int) int {
	if b == 0 {
		return a
	}
	// 无进位加法：a + b = a ^ b
	// 只计算进位：a + b = (a & b) << 1
	return bitAdd(a^b, (a&b)<<1)
}

// 乘法：类似于手工计算十进制乘法的思路
func bitMultiply(a, b int) int {
	res := 0
	for b != 0 {
		if b&1 == 1 {
			res = bitAdd(res, a)
		}
		a <<= 1
		b = unsignedRightShift(b)
	}
	return res
}

// 除法：todo 后续补充
func bitDivide(a, b int) int {
	return a / b
}

// go没有无符号右移操作，需要自己实现无符号右移1位（转成uint后右移，左边就会补0达到无符号右移的目的）
func unsignedRightShift(x int) int {
	return int(uint(x) >> 1)
}
