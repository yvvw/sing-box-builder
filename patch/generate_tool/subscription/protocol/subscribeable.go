package protocol

type Subscription interface {
	GetRemark() string
	GetServer() string

	ToOutbound(string) (string, error)
}
