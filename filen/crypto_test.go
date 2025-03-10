package filen

import "testing"

func TestHashFileName(t *testing.T) {
	api := Filen{
		AuthVersion: 2,
	}
	hashes := map[string]string{
		"abc": "5c5a4ad792911a5a58741e16257f62b664aa2df3",
		"cde": "dc4237084f19afa9eb668edcbc39b5da51f63273",
	}

	for name, hash := range hashes {
		if api.HashFileName(name) != hash {
			t.Errorf("expected %s to hash to %s, got %s", name, hash, api.HashFileName(name))
		}
	}
}
