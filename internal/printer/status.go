package printer

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
)

// Windows spooler flags (winspool.h). They live here, without a build tag, so
// the decisions made from them are tested on every OS.
const (
	jobStatusPaused           = 0x00000001
	jobStatusError            = 0x00000002
	jobStatusDeleting         = 0x00000004
	jobStatusPrinting         = 0x00000010
	jobStatusOffline          = 0x00000020
	jobStatusPaperOut         = 0x00000040
	jobStatusPrinted          = 0x00000080
	jobStatusDeleted          = 0x00000100
	jobStatusBlockedDevq      = 0x00000200
	jobStatusUserIntervention = 0x00000400
	jobStatusComplete         = 0x00001000
	jobStatusRetained         = 0x00002000

	printerStatusPaused           = 0x00000001
	printerStatusError            = 0x00000002
	printerStatusPaperJam         = 0x00000008
	printerStatusPaperOut         = 0x00000010
	printerStatusPaperProblem     = 0x00000040
	printerStatusOffline          = 0x00000080
	printerStatusNotAvailable     = 0x00001000
	printerStatusUserIntervention = 0x00100000
	printerStatusDoorOpen         = 0x00400000

	printerAttrWorkOffline = 0x00000400
)

// queueInfo is what the spooler says about a printer queue.
type queueInfo struct {
	Port       string
	Driver     string
	Attributes uint32
	Status     uint32
	Jobs       uint32
}

// virtualDrivers print to a file or app, never to a thermal printer. They are
// often the Windows default printer, which is used when none is chosen.
var virtualDrivers = []string{
	"microsoft print to pdf",
	"microsoft xps document writer",
	"microsoft shared fax",
	"send to microsoft onenote",
	"onenote",
	"fax",
}

var virtualPorts = []string{"portprompt:", "shrfax:", "nul:", "nul"}

// blocker says why a job sent to this queue can never come out of a thermal
// printer, or "" if it can.
func (q queueInfo) blocker() string {
	driver := strings.ToLower(strings.TrimSpace(q.Driver))
	port := strings.ToLower(strings.TrimSpace(q.Port))
	for _, v := range virtualDrivers {
		if strings.HasPrefix(driver, v) {
			return fmt.Sprintf("ini printer virtual (%s), bukan printer thermal. Pilih printer thermal di menu Pengaturan", q.Driver)
		}
	}
	for _, v := range virtualPorts {
		if port == v {
			return fmt.Sprintf("printer ini mencetak ke port %s, bukan ke printer thermal. Pilih printer thermal di menu Pengaturan", q.Port)
		}
	}
	if q.Attributes&printerAttrWorkOffline != 0 {
		return `printer disetel "Use Printer Offline" di Windows. Buka antrean printer lalu hilangkan centang menu Printer > Use Printer Offline`
	}
	if q.Status&printerStatusPaused != 0 {
		return "antrean printer sedang di-pause di Windows. Buka antrean printer lalu pilih Printer > Resume"
	}
	return ""
}

// problem describes what the printer itself reports, or "".
func (q queueInfo) problem() string {
	var p []string
	add := func(flag uint32, text string) {
		if q.Status&flag != 0 {
			p = append(p, text)
		}
	}
	add(printerStatusOffline, "printer offline")
	add(printerStatusNotAvailable, "printer tidak tersedia")
	add(printerStatusPaperOut, "kertas habis")
	add(printerStatusPaperJam, "kertas macet")
	add(printerStatusPaperProblem, "masalah kertas")
	add(printerStatusDoorOpen, "tutup printer terbuka")
	add(printerStatusUserIntervention, "printer perlu dicek")
	if len(p) == 0 && q.Status&printerStatusError != 0 {
		p = append(p, "printer error")
	}
	return strings.Join(p, ", ")
}

// job is one spooled document as the spooler reports it.
type job struct {
	Status   uint32
	Text     string // status text from the driver or port monitor, if any
	Position uint32
}

// done reports whether the job reached the printer.
func (j job) done() bool {
	if j.Status&(jobStatusError|jobStatusBlockedDevq) != 0 {
		return false
	}
	return j.Status&(jobStatusPrinted|jobStatusComplete|jobStatusRetained) != 0
}

// failure describes why the job cannot print, or "" if nothing is wrong yet.
func (j job) failure() string {
	var p []string
	add := func(flag uint32, text string) {
		if j.Status&flag != 0 {
			p = append(p, text)
		}
	}
	add(jobStatusOffline, "printer offline")
	add(jobStatusPaperOut, "kertas habis")
	add(jobStatusBlockedDevq, "driver printer menolak data")
	add(jobStatusUserIntervention, "printer perlu dicek")
	add(jobStatusPaused, "dokumen di-pause")
	add(jobStatusDeleting|jobStatusDeleted, "dokumen dibatalkan")
	if len(p) == 0 && j.Status&jobStatusError != 0 {
		p = append(p, "printer error")
	}
	if len(p) == 0 {
		return ""
	}
	return j.withText(strings.Join(p, ", "))
}

func (j job) withText(s string) string {
	if t := strings.TrimSpace(j.Text); t != "" && !strings.EqualFold(t, s) {
		return s + " (" + t + ")"
	}
	return s
}

// stalled explains a job that is still waiting when we stop watching it, or
// returns "" if it is being sent to the printer and only needs more time.
func (j job) stalled(q queueInfo) string {
	if j.Status&jobStatusPrinting != 0 {
		return ""
	}
	if p := q.problem(); p != "" {
		return j.withText(p)
	}
	if j.Position > 1 {
		return fmt.Sprintf("masih ada %d dokumen lain yang macet di antrean printer. Hapus dulu dari antrean Windows", j.Position-1)
	}
	return j.withText("printer tidak merespons. Cek kabel/daya printer dan pastikan printer yang dipilih di Pengaturan sama dengan yang dipakai untuk Print Test Page dari Windows")
}

// Win32 errors the spooler returns (winerror.h), explained for the cashier.
// Windows' own text is in the PC's language and rarely says what to do.
var winMessages = map[uintptr]string{
	5:    "Windows menolak akses ke printer ini. Cek izin printer (tab Security) di Windows",
	21:   "printer tidak siap. Cek kabel USB dan daya printer",
	31:   "printer tidak terhubung. Cek kabel USB dan daya printer",
	1167: "printer tidak terhubung. Cek kabel USB dan daya printer",
	53:   "komputer tempat printer di-share tidak bisa dihubungi. Pastikan komputer itu menyala dan satu jaringan",
	64:   "komputer tempat printer di-share tidak bisa dihubungi. Pastikan komputer itu menyala dan satu jaringan",
	67:   "komputer tempat printer di-share tidak bisa dihubungi. Pastikan komputer itu menyala dan satu jaringan",
	1722: "komputer tempat printer di-share tidak bisa dihubungi. Pastikan komputer itu menyala dan satu jaringan",
	61:   "antrean printer penuh. Hapus dokumen lama dari antrean printer di Windows",
	63:   "cetakan dibatalkan di Windows",
	1797: "driver printer belum terpasang atau rusak. Pasang ulang driver printer",
	1801: "printer ini tidak ada di Windows (mungkin sudah dihapus atau namanya berubah). Pilih ulang printer di Pengaturan",
	1905: "printer ini tidak ada di Windows (mungkin sudah dihapus atau namanya berubah). Pilih ulang printer di Pengaturan",
	3012: "printer ini tidak ada di Windows (mungkin sudah dihapus atau namanya berubah). Pilih ulang printer di Pengaturan",
	1804: "driver printer tidak menerima data langsung (RAW). Pasang driver printer thermal ESC/POS, atau pakai driver Generic / Text Only",
}

// explain turns an error from step into a message for the cashier: the known
// Windows errors say what to check, others keep Windows' text and its code so
// the log and a screenshot show the same thing.
func explain(step string, err error) error {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return fmt.Errorf("%s: %w", step, err)
	}
	if msg := winMessages[uintptr(errno)]; msg != "" {
		return fmt.Errorf("%s (%w)", msg, errorCode{errno})
	}
	text := strings.TrimRight(strings.TrimSpace(errno.Error()), ".")
	return fmt.Errorf("%s: %s (%w)", step, text, errorCode{errno})
}

// errorCode prints as the Windows error code while still matching the errno
// with errors.Is.
type errorCode struct{ errno syscall.Errno }

func (e errorCode) Error() string { return fmt.Sprintf("kode %d", uintptr(e.errno)) }
func (e errorCode) Unwrap() error { return e.errno }
