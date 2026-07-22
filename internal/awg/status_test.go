package awg

import (
	"testing"
	"time"
)

func TestParseAllDump(t *testing.T) {
	dump := []byte("awg0\tprivate\tpublic\t51820\t0\t5\t50\t100\t200\t1\t2\ti1\ti2\ti3\ti4\ti5\th1\th2\th3\th4\t100\n" +
		"awg0\tpeer-key\tpsk\t198.51.100.1:1234\t10.8.0.2/32\t1720000000\t123\t456\t25\n" +
		"awg1\tprivate\tpublic\t51821\t0\t5\t50\t100\t200\t1\t2\ti1\ti2\ti3\ti4\ti5\th1\th2\th3\th4\t100\n" +
		"awg1\tsecond-peer\t(none)\t(none)\t10.8.1.2/32\t0\t0\t0\toff\n")

	got := parseDump(dump, true)
	if len(got) != 2 {
		t.Fatalf("got %d peers, want 2", len(got))
	}
	peer := got["peer-key"]
	if peer.RxBytes != 123 || peer.TxBytes != 456 || peer.Endpoint != "198.51.100.1:1234" {
		t.Fatalf("unexpected peer: %+v", peer)
	}
	wantHandshake := time.Unix(1720000000, 0)
	if peer.LatestHandshake == nil || !peer.LatestHandshake.Equal(wantHandshake) {
		t.Fatalf("handshake = %v, want %v", peer.LatestHandshake, wantHandshake)
	}
	if got["second-peer"].LatestHandshake != nil {
		t.Fatal("zero handshake must remain nil")
	}
}
