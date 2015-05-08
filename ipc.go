package msgbud

import (
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"
)

var endianness binary.ByteOrder = binary.LittleEndian

type ident struct {
	ID   uint32 // Number of request
	Type uint32 // Type identifier
}

type typeMapper interface {
	typeMap(d interface{}) (uint32, error)
}

type sender struct {
	writer io.Writer
	enc    *gob.Encoder
	ident  ident
	mapper typeMapper
	mux    sync.Mutex
}

func newSender(w io.Writer, mapper typeMapper) *sender {
	return &sender{
		writer: w,
		enc:    gob.NewEncoder(w),
		ident:  ident{0, 0},
		mapper: mapper,
	}
}

func (s *sender) send(data interface{}, i ident) error {
	var err error
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.ident.Type, err = s.mapper.typeMap(data); err != nil {
		return err
	}
	if err := binary.Write(s.writer, endianness, &i); err != nil {
		return err
	}
	if err := s.enc.Encode(data); err != nil {
		return err
	}
	return nil
}

func (s *sender) sendListen(req interface{}, l *listener, pr pointerReceiver) error {
	var err error
	s.mux.Lock()
	defer s.mux.Unlock()
	s.ident.ID = s.ident.ID + 1
	if s.ident.Type, err = s.mapper.typeMap(req); err != nil {
		return err
	}
	l.registerIdent(pr, s.ident)
	if err := binary.Write(s.writer, endianness, &s.ident); err != nil {
		return err
	}
	if err := s.enc.Encode(req); err != nil {
		return err
	}
	return nil
}

type pointerReceiver interface {
	pointer() interface{}
	receive(interface{})
}

type listener struct {
	reader io.Reader
	dec    *gob.Decoder
	recvI  map[uint32]pointerReceiver
	recvT  map[uint32]pointerReceiver
	mapper typeMapper
	mux    sync.Mutex
}

func newListener(r io.Reader, mapper typeMapper) *listener {
	return &listener{
		reader: r,
		dec:    gob.NewDecoder(r),
		recvI:  make(map[uint32]pointerReceiver),
		recvT:  make(map[uint32]pointerReceiver),
		mapper: mapper,
	}
}

func (l *listener) registerType(pr pointerReceiver, data interface{}) error {
	l.mux.Lock()
	defer l.mux.Unlock()
	t, err := l.mapper.typeMap(data)
	if err != nil {
		return err
	}
	l.recvT[t] = pr
	return nil
}

func (l *listener) registerIdent(pr pointerReceiver, i ident) error {
	l.mux.Lock()
	defer l.mux.Unlock()
	l.recvI[i.ID] = pr
	return nil
}

func (l *listener) accept() error {
	var ident ident
	var err error

	err = binary.Read(l.reader, endianness, &ident)
	if err != nil {
		return err
	}

	l.mux.Lock()
	// First try to get a receiver for this particular message,
	// Otherwise, try a receiver for this type.
	rcv, ok := l.recvI[ident.ID]
	if !ok {
		rcv, ok = l.recvT[ident.Type]
		if !ok {
			l.mux.Unlock()
			return fmt.Errorf("No receiver for id %d, type ID %d", ident.ID, ident.Type)
		}
	}
	delete(l.recvI, ident.ID)
	l.mux.Unlock()

	data := rcv.pointer()
	err = l.dec.Decode(data)
	if err != nil && err != io.EOF {
		return err
	}
	go rcv.receive(data)
	return err
}

func (l *listener) listen() {
	for {
		if err := l.accept(); err != nil {
			if err == io.EOF {
				break
			}
			log.Print("ERROR: ", err)
		}
	}
}
