package msgbud

import (
	"bufio"
	"bytes"
	"errors"
	"testing"
)

type _mapper struct{}

func (m *_mapper) typeMap(data interface{}) (uint32, error) {
	if _, ok := data.(*TestVal); ok {
		return 1, nil
	}
	return 0, errors.New("no such type")
}

type _pr struct {
	ch chan *TestVal
}

type TestVal struct {
	I int
}

func (p *_pr) pointer() interface{} {
	var t TestVal
	return &t
}

func (p *_pr) receive(data interface{}) {
	if t, ok := data.(*TestVal); ok {
		p.ch <- t
	}
	close(p.ch)
}

func TestRequestResponse(t *testing.T) {
	var buf bytes.Buffer

	r := bufio.NewReader(&buf)
	mapper := &_mapper{}

	l := newListener(r, mapper)
	go l.listen()

	pr := &_pr{ch: make(chan *TestVal)}

	s := newSender(&buf, mapper)

	req := &TestVal{42}
	if err := s.sendListen(req, l, pr); err != nil {
		t.Fatal(err)
	}

	d := <-pr.ch

	if d.I != 42 {
		t.Error("Expected 42, got value ", d.I)
	}
}
