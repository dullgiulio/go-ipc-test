package msgbud

import (
	"bufio"
	"bytes"
	"errors"
	"testing"
)

type _mapper struct{}

func (m *_mapper) typeMap(data interface{}) (uint32, error) {
	if _, ok := data.(int); ok {
		return 1, nil
	}
	return 0, errors.New("no such type")
}

type _pr struct {
	ch chan int
}

func (p *_pr) pointer() interface{} {
	var i int
	return &i
}

func (p *_pr) receive(data interface{}) {
	if d, ok := data.(int); ok {
		p.ch <- d
	}
	// We assume in this test, that sending 0 is forbidden.
	close(p.ch)
}

func TestRequestResponse(t *testing.T) {
	var buf bytes.Buffer

	w := bufio.NewWriter(&buf)
	r := bufio.NewReader(&buf)
	mapper := &_mapper{}

	l := newListener(r, mapper)
	go l.listen()

	pr := &_pr{ch: make(chan int)}

	s := newSender(w, mapper)

	req := 42
	go s.sendListen(&req, l, pr)

	d := <-pr.ch

	if d != 42 {
		t.Error("Expected 42, got another value")
	}
}
