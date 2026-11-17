package discover

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func network(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	ip, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatal(err)
	}
	n.IP = ip
	return n
}

func TestHosts(t *testing.T) {
	for _, c := range []struct {
		cidr        string
		count       int
		first, last string
	}{
		{"192.168.1.23/24", 254, "192.168.1.1", "192.168.1.254"},
		{"192.168.1.130/25", 126, "192.168.1.129", "192.168.1.254"},
		// A large network is only searched around this PC.
		{"10.20.30.40/8", 254, "10.20.30.1", "10.20.30.254"},
		{"192.168.1.2/30", 2, "192.168.1.1", "192.168.1.2"},
		// A link between two PCs only (VPN) has no network to search.
		{"10.0.0.1/32", 0, "", ""},
	} {
		got := hosts(network(t, c.cidr))
		if len(got) != c.count {
			t.Errorf("%s: %d hosts, want %d", c.cidr, len(got), c.count)
			continue
		}
		if c.count > 0 && (got[0].String() != c.first || got[len(got)-1].String() != c.last) {
			t.Errorf("%s: %s..%s, want %s..%s", c.cidr, got[0], got[len(got)-1], c.first, c.last)
		}
	}
}

func TestFind(t *testing.T) {
	answer := `{"version":"1.2.3","name":"PC-SERVER","hasUsers":true}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(answer))
	}))
	defer srv.Close()
	_, portText, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	// 127.0.0.1 and 127.0.0.2; the server listens on the first only.
	nets := []*net.IPNet{network(t, "127.0.0.1/30"), network(t, "127.0.0.2/30")}

	got := Find(context.Background(), nets, port, "1.2.0")
	if len(got) != 1 {
		t.Fatalf("found %+v, want one server", got)
	}
	want := Server{Address: "127.0.0.1:" + portText, Name: "PC-SERVER", Version: "1.2.3", HasUsers: true, Compatible: true}
	if got[0] != want {
		t.Errorf("found %+v, want %+v", got[0], want)
	}

	if got := Find(context.Background(), nets, port, "1.3.0"); len(got) != 1 || got[0].Compatible {
		t.Errorf("another version should be listed as not compatible: %+v", got)
	}

	// The address this PC was set up with is asked too, whatever its port.
	if got := Find(context.Background(), nil, 1, "1.2.0", "127.0.0.1:"+portText, ""); len(got) != 1 || got[0].Address != "127.0.0.1:"+portText {
		t.Errorf("the saved address was not found: %+v", got)
	}

	// Another program on the port is not a server.
	answer = `{"hello":"world"}`
	if got := Find(context.Background(), nets, port, "1.2.0"); len(got) != 0 {
		t.Errorf("found %+v, want nothing", got)
	}
}
