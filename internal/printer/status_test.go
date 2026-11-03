package printer

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

func TestQueueBlocker(t *testing.T) {
	tests := []struct {
		name string
		q    queueInfo
		want string // substring; "" means not blocked
	}{
		{"thermal on USB", queueInfo{Port: "USB001", Driver: "XP-80C"}, ""},
		{"thermal reporting offline still tried", queueInfo{Port: "USB001", Driver: "XP-80C", Status: printerStatusOffline}, ""},
		{"print to pdf", queueInfo{Port: "PORTPROMPT:", Driver: "Microsoft Print To PDF"}, "printer virtual"},
		{"xps writer", queueInfo{Port: "PORTPROMPT:", Driver: "Microsoft XPS Document Writer v4"}, "printer virtual"},
		{"onenote", queueInfo{Port: "nul:", Driver: "Send to Microsoft OneNote 16 Driver"}, "printer virtual"},
		{"unknown driver on file prompt port", queueInfo{Port: "PORTPROMPT:", Driver: "Generic / Text Only"}, "port PORTPROMPT:"},
		{"use printer offline", queueInfo{Port: "USB001", Driver: "XP-80C", Attributes: printerAttrWorkOffline}, "Use Printer Offline"},
		{"paused queue", queueInfo{Port: "USB001", Driver: "XP-80C", Status: printerStatusPaused}, "di-pause"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.q.blocker()
			if tt.want == "" && got != "" || !strings.Contains(got, tt.want) {
				t.Fatalf("blocker() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJobDoneAndFailure(t *testing.T) {
	tests := []struct {
		name    string
		j       job
		done    bool
		failure string
	}{
		{"queued", job{}, false, ""},
		{"printing", job{Status: jobStatusPrinting}, false, ""},
		{"printed and kept", job{Status: jobStatusPrinted}, true, ""},
		{"delivered (v4)", job{Status: jobStatusComplete}, true, ""},
		{"error while printing", job{Status: jobStatusError | jobStatusPrinting}, false, "printer error"},
		{"offline with driver text", job{Status: jobStatusOffline, Text: "Printer offline"}, false, "printer offline"},
		{"paper out", job{Status: jobStatusPaperOut | jobStatusError}, false, "kertas habis"},
		{"driver rejects", job{Status: jobStatusBlockedDevq | jobStatusComplete}, false, "driver printer menolak data"},
		{"error text from port monitor", job{Status: jobStatusError, Text: "Error - Printing"}, false, "printer error (Error - Printing)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.j.done(); got != tt.done {
				t.Errorf("done() = %v, want %v", got, tt.done)
			}
			got := tt.j.failure()
			if tt.failure == "" && got != "" || !strings.Contains(got, tt.failure) {
				t.Errorf("failure() = %q, want %q", got, tt.failure)
			}
		})
	}
	if got := (job{Status: jobStatusOffline, Text: "Printer Offline"}).failure(); got != "printer offline" {
		t.Errorf("status text equal to the reason is repeated: %q", got)
	}
}

func TestJobStalled(t *testing.T) {
	tests := []struct {
		name string
		j    job
		q    queueInfo
		want string // substring; "" means keep waiting / accept
	}{
		{"still sending", job{Status: jobStatusPrinting}, queueInfo{}, ""},
		{"printer reports offline", job{Position: 1}, queueInfo{Status: printerStatusOffline}, "printer offline"},
		{"paper out and door open", job{Position: 1}, queueInfo{Status: printerStatusPaperOut | printerStatusDoorOpen}, "kertas habis, tutup printer terbuka"},
		{"stuck behind other jobs", job{Position: 3}, queueInfo{}, "masih ada 2 dokumen lain"},
		{"silent printer", job{Position: 1}, queueInfo{}, "printer tidak merespons"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.j.stalled(tt.q)
			if tt.want == "" && got != "" || !strings.Contains(got, tt.want) {
				t.Fatalf("stalled() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExplain(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"printer gone", syscall.Errno(1801), "printer ini tidak ada di Windows (mungkin sudah dihapus atau namanya berubah). Pilih ulang printer di Pengaturan (kode 1801)"},
		{"share host off", syscall.Errno(1722), "komputer tempat printer di-share tidak bisa dihubungi"},
		{"raw rejected", syscall.Errno(1804), "Generic / Text Only (kode 1804)"},
		{"unknown code keeps step and code", syscall.Errno(9999), "printer tidak bisa dibuka: "},
		{"not a Windows error", errors.New("printer tidak menerima data"), "printer tidak bisa dibuka: printer tidak menerima data"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := explain("printer tidak bisa dibuka", tt.err)
			if !strings.Contains(got.Error(), tt.want) {
				t.Fatalf("explain() = %q, want %q", got, tt.want)
			}
			if !errors.Is(got, tt.err) {
				t.Fatalf("explain() lost the original error")
			}
		})
	}
	if got := explain("x", syscall.Errno(9999)).Error(); !strings.HasSuffix(got, "(kode 9999)") {
		t.Fatalf("unknown code not shown: %q", got)
	}
}
