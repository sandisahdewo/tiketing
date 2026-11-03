//go:build windows

package printer

import (
	"errors"
	"fmt"
	"log/slog"
	"syscall"
	"time"
	"unsafe"

	ap "github.com/alexbrainman/printer"
	"golang.org/x/sys/windows"
)

func List(_ string) ([]string, error) {
	return ap.ReadNames()
}

func Default() string {
	name, _ := ap.Default()
	return name
}

var (
	winspool         = windows.NewLazySystemDLL("winspool.drv")
	procOpenPrinter  = winspool.NewProc("OpenPrinterW")
	procClosePrinter = winspool.NewProc("ClosePrinter")
	procGetPrinter   = winspool.NewProc("GetPrinterW")
	procGetDriver    = winspool.NewProc("GetPrinterDriverW")
	procStartDoc     = winspool.NewProc("StartDocPrinterW")
	procStartPage    = winspool.NewProc("StartPagePrinter")
	procWritePrinter = winspool.NewProc("WritePrinter")
	procEndPage      = winspool.NewProc("EndPagePrinter")
	procEndDoc       = winspool.NewProc("EndDocPrinter")
	procGetJob       = winspool.NewProc("GetJobW")
	procSetJob       = winspool.NewProc("SetJobW")
)

const (
	// How long a job is watched after it is spooled. A receipt normally
	// leaves the queue well within a second.
	jobTimeout  = 8 * time.Second
	jobInterval = 150 * time.Millisecond
	// The spooler retries a port that failed once, so a job error only
	// counts once it has lasted this long.
	jobFailAfter = time.Second

	jobControlDelete = 5
	driverXPS        = 0x00000002 // PRINTER_DRIVER_XPS
)

type docInfo1 struct {
	DocName    *uint16
	OutputFile *uint16
	Datatype   *uint16
}

type printerInfo2 struct {
	ServerName         *uint16
	PrinterName        *uint16
	ShareName          *uint16
	PortName           *uint16
	DriverName         *uint16
	Comment            *uint16
	Location           *uint16
	DevMode            uintptr
	SepFile            *uint16
	PrintProcessor     *uint16
	Datatype           *uint16
	Parameters         *uint16
	SecurityDescriptor uintptr
	Attributes         uint32
	Priority           uint32
	DefaultPriority    uint32
	StartTime          uint32
	UntilTime          uint32
	Status             uint32
	Jobs               uint32
	AveragePPM         uint32
}

type jobInfo1 struct {
	JobID        uint32
	PrinterName  *uint16
	MachineName  *uint16
	UserName     *uint16
	Document     *uint16
	Datatype     *uint16
	Status       *uint16
	StatusCode   uint32
	Priority     uint32
	Position     uint32
	TotalPages   uint32
	PagesPrinted uint32
	Submitted    windows.Systemtime
}

// winErr turns the error of a failed winspool call into a real error even if
// the call left GetLastError at 0.
func winErr(err error) error {
	if errno, ok := err.(syscall.Errno); ok && errno != 0 {
		return errno
	}
	return syscall.EINVAL
}

// query runs a winspool Get* call that fills a caller-sized buffer, growing
// the buffer until it fits.
func query(get func(buf *byte, n uint32, needed *uint32) (uintptr, error)) ([]byte, error) {
	var needed uint32
	if r, err := get(nil, 0, &needed); r != 0 {
		return nil, errors.New("winspool: no data")
	} else if err := winErr(err); err != windows.ERROR_INSUFFICIENT_BUFFER {
		return nil, err
	}
	for range 4 {
		buf := make([]byte, needed)
		r, err := get(&buf[0], needed, &needed)
		if r != 0 {
			return buf, nil
		}
		if err := winErr(err); err != windows.ERROR_INSUFFICIENT_BUFFER {
			return nil, err
		}
	}
	return nil, windows.ERROR_INSUFFICIENT_BUFFER
}

type handle windows.Handle

func open(name string) (handle, error) {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	var h windows.Handle
	r, _, e := procOpenPrinter.Call(uintptr(unsafe.Pointer(n)), uintptr(unsafe.Pointer(&h)), 0)
	if r == 0 {
		return 0, winErr(e)
	}
	return handle(h), nil
}

func (h handle) close() {
	procClosePrinter.Call(uintptr(h))
}

func (h handle) queue() (queueInfo, error) {
	buf, err := query(func(b *byte, n uint32, needed *uint32) (uintptr, error) {
		r, _, e := procGetPrinter.Call(uintptr(h), 2, uintptr(unsafe.Pointer(b)), uintptr(n), uintptr(unsafe.Pointer(needed)))
		return r, e
	})
	if err != nil {
		return queueInfo{}, err
	}
	pi := (*printerInfo2)(unsafe.Pointer(&buf[0]))
	return queueInfo{
		Port:       windows.UTF16PtrToString(pi.PortName),
		Driver:     windows.UTF16PtrToString(pi.DriverName),
		Attributes: pi.Attributes,
		Status:     pi.Status,
		Jobs:       pi.Jobs,
	}, nil
}

// xpsDriver reports whether the driver is XPS based (a v4 driver), which
// passes raw data through only with the XPS_PASS data type.
func (h handle) xpsDriver() (bool, error) {
	buf, err := query(func(b *byte, n uint32, needed *uint32) (uintptr, error) {
		r, _, e := procGetDriver.Call(uintptr(h), 0, 8, uintptr(unsafe.Pointer(b)), uintptr(n), uintptr(unsafe.Pointer(needed)))
		return r, e
	})
	if err != nil {
		return false, err
	}
	di := (*ap.DRIVER_INFO_8)(unsafe.Pointer(&buf[0]))
	return di.PrinterDriverAttributes&driverXPS != 0, nil
}

func (h handle) startDoc(name, datatype string) (uint32, error) {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	dt, err := windows.UTF16PtrFromString(datatype)
	if err != nil {
		return 0, err
	}
	d := docInfo1{DocName: n, Datatype: dt}
	r, _, e := procStartDoc.Call(uintptr(h), 1, uintptr(unsafe.Pointer(&d)))
	if r == 0 {
		return 0, winErr(e)
	}
	return uint32(r), nil
}

func (h handle) write(data []byte) error {
	for len(data) > 0 {
		var n uint32
		r, _, e := procWritePrinter.Call(uintptr(h), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&n)))
		if r == 0 {
			return winErr(e)
		}
		if n == 0 {
			return errors.New("printer tidak menerima data")
		}
		data = data[n:]
	}
	return nil
}

func (h handle) call(p *windows.LazyProc) error {
	if r, _, e := p.Call(uintptr(h)); r == 0 {
		return winErr(e)
	}
	return nil
}

// job returns the spooled job, or ok=false once it has left the queue.
func (h handle) job(id uint32) (j job, ok bool, err error) {
	buf, err := query(func(b *byte, n uint32, needed *uint32) (uintptr, error) {
		r, _, e := procGetJob.Call(uintptr(h), uintptr(id), 1, uintptr(unsafe.Pointer(b)), uintptr(n), uintptr(unsafe.Pointer(needed)))
		return r, e
	})
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return job{}, false, nil
	}
	if err != nil {
		return job{}, false, err
	}
	ji := (*jobInfo1)(unsafe.Pointer(&buf[0]))
	j = job{Status: ji.StatusCode, Position: ji.Position}
	if ji.Status != nil {
		j.Text = windows.UTF16PtrToString(ji.Status)
	}
	return j, true, nil
}

func (h handle) cancel(id uint32) {
	procSetJob.Call(uintptr(h), uintptr(id), 0, 0, jobControlDelete)
}

// Print sends ESC/POS data to the printer as a raw job and waits until the job
// has reached the printer, so a job stuck in the Windows queue is reported as
// a failure instead of "sent".
func Print(name string, data []byte, _ string) (string, error) {
	h, err := open(name)
	if err != nil {
		slog.Warn("printer: OpenPrinter", "printer", name, "error", err.Error())
		return "", explain("printer tidak bisa dibuka", err)
	}
	defer h.close()

	q, qErr := h.queue()
	log := slog.With("printer", name, "port", q.Port, "driver", q.Driver,
		"status", fmt.Sprintf("%#x", q.Status), "attrs", fmt.Sprintf("%#x", q.Attributes), "queued", q.Jobs)
	if qErr != nil {
		log.Warn("printer: GetPrinter", "error", qErr.Error())
	} else if b := q.blocker(); b != "" {
		log.Warn("printer: blocked", "reason", b)
		return "", errors.New(b)
	}

	// The driver decides the data type; if that cannot be read (e.g. some
	// shared printers) or the spooler rejects it, the other one is tried.
	datatypes := []string{"RAW", "XPS_PASS"}
	if xps, err := h.xpsDriver(); err != nil {
		log.Warn("printer: GetPrinterDriver", "error", err.Error())
	} else if xps {
		datatypes = []string{"XPS_PASS", "RAW"}
	}
	var id uint32
	var datatype string
	for _, dt := range datatypes {
		if id, err = h.startDoc("Nota Tiket", dt); err == nil {
			datatype = dt
			break
		}
		log.Warn("printer: StartDocPrinter", "datatype", dt, "error", err.Error())
	}
	if err != nil {
		return "", explain("printer menolak dokumen", err)
	}
	log = log.With("job", id, "datatype", datatype, "bytes", len(data))

	if err := h.call(procStartPage); err != nil {
		h.call(procEndDoc)
		h.cancel(id)
		log.Warn("printer: StartPagePrinter", "error", err.Error())
		return "", explain("printer menolak dokumen", err)
	}
	if err := h.write(data); err != nil {
		h.call(procEndPage)
		h.call(procEndDoc)
		h.cancel(id)
		log.Warn("printer: WritePrinter", "error", err.Error())
		return "", explain("gagal mengirim data ke printer", err)
	}
	if err := h.call(procEndPage); err != nil {
		h.call(procEndDoc)
		h.cancel(id)
		log.Warn("printer: EndPagePrinter", "error", err.Error())
		return "", explain("gagal mengirim data ke printer", err)
	}
	if err := h.call(procEndDoc); err != nil {
		h.cancel(id)
		log.Warn("printer: EndDocPrinter", "error", err.Error())
		return "", explain("gagal mengirim data ke printer", err)
	}

	if reason := h.watch(id, log); reason != "" {
		h.cancel(id)
		if q.Port != "" {
			reason += " [port " + q.Port + "]"
		}
		return "", errors.New(reason + ". Cetakan dibatalkan dari antrean Windows, silakan cetak lagi setelah diperbaiki")
	}
	return "", nil
}

// watch follows a spooled job and returns why it cannot print, or "" once it
// has reached the printer (or cannot be followed).
func (h handle) watch(id uint32, log *slog.Logger) string {
	deadline := time.Now().Add(jobTimeout)
	var failingSince time.Time
	for {
		j, ok, err := h.job(id)
		switch {
		case err != nil:
			log.Warn("printer: GetJob", "error", err.Error())
			return ""
		case !ok:
			log.Info("printer: job done")
			return ""
		case j.done():
			log.Info("printer: job done", "jobStatus", fmt.Sprintf("%#x", j.Status))
			return ""
		}
		if f := j.failure(); f == "" {
			failingSince = time.Time{}
		} else if failingSince.IsZero() {
			failingSince = time.Now()
		} else if time.Since(failingSince) >= jobFailAfter || time.Now().After(deadline) {
			log.Warn("printer: job failed", "jobStatus", fmt.Sprintf("%#x", j.Status), "text", j.Text, "reason", f)
			return f
		}
		if time.Now().After(deadline) {
			q, _ := h.queue()
			reason := j.stalled(q)
			log.Warn("printer: job stalled", "jobStatus", fmt.Sprintf("%#x", j.Status), "text", j.Text,
				"position", j.Position, "printerStatus", fmt.Sprintf("%#x", q.Status), "reason", reason)
			return reason
		}
		time.Sleep(jobInterval)
	}
}
