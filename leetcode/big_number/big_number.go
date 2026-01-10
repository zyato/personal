package main

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"strings"
)

func main() {
	get := func(n int) string {
		sb := strings.Builder{}
		sb.Grow(n)
		for i := 0; i < n; i++ {
			sb.WriteByte(byte(rand.IntN(10)) + '0')
		}
		return sb.String()
	}
	for i := 0; i < 10000000; i++ {
		l1, l2 := rand.IntN(100)+1, rand.IntN(100)+1
		s1, s2 := get(l1), get(l2)
		abMult1 := newBigNumber(s1).multiply(newBigNumber(s2)).String()
		abPlus1 := newBigNumber(s1).plus(newBigNumber(s2)).String()
		abMinus1 := newBigNumber(s1).minus(newBigNumber(s2)).String()
		b1, ok1 := big.NewInt(0).SetString(s1, 10)
		b2, ok2 := big.NewInt(0).SetString(s2, 10)
		abMult2 := big.NewInt(0).Mul(b1, b2)
		abPlus2 := big.NewInt(0).Add(b1, b2)
		abMinus2 := big.NewInt(0).Sub(b1, b2)
		if abMult1 != abMult2.String() {
			fmt.Println("Mult:", s1, s2, ok1, ok2, abMult1, abMult2)
			break
		}
		if abPlus1 != abPlus2.String() {
			fmt.Println("Plus:", s1, s2, ok1, ok2, abPlus1, abPlus2)
			break
		}
		if abMinus1 != abMinus2.String() {
			fmt.Println("Minus:", s1, s2, ok1, ok2, abMinus1, abMinus2)
			break
		}
		if i%100000 == 0 {
			fmt.Println(i)
		}
	}
}

type bigNumber struct {
	data       []int8
	isNegative bool
}

// 可接受负数 ("-2")
func newBigNumber(str string) *bigNumber {
	// 倒序存储数字，并且字符转成int8对应数值
	data := make([]int8, 0, len(str))
	for i := len(str) - 1; i >= 0; i-- {
		data = append(data, int8(str[i]-'0'))
	}
	isNegative := str[0] == '-'
	if isNegative {
		data = data[:len(data)-1]
	}
	// 去掉前导0
	for len(data) > 1 && data[len(data)-1] == 0 {
		data = data[:len(data)-1]
	}
	return &bigNumber{
		data:       data,
		isNegative: isNegative,
	}
}

// -1:x<y 0:x=y 1:x>y
func (bn *bigNumber) compare(bn2 *bigNumber) int {
	x, y := bn.data, bn2.data
	if len(x) < len(y) {
		return -1
	}
	if len(x) > len(y) {
		return 1
	}
	for i := len(x) - 1; i >= 0; i-- {
		if x[i] < y[i] {
			return -1
		}
		if x[i] > y[i] {
			return 1
		}
	}
	return 0
}

// x+y
func (bn *bigNumber) plus(bn2 *bigNumber) *bigNumber {
	if bn.isNegative || bn2.isNegative {
		// 两个负数: x+y -> -(|x|+|y|)
		if bn.isNegative && bn2.isNegative {
			return bn.getOpposite().
				plus(bn2.getOpposite()).
				getOpposite()
		}
		// 一个负数x: x+y -> y-|x|
		if bn.isNegative {
			return bn2.minus(bn.getOpposite())
		}
		// 一个负数y: x+y -> x-|y|
		return bn.minus(bn2.getOpposite())
	}
	// 两个正数相加
	x, y := bn.data, bn2.data
	i, j := 0, 0
	var extra int8
	// 在x上原地相加
	for ; i < len(x) || j < len(y) || extra > 0; i, j = i+1, j+1 {
		v := extra
		if i < len(x) {
			v += x[i]
		}
		if j < len(y) {
			v += y[j]
		}
		if v >= 10 {
			extra, v = 1, v-10
		} else {
			extra = 0
		}
		if i >= len(x) {
			x = append(x, 0)
		}
		x[i] = v
	}
	bn.data = x
	return bn
}

// x-y
func (bn *bigNumber) minus(bn2 *bigNumber) *bigNumber {
	if bn.isNegative || bn2.isNegative {
		// 两个负数: x-y -> |y|-|x|
		if bn.isNegative && bn2.isNegative {
			return bn2.getOpposite().minus(bn.getOpposite())
		}
		// 一个负数x: x-y -> -(|x|+|y|)
		if bn.isNegative {
			return bn.getOpposite().
				plus(bn2.getOpposite()).
				getOpposite()
		}
		// 一个负数y: x-y -> x+|y|
		return bn.plus(bn2.getOpposite())
	}
	// 两个正数相减
	cmp := bn.compare(bn2)
	if cmp == 0 {
		return newBigNumber("0")
	}
	// x < y
	if cmp < 0 {
		return bn2.minus(bn).getOpposite()
	}
	// x > y
	x, y := bn.data, bn2.data
	i, j := 0, 0
	var extra int8
	// 在x上原地相减
	for ; i < len(x); i, j = i+1, j+1 {
		if j >= len(y) {
			if extra == 0 {
				break
			}
			if x[i] > 0 {
				x[i]--
				break
			}
			x[i] = 9
			continue
		}
		if x[i]-extra >= y[j] {
			x[i] = x[i] - extra - y[j]
			extra = 0
		} else {
			x[i] = x[i] + 10 - extra - y[j]
			extra = 1
		}
	}
	for len(x) > 1 && x[len(x)-1] == 0 {
		x = x[:len(x)-1]
	}
	bn.data = x
	return bn
}

// x*y
func (bn *bigNumber) multiply(bn2 *bigNumber) *bigNumber {
	// 单独计算符号
	isNegative := bn.isNegative != bn2.isNegative
	// 绝对值相乘
	x, y := bn.data, bn2.data
	v := make([]int8, 0, len(x)+len(y))
	for j := 0; j < len(y); j++ {
		var extra int8
		for i := 0; i < len(x); i++ {
			if i+j >= len(v) {
				v = append(v, 0)
			}
			v[i+j] += extra + x[i]*y[j]
			extra = v[i+j] / 10
			v[i+j] = v[i+j] % 10
		}
		if extra > 0 {
			v = append(v, extra)
		}
	}
	for len(v) > 1 && v[len(v)-1] == 0 {
		v = v[:len(v)-1]
	}
	return &bigNumber{
		data:       v,
		isNegative: isNegative,
	}
}

// x/y
// eg: 1/1 = 1
// eg: 1/2 = 0.5
// eg: 1/3 = 0.(3)
// eg: 1/6 = 0.1(6)
func (bn *bigNumber) divide(bn2 *bigNumber) *bigNumber {
	v := strings.Builder{}
	_, _ = v, v
	return nil
}

func (bn *bigNumber) getOpposite() *bigNumber {
	bn.isNegative = !bn.isNegative
	return bn
}

func (bn *bigNumber) String() string {
	if bn == nil {
		return "Nan"
	}
	sb := strings.Builder{}
	if bn.isNegative {
		sb.WriteByte('-')
	}
	for i := len(bn.data) - 1; i >= 0; i-- {
		sb.WriteByte(byte(bn.data[i] + '0'))
	}
	return sb.String()
}

func (bn *bigNumber) print() {
	fmt.Println(bn.String())
}
