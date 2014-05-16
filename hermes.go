package hermes

type Service interface {
	Send() (*Response, error)
}

type Message interface {
	Bytes() ([]byte, error)
}

type Response interface {
	Bytes() ([]byte, error)
}
