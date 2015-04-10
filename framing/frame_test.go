package framing

import (
	"bytes"
	"io"
	"testing"
)

func TestWinchExchangeErrors(t *testing.T) {
	buf := new(bytes.Buffer)
	buf.Write([]byte{byte(WinchMessage), 0, 0})
	winch := &Winch{}
	if err := winch.ReadFrom(buf); err == nil {
		t.Fatal("Under-full buffer in winch read does not error")
	}
}

func TestDataExchangeErrors(t *testing.T) {
	buf := new(bytes.Buffer)

	data := &Data{data: []byte("hello"), length: 30}
	if err := data.WriteTo(buf); err != nil {
		// this call should not error. see below for the error condition.
		t.Fatal(err)
	}

	if data.length != 5 {
		t.Fatal("Did not adjust frame after setting length to 30 bytes for a 5 byte frame")
	}

	buf.Reset()

	buf.Write([]byte{byte(DataMessage), 255, 0})
	data = &Data{}
	if err := data.ReadFrom(buf); err == nil {
		t.Fatal("Did not receive error after writing a frame with a longer message length than the message")
	}
}

func TestDataExchange(t *testing.T) {
	buf := new(bytes.Buffer)
	data := &Data{data: []byte("hello")}

	if err := data.WriteTo(buf); err != nil {
		t.Fatal(err)
	}

	if string(data.Bytes()) != "hello" {
		t.Fatalf("Written payload does not match what was actually written: %v, %s", data.Bytes(), string(data.Bytes()))
	}

	data = &Data{}

	if err := data.ReadFrom(buf); err != nil {
		t.Fatal(err)
	}

	if string(data.Bytes()) != "hello" {
		t.Fatalf("Read payload does not match what was actually written: %v, %s", data.Bytes(), string(data.Bytes()))
	}
}

func TestWinchExchange(t *testing.T) {
	buf := new(bytes.Buffer)
	winch := &Winch{Height: 20, Width: 30}

	if err := winch.WriteTo(buf); err != nil {
		t.Fatal(err)
	}

	content := buf.Bytes()
	if len(content) != 4 {
		t.Fatalf("Length was incorrect after winch write: %d", len(content))
	}

	if content[0] != 30 || content[2] != 20 {
		t.Fatalf("Content was incorrect after write: %v", content)
	}

	winch = &Winch{}

	if err := winch.ReadFrom(buf); err != nil {
		t.Fatal(err)
	}

	if winch.Height != 20 || winch.Width != 30 {
		t.Fatalf("Winch payload is incorrect: height %d, width %d", winch.Height, winch.Width)
	}
}

func TestStreamParser(t *testing.T) {
	winch := &Winch{}
	data := &Data{}

	r, w := io.Pipe()

	s := &StreamParser{
		Reader: r,
		DataHandler: func(r io.Reader) error {
			if err := data.ReadFrom(r); err != nil {
				return err
			}

			return nil
		},
		WinchHandler: func(r io.Reader) error {
			if err := winch.ReadFrom(r); err != nil {
				return err
			}

			return nil
		},
		ErrorHandler: func(err error) {
			t.Fatal(err)
		},
	}

	go s.Loop()

	newWinch := &Winch{20, 30}
	if err := newWinch.WriteType(w); err != nil {
		t.Fatal(err)
	}

	if err := newWinch.WriteTo(w); err != nil {
		t.Fatal(err)
	}

	if winch.Width != 20 || winch.Height != 30 {
		t.Fatalf("Window change protocol failed to read winch properly: %v", winch)
	}

	newData := &Data{data: []byte("hello")}
	if err := newData.WriteType(w); err != nil {
		t.Fatal(err)
	}

	if err := newData.WriteTo(w); err != nil {
		t.Fatal(err)
	}

	if string(data.data) != "hello" {
		t.Fatalf("Data protocol failed to read properly: %v, %s", data.data, string(data.data))
	}
}
