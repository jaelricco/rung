package secret

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func newTestBox(t *testing.T) *Box {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	box, err := New(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return box
}

func TestSealAndOpenRoundTrip(t *testing.T) {
	box := newTestBox(t)
	const key = "sk-ant-api03-not-a-real-key"

	sealed, err := box.Seal(key)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(sealed, []byte(key)) {
		t.Fatal("the sealed value still contains the plaintext key")
	}

	opened, err := box.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if opened != key {
		t.Fatalf("opened %q, want %q", opened, key)
	}
}

// Two seals of the same key must not look alike, or the database tells anyone
// reading it which users share a key.
func TestSealIsNeverTheSameTwice(t *testing.T) {
	box := newTestBox(t)
	first, _ := box.Seal("sk-ant-same")
	second, _ := box.Seal("sk-ant-same")
	if bytes.Equal(first, second) {
		t.Fatal("two seals of the same key were identical")
	}
}

// A restored database opened with the wrong sealing key must fail here rather
// than handing garbage to a provider.
func TestOpenRejectsAnotherBoxesCiphertext(t *testing.T) {
	sealed, err := newTestBox(t).Seal("sk-ant-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newTestBox(t).Open(sealed); err == nil {
		t.Fatal("a key sealed under a different secret was opened")
	}
	if _, err := newTestBox(t).Open([]byte("short")); err == nil {
		t.Fatal("a value too short to be sealed data was accepted")
	}
}

func TestNewRejectsAKeyOfTheWrongSize(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("an empty key was accepted")
	}
	_, err := New(base64.StdEncoding.EncodeToString([]byte("sixteen bytes...")))
	if err == nil || !strings.Contains(err.Error(), "want 32") {
		t.Fatalf("a 16-byte key should be refused by size, got: %v", err)
	}
}
