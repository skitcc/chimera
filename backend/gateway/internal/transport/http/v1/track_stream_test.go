package v1

import "testing"

func TestStreamLocation(t *testing.T) {
	const host = "127.0.0.1:9000"

	t.Run("keeps a presigned url on the storage host", func(t *testing.T) {
		raw := "http://127.0.0.1:9000/chimera/track.mp3?X-Amz-Signature=abc"
		got, err := streamLocation(raw, host)
		if err != nil {
			t.Fatalf("streamLocation() error = %v", err)
		}
		if got != raw {
			t.Fatalf("streamLocation() = %q", got)
		}
	})

	t.Run("rejects another host", func(t *testing.T) {
		_, err := streamLocation("https://evil.example/track.mp3", host)
		if err == nil {
			t.Fatal("streamLocation() accepted a foreign host")
		}
	})

	t.Run("rejects embedded credentials", func(t *testing.T) {
		_, err := streamLocation("http://user:secret@127.0.0.1:9000/track.mp3", host)
		if err == nil {
			t.Fatal("streamLocation() accepted userinfo")
		}
	})
}
