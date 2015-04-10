package framing

import (
	"bytes"
	"testing"
)

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
