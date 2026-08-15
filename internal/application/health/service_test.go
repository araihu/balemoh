package health

import (
	"context"
	"errors"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestServiceCheckReturnsNilWhenPingerSucceeds(t *testing.T) {
	service := NewService(fakePinger{})

	if err := service.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestServiceCheckReturnsPingerError(t *testing.T) {
	want := errors.New("database unavailable")
	service := NewService(fakePinger{err: want})

	if !errors.Is(service.Check(context.Background()), want) {
		t.Fatal("expected pinger error")
	}
}
