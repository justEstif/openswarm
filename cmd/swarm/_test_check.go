//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"github.com/justEstif/openswarm/internal/task"
)

func main() {
	problems := []task.Problem{}
	b, _ := json.Marshal(problems)
	fmt.Println("empty slice:", string(b))

	var nilProblems []task.Problem
	b2, _ := json.Marshal(nilProblems)
	fmt.Println("nil slice:", string(b2))
}
