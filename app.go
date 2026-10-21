package main

import (
	"context"
	"encoding/base64"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pkg/browser"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"tiket/internal/config"
	"tiket/internal/db"
	"tiket/internal/export"
	"tiket/internal/model"
	"tiket/internal/printer"
	"tiket/internal/receipt"
	"tiket/internal/server"
	"tiket/internal/service"
	"tiket/internal/store"
	"tiket/internal/updater"
	"tiket/internal/version"
)

type App struct {
	ctx     context.Context
	mu      sync.Mutex
	dataDir string
	cfg     config.Config

	svc      service.Service
	conn     *sql.DB
	httpSrv  *http.Server
	startErr string

	token string
	user  *model.User

	pendingUpdate *updater.Manifest
}

type AppInfo struct {
	Version     string   `json:"version"`
	Mode        string   `json:"mode"`
	Ready       bool     `json:"ready"`
	StartError  string   `json:"startError"`
	Terminal    string   `json:"terminal"`
	ServerURL   string   `json:"serverUrl"`
	Port        int      `json:"port"`
	Addresses   []string `json:"addresses"`
	DataDir     string   `json:"dataDir"`
	InvoiceCode string   `json:"invoiceCode"`
	PrinterName string   `json:"printerName"`
	PaperWidth  int      `json:"paperWidth"`
	OS          string   `json:"os"`
}

func NewApp(dataDir string) *App {
	return &App{dataDir: dataDir}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cfg, err := config.Load(a.dataDir)
	if err != nil {
		a.startErr = "Gagal membaca konfigurasi: " + err.Error()
		return
	}
	a.cfg = cfg
	if cfg.Mode != "" {
		if err := a.activate(); err != nil {
			a.startErr = err.Error()
		}
	}
}

func (a *App) shutdown(_ context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deactivate()
}

func (a *App) activate() error {
	switch a.cfg.Mode {
	case config.ModeServer:
		return a.startServer()
	case config.ModeClient:
		a.svc = service.NewRemote(a.cfg.ServerURL, appVersion)
		return nil
	}
	return errors.New("mode aplikasi belum dipilih")
}

func (a *App) startServer() error {
	conn, err := db.Open(filepath.Join(a.dataDir, "tiket.db"))
	if err != nil {
		return fmt.Errorf("Gagal membuka database: %w", err)
	}
	backupDir := filepath.Join(a.dataDir, "backups")
	if err := db.Migrate(conn, backupDir); err != nil {
		conn.Close()
		return fmt.Errorf("Gagal migrasi database: %w", err)
	}
	if err := db.BackupDaily(conn, backupDir); err != nil {
		log.Printf("backup harian gagal: %v", err)
	}
	local := service.NewLocal(store.New(conn), appVersion)
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", a.cfg.Port))
	if err != nil {
		conn.Close()
		return fmt.Errorf("Port %d sudah dipakai program lain. Ganti port di pengaturan. (%v)", a.cfg.Port, err)
	}
	srv := &http.Server{
		Handler:           server.Handler(local, appVersion),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server berhenti: %v", err)
		}
	}()
	a.conn, a.httpSrv, a.svc = conn, srv, local
	return nil
}

func (a *App) deactivate() {
	if a.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		a.httpSrv.Shutdown(ctx)
		cancel()
		a.httpSrv = nil
	}
	if a.conn != nil {
		a.conn.Close()
		a.conn = nil
	}
	a.svc = nil
	a.token, a.user = "", nil
}

func (a *App) service() (service.Service, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.svc == nil {
		if a.startErr != "" {
			return nil, "", errors.New(a.startErr)
		}
		return nil, "", errors.New("Aplikasi belum diatur")
	}
	return a.svc, a.token, nil
}

func (a *App) check(err error) error {
	if errors.Is(err, model.ErrUnauthorized) {
		a.mu.Lock()
		a.token, a.user = "", nil
		a.mu.Unlock()
	}
	return err
}

func lanAddresses() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				out = append(out, ipnet.IP.String())
			}
		}
	}
	return out
}

func defaultTerminal() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "LOKET 1"
	}
	return strings.ToUpper(name)
}

func (a *App) GetAppInfo() AppInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	terminal := a.cfg.Terminal
	if terminal == "" {
		terminal = defaultTerminal()
	}
	printerName := a.cfg.PrinterName
	if printerName == "" {
		printerName = printer.Default()
	}
	return AppInfo{
		Version:     appVersion,
		Mode:        a.cfg.Mode,
		Ready:       a.svc != nil,
		StartError:  a.startErr,
		Terminal:    terminal,
		ServerURL:   a.cfg.ServerURL,
		Port:        a.cfg.Port,
		Addresses:   lanAddresses(),
		DataDir:     a.dataDir,
		InvoiceCode: a.cfg.InvoiceCode,
		PrinterName: printerName,
		PaperWidth:  a.cfg.PaperWidth,
		OS:          runtime.GOOS,
	}
}

func cleanTerminal(terminal string) (string, error) {
	terminal = strings.TrimSpace(terminal)
	if terminal == "" {
		return "", model.Invalid("Nama terminal/loket wajib diisi")
	}
	if len([]rune(terminal)) > 30 {
		return "", model.Invalid("Nama terminal maksimal 30 karakter")
	}
	return terminal, nil
}

func (a *App) SetupServer(port int, terminal string) error {
	terminal, err := cleanTerminal(terminal)
	if err != nil {
		return err
	}
	if port < 1024 || port > 65535 {
		return model.Invalid("Port harus antara 1024 dan 65535")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deactivate()
	cfg := a.cfg
	cfg.Mode, cfg.Port, cfg.Terminal, cfg.ServerURL = config.ModeServer, port, terminal, ""
	a.cfg = cfg
	if err := a.activate(); err != nil {
		a.cfg.Mode = ""
		a.deactivate()
		return err
	}
	a.startErr = ""
	return config.Save(a.dataDir, a.cfg)
}

func (a *App) TestServer(serverURL string) (model.Status, error) {
	if strings.TrimSpace(serverURL) == "" {
		return model.Status{}, model.Invalid("Alamat server wajib diisi")
	}
	st, err := service.NewRemote(serverURL, appVersion).Status()
	if err != nil {
		return st, err
	}
	if !version.Compatible(st.Version, appVersion) {
		return st, fmt.Errorf("Versi server (%s) tidak cocok dengan aplikasi ini (%s). Update aplikasi di kedua PC ke versi yang sama.", st.Version, appVersion)
	}
	return st, nil
}

func (a *App) SetupClient(serverURL, terminal string) error {
	terminal, err := cleanTerminal(terminal)
	if err != nil {
		return err
	}
	if _, err := a.TestServer(serverURL); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deactivate()
	a.cfg.Mode, a.cfg.ServerURL, a.cfg.Terminal = config.ModeClient, service.NormalizeServerURL(serverURL), terminal
	if err := a.activate(); err != nil {
		return err
	}
	a.startErr = ""
	return config.Save(a.dataDir, a.cfg)
}

func (a *App) ResetSetup() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.user != nil && a.user.Role != model.RoleAdmin {
		return model.ErrForbidden
	}
	a.deactivate()
	a.cfg.Mode = ""
	a.startErr = ""
	return config.Save(a.dataDir, a.cfg)
}

func (a *App) Status() (model.Status, error) {
	svc, _, err := a.service()
	if err != nil {
		return model.Status{}, err
	}
	return svc.Status()
}

func (a *App) CreateFirstAdmin(in model.UserInput) error {
	svc, _, err := a.service()
	if err != nil {
		return err
	}
	return svc.CreateFirstAdmin(in)
}

func (a *App) Login(username, password string) (model.User, error) {
	svc, _, err := a.service()
	if err != nil {
		return model.User{}, err
	}
	sess, err := svc.Login(username, password)
	if err != nil {
		return model.User{}, err
	}
	a.mu.Lock()
	a.token, a.user = sess.Token, &sess.User
	a.mu.Unlock()
	return sess.User, nil
}

func (a *App) Logout() error {
	svc, token, err := a.service()
	if err == nil && token != "" {
		svc.Logout(token)
	}
	a.mu.Lock()
	a.token, a.user = "", nil
	a.mu.Unlock()
	return nil
}

func (a *App) CurrentUser() *model.User {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.user
}

func (a *App) ListTickets(search string, deleted bool) ([]model.Ticket, error) {
	svc, token, err := a.service()
	if err != nil {
		return nil, err
	}
	out, err := svc.ListTickets(token, search, deleted)
	return out, a.check(err)
}

func (a *App) SaveTicket(in model.TicketInput) (model.Ticket, error) {
	svc, token, err := a.service()
	if err != nil {
		return model.Ticket{}, err
	}
	out, err := svc.SaveTicket(token, in)
	return out, a.check(err)
}

func (a *App) DeleteTicket(id int64) error {
	svc, token, err := a.service()
	if err != nil {
		return err
	}
	return a.check(svc.DeleteTicket(token, id))
}

func (a *App) RestoreTicket(id int64) error {
	svc, token, err := a.service()
	if err != nil {
		return err
	}
	return a.check(svc.RestoreTicket(token, id))
}

func (a *App) ListUsers() ([]model.User, error) {
	svc, token, err := a.service()
	if err != nil {
		return nil, err
	}
	out, err := svc.ListUsers(token)
	return out, a.check(err)
}

func (a *App) SaveUser(in model.UserInput) (model.User, error) {
	svc, token, err := a.service()
	if err != nil {
		return model.User{}, err
	}
	out, err := svc.SaveUser(token, in)
	if err == nil {
		a.mu.Lock()
		if a.user != nil && a.user.ID == out.ID {
			a.user = &out
		}
		a.mu.Unlock()
	}
	return out, a.check(err)
}

func (a *App) DeleteUser(id int64) error {
	svc, token, err := a.service()
	if err != nil {
		return err
	}
	return a.check(svc.DeleteUser(token, id))
}

func (a *App) CreateSale(items []model.SaleItemInput, paid int64) (model.Sale, error) {
	svc, token, err := a.service()
	if err != nil {
		return model.Sale{}, err
	}
	a.mu.Lock()
	terminal, code := a.cfg.Terminal, a.cfg.InvoiceCode
	a.mu.Unlock()
	out, err := svc.CreateSale(token, model.SaleInput{Terminal: terminal, InvoiceCode: code, Items: items, Paid: paid})
	return out, a.check(err)
}

func (a *App) GetSale(id int64) (model.Sale, error) {
	svc, token, err := a.service()
	if err != nil {
		return model.Sale{}, err
	}
	out, err := svc.GetSale(token, id)
	return out, a.check(err)
}

func (a *App) ListSales(q model.SaleQuery) ([]model.Sale, error) {
	svc, token, err := a.service()
	if err != nil {
		return nil, err
	}
	out, err := svc.ListSales(token, q)
	return out, a.check(err)
}

func (a *App) DeleteSale(id int64, reason string) error {
	svc, token, err := a.service()
	if err != nil {
		return err
	}
	return a.check(svc.DeleteSale(token, model.DeleteSaleInput{ID: id, Reason: reason}))
}

func (a *App) Report(q model.ReportQuery) (model.Report, error) {
	svc, token, err := a.service()
	if err != nil {
		return model.Report{}, err
	}
	out, err := svc.Report(token, q)
	return out, a.check(err)
}

// ExportReport saves the report as "pdf" or "xlsx" through a save dialog and
// opens the file. It returns the saved path, or "" when the dialog was cancelled.
func (a *App) ExportReport(q model.ReportQuery, format string) (string, error) {
	svc, token, err := a.service()
	if err != nil {
		return "", err
	}
	report, err := svc.Report(token, q)
	if err != nil {
		return "", a.check(err)
	}
	sales, err := svc.ListSales(token, model.SaleQuery{From: report.From, To: report.To})
	if err != nil {
		return "", a.check(err)
	}
	settings, err := svc.GetSettings(token)
	if err != nil {
		return "", a.check(err)
	}
	data := export.Data{Settings: settings, Report: report, Sales: sales, PrintedAt: time.Now()}
	if u := a.CurrentUser(); u != nil {
		data.PrintedBy = u.FullName
	}

	var body []byte
	var filter wruntime.FileFilter
	switch format {
	case "pdf":
		body, err = export.PDF(data)
		filter = wruntime.FileFilter{DisplayName: "PDF (*.pdf)", Pattern: "*.pdf"}
	case "xlsx":
		body, err = export.Excel(data)
		filter = wruntime.FileFilter{DisplayName: "Excel (*.xlsx)", Pattern: "*.xlsx"}
	default:
		return "", model.Invalid("Format laporan tidak valid")
	}
	if err != nil {
		return "", err
	}

	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:           "Simpan Laporan",
		DefaultFilename: export.FileName(report, format),
		Filters:         []wruntime.FileFilter{filter},
	})
	if err != nil || path == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(path), "."+format) {
		path += "." + format
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", fmt.Errorf("gagal menyimpan laporan: %w", err)
	}
	if err := browser.OpenFile(path); err != nil {
		log.Printf("open %s: %v", path, err)
	}
	return path, nil
}

func (a *App) GetSettings() (model.Settings, error) {
	svc, token, err := a.service()
	if err != nil {
		return model.Settings{}, err
	}
	out, err := svc.GetSettings(token)
	return out, a.check(err)
}

func (a *App) SaveSettings(in model.Settings) error {
	svc, token, err := a.service()
	if err != nil {
		return err
	}
	return a.check(svc.SaveSettings(token, in))
}

type ReceiptView struct {
	Lines        []receipt.Line `json:"lines"`
	Logo         string         `json:"logo"`
	PrintCount   int64          `json:"printCount"`
	Deleted      bool           `json:"deleted"`
	DeleteReason string         `json:"deleteReason"`
}

func (a *App) paperWidth() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg.PaperWidth
}

func (a *App) logo(st model.Settings) (*receipt.Bitmap, error) {
	if !st.HasLogo {
		return nil, nil
	}
	svc, token, err := a.service()
	if err != nil {
		return nil, err
	}
	data, err := svc.GetLogo(token)
	if err != nil || data == "" {
		return nil, a.check(err)
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	return receipt.PrepareLogo(raw, a.paperWidth())
}

func (a *App) receiptFor(saleID int64) (model.Sale, []receipt.Line, *receipt.Bitmap, error) {
	sale, err := a.GetSale(saleID)
	if err != nil {
		return sale, nil, nil, err
	}
	st, err := a.GetSettings()
	if err != nil {
		return sale, nil, nil, err
	}
	logo, err := a.logo(st)
	if err != nil {
		return sale, nil, nil, fmt.Errorf("Gagal memuat logo: %w", err)
	}
	return sale, receipt.Build(st, sale, a.paperWidth()), logo, nil
}

func logoDataURL(logo *receipt.Bitmap) string {
	if logo == nil {
		return ""
	}
	data, err := logo.PNG()
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

func (a *App) ReceiptPreview(saleID int64) (ReceiptView, error) {
	sale, lines, logo, err := a.receiptFor(saleID)
	if err != nil {
		return ReceiptView{}, err
	}
	return ReceiptView{
		Lines:        lines,
		Logo:         logoDataURL(logo),
		PrintCount:   sale.PrintCount,
		Deleted:      sale.DeletedAt != nil,
		DeleteReason: sale.DeleteReason,
	}, nil
}

func (a *App) printBytes(data []byte) (string, error) {
	a.mu.Lock()
	name := a.cfg.PrinterName
	a.mu.Unlock()
	if name == "" {
		name = printer.Default()
	}
	if name == "" {
		return "", model.Invalid("Printer belum dipilih. Atur di menu Pengaturan.")
	}
	path, err := printer.Print(name, data, a.dataDir)
	if err != nil {
		return "", fmt.Errorf("Gagal mencetak ke %s: %w", name, err)
	}
	if path != "" {
		return "Nota disimpan ke " + path, nil
	}
	return "Nota dikirim ke " + name, nil
}

func (a *App) PrintReceipt(saleID int64) (string, error) {
	sale, lines, logo, err := a.receiptFor(saleID)
	if err != nil {
		return "", err
	}
	if sale.DeletedAt != nil {
		return "", model.Invalid("Nota sudah dihapus, tidak bisa dicetak")
	}
	msg, err := a.printBytes(receipt.ESCPOS(lines, logo))
	if err != nil {
		return "", err
	}
	svc, token, err := a.service()
	if err != nil {
		return msg, err
	}
	a.mu.Lock()
	terminal := a.cfg.Terminal
	a.mu.Unlock()
	if err := svc.RecordPrint(token, model.PrintInput{SaleID: saleID, Terminal: terminal}); err != nil {
		return msg, a.check(fmt.Errorf("Nota tercetak, tapi gagal mencatat cetakan: %w", err))
	}
	return msg, nil
}

func (a *App) PrintTest() (string, error) {
	st, err := a.GetSettings()
	if err != nil {
		st = model.Settings{}
	}
	logo, err := a.logo(st)
	if err != nil {
		return "", fmt.Errorf("Gagal memuat logo: %w", err)
	}
	a.mu.Lock()
	name := a.cfg.PrinterName
	a.mu.Unlock()
	if name == "" {
		name = printer.Default()
	}
	return a.printBytes(receipt.ESCPOS(receipt.BuildTest(st, a.paperWidth(), name), logo))
}

func (a *App) LogoPreview() (string, error) {
	st, err := a.GetSettings()
	if err != nil {
		return "", err
	}
	logo, err := a.logo(st)
	if err != nil {
		return "", err
	}
	return logoDataURL(logo), nil
}

func (a *App) ChooseLogo() (bool, error) {
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Pilih logo nota",
		Filters: []wruntime.FileFilter{
			{DisplayName: "Gambar (*.png;*.jpg;*.jpeg;*.gif)", Pattern: "*.png;*.jpg;*.jpeg;*.gif"},
		},
	})
	if err != nil || path == "" {
		return false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if info.Size() > receipt.MaxLogoBytes {
		return false, model.Invalid("Ukuran logo maksimal 1 MB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if err := receipt.ValidateLogo(data); err != nil {
		return false, model.Invalid(err.Error())
	}
	return true, a.saveLogo(base64.StdEncoding.EncodeToString(data))
}

func (a *App) RemoveLogo() error {
	return a.saveLogo("")
}

func (a *App) saveLogo(data string) error {
	svc, token, err := a.service()
	if err != nil {
		return err
	}
	return a.check(svc.SaveLogo(token, data))
}

func (a *App) ListPrinters() ([]string, error) {
	return printer.List(a.dataDir)
}

func (a *App) SaveLocalSettings(printerName string, paperWidth int, terminal, invoiceCode string) error {
	terminal, err := cleanTerminal(terminal)
	if err != nil {
		return err
	}
	code, err := service.ValidateInvoiceCode(invoiceCode)
	if err != nil {
		return err
	}
	if paperWidth != 58 && paperWidth != 80 {
		return model.Invalid("Lebar kertas harus 58 atau 80 mm")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.PrinterName, a.cfg.PaperWidth, a.cfg.Terminal, a.cfg.InvoiceCode = printerName, paperWidth, terminal, code
	return config.Save(a.dataDir, a.cfg)
}

func (a *App) CheckUpdate() (updater.Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	info, m, err := updater.Check(ctx, appVersion)
	if err != nil {
		return info, err
	}
	a.mu.Lock()
	if info.Available {
		a.pendingUpdate = m
	} else {
		a.pendingUpdate = nil
	}
	a.mu.Unlock()
	return info, nil
}

func (a *App) ApplyUpdate() error {
	a.mu.Lock()
	m := a.pendingUpdate
	a.mu.Unlock()
	if m == nil {
		return model.Invalid("Tidak ada update yang tersedia. Klik Cek Update dulu.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := updater.DownloadAndApply(ctx, *m); err != nil {
		return err
	}
	if err := updater.Restart(afterUpdateFlag); err != nil {
		return fmt.Errorf("Update terpasang, tapi gagal membuka ulang aplikasi. Silakan buka manual. (%v)", err)
	}
	wruntime.Quit(a.ctx)
	return nil
}
