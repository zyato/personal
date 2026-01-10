package sieve_eratosthenes

/*
@description: 埃拉托色尼筛法，适用于 n 较小的场景(n<1e7)
@time_complexity: O(nloglogn)
@space_complexity: O(n)
*/
func sieveOfEratosthenes(n int) (isComposite []bool, primes []int) {
	isComposite = make([]bool, n+1)
	for i := 2; i <= n; i++ {
		if !isComposite[i] {
			primes = append(primes, i)
			for j := i * i; j <= n; j += i {
				isComposite[j] = true
			}
		}
	}
	return isComposite, primes
}
