package main

import (
	"bytes"
	"testing"
)

func Test_Encrypt(t *testing.T) {
	key := make([]byte, 32)
	copy(key, []byte("Secret"))
	plain := []byte("Tiger, Tiger, Burining bright in the forest of the night.")

	encrypted, iv, err := Encrypt(key, 8, plain)
	if err != nil {
		t.Error(err)
		return
	}

	if bytes.Compare(plain, encrypted) == 0 {
		t.Error("Encrypted text is the same as the plain text")
		return
	}

	plain2, err := Decrypt(key, iv, encrypted)

	if bytes.Compare(plain, plain2) != 0 {
		t.Error("Decrypted text is NOT the same as the plain text")
		return
	}
}

func Test_EncryptPacket(t *testing.T) {
	key := make([]byte, 32)
	copy(key, []byte("Secret"))

	const testText = "Tiger, Tiger, Burining bright in the forest of the night." // "Some plain text to be encrypted and decrypted"
	p := Packet{
		Sk:   [...]byte{'S', 'E', 'S', 'S', 'I', 'N'},
		Data: []byte(testText),
	}

	err := EncryptPacket(&p, key)
	if err != nil {
		t.Error("Failed to encrypt the ", err)
		return
	}

	if len(p.EncryptedData) == 0 {
		t.Error("Encrypted data is still empty")
		return
	}

	p.Data = nil

	err = DecryptPacket(&p, key)
	if err != nil {
		t.Error("Failed to Decrypt the ", err)
		return
	}

	if bytes.Compare(p.Data, []byte(testText)) != 0 {
		t.Error("Decrypted data failed to match the original text")
		return
	}

}
