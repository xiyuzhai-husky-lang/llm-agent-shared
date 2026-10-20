// proposal-diff prepares standalone diff pages from one explicit Git range.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"proposal-decision/internal/reviewer"
)

func run() error {
	var input struct {
		Repository string `json:"repo"`
		Base       string `json:"base"`
		Candidate  struct {
			Kind   string `json:"kind"`
			Commit string `json:"commit"`
		} `json:"candidate"`
	}
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 65536))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return err
	}
	if input.Repository == "" || input.Base == "" || (input.Candidate.Kind != "commit" && input.Candidate.Kind != "working-tree") {
		return fmt.Errorf("repo, base and commit or working-tree candidate required")
	}
	result, err := reviewer.DiffByRefs(context.Background(), input.Repository, input.Base, input.Candidate.Commit, input.Candidate.Kind == "working-tree")
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
