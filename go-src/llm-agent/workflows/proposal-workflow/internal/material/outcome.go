package material

// Outcome is the fragment runner's measured report index.
type Outcome struct {
	Format  int             `json:"format"`
	Commit  string          `json:"commit"`
	Reports []OutcomeReport `json:"reports"`
}
