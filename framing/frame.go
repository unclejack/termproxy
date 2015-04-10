package framing

import (
	"encoding/binary"
	"fmt"
	"io"
)

type MessageType int16

const (
	WinchMessage MessageType = iota
	DataMessage
)

type Message interface {
	Type() MessageType
	WriteType(io.Writer) error
	WriteTo(io.Writer) error
	ReadFrom(io.Reader) error
}

type Winch struct {
	Width  int16
	Height int16
}

type Data struct {
	data   []byte
	length int32
}

func (msg *Data) Type() MessageType {
	return DataMessage
}

func (w *Winch) Type() MessageType {
	return WinchMessage
}

func (data *Data) WriteType(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, DataMessage)
}

func (winch *Winch) WriteType(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, WinchMessage)
}

func (msg *Data) Len() int {
	return int(msg.length)
}

func (winch *Winch) WriteTo(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, winch.Width); err != nil {
		return err
	}

	if err := binary.Write(w, binary.LittleEndian, winch.Height); err != nil {
		return err
	}

	return nil
}

func (w *Winch) ReadFrom(r io.Reader) (err error) {
	err = binary.Read(r, binary.LittleEndian, &w.Width)
	if err != nil {
		return
	}
	err = binary.Read(r, binary.LittleEndian, &w.Height)
	return
}

func (msg *Data) WriteTo(w io.Writer) error {
	msg.length = int32(len(msg.data))
	binary.Write(w, binary.LittleEndian, msg.length)
	_, err := w.Write(msg.data)
	return err
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
	MsgTypeHandler func(MessageType)
	WinchHandler   func(io.Reader) error
	Reader         io.Reader
}

func (s *StreamParser) Loop() {
	var (
		err     error
		msgType MessageType
	)

	for err == nil {
		err = binary.Read(s.Reader, binary.LittleEndian, &msgType)
		if err != nil {
			s.ErrorHandler(err)
			return
		}

		if s.MsgTypeHandler != nil {
			s.MsgTypeHandler(msgType)
		}

		switch msgType {
		case WinchMessage:
			err = s.WinchHandler(s.Reader)
		case DataMessage:
			err = s.DataHandler(s.Reader)
		}
	}

	if err != io.EOF {
		s.ErrorHandler(err)
	}
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

func MsgTypePrinter(msgType MessageType) {
	fmt.Printf("found a message of type %d\n", msgType)
}
