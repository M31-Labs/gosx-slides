package slides

import "testing"

func TestCaptureEndpointWaitsForCompletePortAnnouncement(t *testing.T) {
	// A newly created file, then a partially written port, then the complete
	// announcement: only the last state may be used for a DevTools connection.
	for _, data := range []string{"", "3", "34317", "\n/devtools/browser/id", "0\n", "65536\n", "-1\n", "not-a-port\n"} {
		if port, ready := captureEndpointPort([]byte(data)); ready || port != "" {
			t.Fatalf("accepted incomplete/invalid announcement %q as %q", data, port)
		}
	}
	for _, data := range []string{"34317\n/devtools/browser/id", "34317\r\n/devtools/browser/id", "34317\n"} {
		if port, ready := captureEndpointPort([]byte(data)); !ready || port != "34317" {
			t.Fatalf("rejected complete announcement %q: %q, %v", data, port, ready)
		}
	}
}
