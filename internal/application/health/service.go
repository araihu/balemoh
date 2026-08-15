package health

import "context"

type Pinger interface {
	Ping(context.Context) error
}

type Checker interface {
	Check(context.Context) error
}

type Service struct {
	pinger Pinger
}

func NewService(pinger Pinger) *Service { return &Service{pinger: pinger} }

func (s Service) Check(ctx context.Context) error { return s.pinger.Ping(ctx) }

var _ Checker = Service{}
