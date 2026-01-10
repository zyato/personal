package main

import (
	"bufio"
	"os"
	"strconv"
)

func main() {}

var sc = newScanner()

type scanner struct {
	*bufio.Scanner
}

func newScanner() *scanner {
	sc := &scanner{
		Scanner: bufio.NewScanner(os.Stdin),
	}
	sc.Split(bufio.ScanWords)
	return sc
}

func (sc *scanner) gInt() int {
    v, _ := strconv.Atoi(sc.gStr())
	return v
}

func (sc *scanner) gStr() string {
    sc.Scan()
    return sc.Text()
}
