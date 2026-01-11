package main

import "fmt"

func main() {
	InitLogger()

	job := Job{
		ID:      "job-1",
		Command: "ls",
		Args:    []string{"-al"},
	}

	RunJob(&job)

	fmt.Printf("\nFinal Job State:\n%+v\n", job)
}
