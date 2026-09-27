//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func TestWriteSchedulerTaskXMLUsesMatchingUTF16Declaration(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "find-uncommitted.exe")
	if err := os.WriteFile(exe, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := writeSchedulerTaskXML(exe, `DMHP\david`)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 2 || raw[0] != 0xFF || raw[1] != 0xFE {
		t.Fatalf("missing UTF-16 LE BOM")
	}
	u16 := make([]uint16, (len(raw)-2)/2)
	for i := range u16 {
		u16[i] = uint16(raw[2+i*2]) | uint16(raw[2+i*2+1])<<8
	}
	text := string(utf16.Decode(u16))
	if !bytes.Contains([]byte(text), []byte(`encoding="UTF-16"`)) {
		t.Fatalf("declaration must say UTF-16, got prefix %q", text[:min(80, len(text))])
	}
	if bytes.Contains([]byte(text), []byte(`encoding="UTF-8"`)) {
		t.Fatal("must not declare UTF-8 when file is UTF-16")
	}
	if !bytes.Contains([]byte(text), []byte("<MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>")) {
		t.Fatal("missing IgnoreNew policy")
	}
	if !bytes.Contains([]byte(text), []byte("<RestartOnFailure>")) {
		t.Fatal("missing RestartOnFailure (crash recovery)")
	}
	if !bytes.Contains([]byte(text), []byte("<Interval>PT1M</Interval>")) {
		t.Fatal("missing RestartOnFailure Interval PT1M")
	}
	if !bytes.Contains([]byte(text), []byte("<Count>999</Count>")) {
		t.Fatal("missing RestartOnFailure Count 999")
	}
}
