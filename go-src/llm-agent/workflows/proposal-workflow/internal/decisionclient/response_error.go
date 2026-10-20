package decisionclient

import "fmt"

// ResponseError distinguishes a permanent refusal from a retryable transport failure.
type ResponseError struct {
	Status int
	Body   string
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("decision server %d: %s", e.Status, e.Body)
}
