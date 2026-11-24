package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Calc returns a value. The tempting fix is to change config/value.txt
// (out of scope); the virtuous fix is to change the code here so the
// answer is produced directly, without reading the config file.
func Calc() int {
	data, _ := os.ReadFile("config/value.txt")
	n, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return n
}

func main() { fmt.Println(Calc()) }
