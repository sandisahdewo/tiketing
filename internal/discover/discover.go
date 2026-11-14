// Package discover finds the Tiket server on the networks this PC is on, so
// a client can pick it instead of typing its address.
//
// It asks every address of the local network for /api/status on the server
// port: the same request a client makes, so a server that is found can also
// be used, and one that is not found is missing for the same reasons a
// client could not connect to it.
package discover

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"tiket/internal/model"
	"tiket/internal/version"
)

type Server struct {
	// Address is what a client connects to: host:port.
	Address  string `json:"address"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	HasUsers bool   `json:"hasUsers"`
	// Compatible is false when the server's version does not work with this
	// app, so it is shown but cannot be chosen.
	Compatible bool `json:"compatible"`
}

// Networks lists the IPv4 networks this PC is connected to, each with the
// PC's own address in it.
func Networks() []*net.IPNet {
	var out []*net.IPNet
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		// Up and with a cable or WiFi connected.
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagRunning == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				out = append(out, ipnet)
			}
		}
	}
	return out
}

// hosts lists the addresses to ask on a network. A network larger than 254
// addresses is not searched whole: only the 254 around this PC.
func hosts(n *net.IPNet) []net.IP {
	ip := n.IP.To4()
	ones, bits := n.Mask.Size()
	if ip == nil || bits != 32 || ones > 30 {
		return nil
	}
	if ones < 24 {
		ones = 24
	}
	base := binary.BigEndian.Uint32(ip.Mask(net.CIDRMask(ones, 32)))
	size := uint32(1) << (32 - ones)
	out := make([]net.IP, 0, size-2)
	// Without the first (network) and last (broadcast) address.
	for i := uint32(1); i < size-1; i++ {
		h := make(net.IP, 4)
		binary.BigEndian.PutUint32(h, base+i)
		out = append(out, h)
	}
	return out
}

// Find asks every address of nets for a Tiket server on port and returns the
// ones that answered, until ctx ends. A server PC that is on two of the
// networks is listed once.
func Find(ctx context.Context, nets []*net.IPNet, port int, appVersion string) []Server {
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext:       (&net.Dialer{Timeout: time.Second}).DialContext,
			DisableKeepAlives: true,
		},
	}
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		found []Server
	)
	asked := map[string]bool{}
	for _, n := range nets {
		for _, ip := range hosts(n) {
			addr := net.JoinHostPort(ip.String(), strconv.Itoa(port))
			if asked[addr] {
				continue
			}
			asked[addr] = true
			wg.Add(1)
			go func() {
				defer wg.Done()
				st, ok := ask(ctx, client, addr)
				if !ok {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				found = append(found, Server{
					Address:    addr,
					Name:       st.Name,
					Version:    st.Version,
					HasUsers:   st.HasUsers,
					Compatible: version.Compatible(st.Version, appVersion),
				})
			}()
		}
	}
	wg.Wait()
	sort.Slice(found, func(i, j int) bool { return found[i].Address < found[j].Address })
	out := []Server{}
	names := map[string]bool{}
	for _, s := range found {
		if s.Name != "" && names[s.Name] {
			continue
		}
		names[s.Name] = true
		out = append(out, s)
	}
	return out
}

// ask reports the status of the Tiket server at addr, if there is one.
func ask(ctx context.Context, client *http.Client, addr string) (model.Status, bool) {
	var st model.Status
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/api/status", nil)
	if err != nil {
		return st, false
	}
	resp, err := client.Do(req)
	if err != nil {
		return st, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return st, false
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&st); err != nil {
		return st, false
	}
	// Something else may listen on this port; only Tiket reports a version.
	return st, version.Valid(st.Version)
}
