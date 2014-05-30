package hermes

import "fmt"

var (
	// ErrRetry means the external server request failed
	// and a retry should happen.
	ErrRetry = fmt.Errorf("retry")
	// ErrUpdateToken means the external server responded
	// with updated tokens.
	ErrUpdateToken = fmt.Errorf("update token")
	// ErrRemoveToken means the external server responded
	// with tokens to remove.
	ErrRemoveToken = fmt.Errorf("remove token")
	// ErrTokenExpired means the api access token has expired.
	ErrTokenExpired = fmt.Errorf("token expired")
)

// Service is the interface for apns/gcm/c2dm/adm
type Service interface {
	Send() (*Response, error)
}

// Message is the json encoded message.
type Message interface {
	Bytes() ([]byte, error)
}

// Response is the interface for responses.
type Response interface {
	Bytes() ([]byte, error)
	Retry() int        // retry send if not -1
	UpdateToken() bool // update/remove token?
}
