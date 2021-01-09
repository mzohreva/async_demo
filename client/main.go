package main

import (
	"math/rand"
	"time"

	"example.com/client/cmd"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	cmd.Execute()
}
