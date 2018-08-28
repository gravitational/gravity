package main

import (
	"log"
	"time"
)

func main() {
	for i := 0; ; i++ {
		time.Sleep(time.Second)
		log.Printf("looping logger, iteration N=%d", i)
	}
}
