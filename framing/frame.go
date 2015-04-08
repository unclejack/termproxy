package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	WinchMessage = iota
	DataMessage
)

type Winch struct {
	Width  int16
	Height int16
}

type Data struct {
	length int32
	data   []byte
}

func (msg *Data) Len() int {
	return int(msg.length)
}

func (winch *Winch) WriteTo(w io.Writer) {
	var messageType int16 = WinchMessage
	binary.Write(w, binary.LittleEndian, messageType)
	binary.Write(w, binary.LittleEndian, winch.Width)
	binary.Write(w, binary.LittleEndian, winch.Height)
}

func (w *Winch) ReadFrom(r io.Reader) (err error) {
	err = binary.Read(r, binary.LittleEndian, &w.Width)
	if err != nil {
		return
	}
	err = binary.Read(r, binary.LittleEndian, &w.Height)
	return
}

func (msg *Data) WriteTo(w io.Writer) {
	var messageType int16 = DataMessage
	binary.Write(w, binary.LittleEndian, messageType)
	binary.Write(w, binary.LittleEndian, msg.length)
	w.Write(msg.data)
}

func (msg *Data) ReadFrom(r io.Reader) (err error) {
	var (
		b []byte
		n int
	)
	err = binary.Read(r, binary.LittleEndian, &msg.length)
	if err != nil {
		return
	}

	if msg.data == nil || msg.length > int32(len(msg.data)) {
		b = make([]byte, msg.length)
	} else {
		b = msg.data
	}

	n, err = r.Read(b)
	if err != nil && msg.length < int32(n) && err != io.EOF {
		return
	}
	if msg.length > int32(n) {
		return fmt.Errorf("expected %d bytes, read only %d", msg.length, n)
	}
	msg.data = b
	return
}

func (msg *Data) Bytes() []byte {
	if len(msg.data) > int(msg.length) {
		return msg.data[:msg.length]
	}
	return msg.data
}

type StreamParser struct {
	DataHandler    func(io.Reader) error
	ErrorHandler   func(error)
	MsgTypeHandler func(int16)
	WinchHandler   func(io.Reader) error
	Reader         io.Reader
}

func (s *StreamParser) Loop() {
	var (
		err     error
		msgType int16
	)
	for err == nil {
		err = binary.Read(s.Reader, binary.LittleEndian, &msgType)
		if err != nil {
			s.ErrorHandler(err)
			return
		}
		s.MsgTypeHandler(msgType)
		switch msgType {
		case WinchMessage:
			err = s.WinchHandler(s.Reader)
		case DataMessage:
			err = s.DataHandler(s.Reader)
		}
	}
	s.ErrorHandler(err)
}

func WinchPrinter(r io.Reader) (err error) {
	w := &Winch{}
	err = w.ReadFrom(r)
	fmt.Println(w)
	return
}

func DataPrinter(r io.Reader) (err error) {
	d := &Data{}
	err = d.ReadFrom(r)
	fmt.Printf("msg length: %d message : %q\n", d.length, string(d.Bytes()))
	return
}

func ErrorPrinter(err error) {
	if err == io.EOF {
		fmt.Println("reached end of stream")
		return
	}
	if err != nil {
		fmt.Printf("encountered error: %v", err)
	}
}

func MsgTypePrinter(msgType int16) {
	fmt.Printf("found a message of type %d\n", msgType)
}
