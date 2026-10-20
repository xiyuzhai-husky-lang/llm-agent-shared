package decision

// Signature permanently identifies the user and original signed content.
type Signature struct {
	Signer            string `json:"signer"`
	SignedAtCandidate string `json:"signed_at_candidate"`
	CarriedFrom       string `json:"carried_from,omitempty"`
	At                string `json:"at"`
}
