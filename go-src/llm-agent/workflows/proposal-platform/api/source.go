package api

// Source pins a read-only Git range and explicit required-signature paths.
type Source struct {
	MustSignReasons map[string]string `json:"must_sign_reasons,omitempty"`
	Repository      string            `json:"repository"`
	Base            string            `json:"base"`
	Commit          string            `json:"commit"`
	MustSign        []string          `json:"must_sign"`
}
