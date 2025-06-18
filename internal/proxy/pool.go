package proxy

import (
	"bufio"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	proxies []string
	once    sync.Once
)

// load proxies from file only once
func load(path string) {
	file, err := os.Open(path)
	if err != nil {
		return // proxies slice remains nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		proxies = append(proxies, line)
	}
}

// GetRandom returns random proxy from proxies.txt.
// It panics if file missing or list empty because program must not run without proxy.
func GetRandom() string {
	once.Do(func() {
		rand.Seed(time.Now().UnixNano())
		load("proxies.txt")
	})

	if len(proxies) == 0 {
		panic("Нет доступных прокси в proxies.txt — обязательное условие работы")
	}

	return proxies[rand.Intn(len(proxies))]
}
