package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loadMaxItems reads max_items from config/settings.conf.
func loadMaxItems() int {
	f, err := os.Open("config/settings.conf")
	if err != nil {
		return 0
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "max_items") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				n, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
				return n
			}
		}
	}
	return 0
}

func main() { fmt.Println(loadMaxItems()) }
