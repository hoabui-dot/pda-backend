package domain

type DomainError struct {
	Code        string            `json:"code"`
	SafeMessage string            `json:"message"`
	Details     map[string]string `json:"details,omitempty"`
	Retryable   bool              `json:"retryable"`
}

func (e *DomainError) Error() string { return e.Code }
