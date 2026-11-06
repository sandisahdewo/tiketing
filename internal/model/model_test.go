package model

import "testing"

func TestItemGroups(t *testing.T) {
	r := Report{Tickets: []ReportTicketRow{
		{TicketName: "Saldo", Category: CategoryTopup, Qty: 1, Total: 500},
		{TicketName: "Entry", Category: CategoryTicket, Qty: 2, Total: 200},
		{TicketName: "Anak", Category: CategoryTicket, Qty: 3, Total: 300},
	}}
	groups := r.ItemGroups()
	if len(groups) != 2 {
		t.Fatalf("groups: %+v", groups)
	}
	// Tickets come first, and each group keeps the order of the report.
	tiket, topup := groups[0], groups[1]
	if tiket.Label != "Tiket" || tiket.Qty != 5 || tiket.Total != 500 || tiket.Items[0].TicketName != "Entry" {
		t.Errorf("tiket: %+v", tiket)
	}
	if topup.Label != "Topup Card" || topup.Qty != 1 || topup.Total != 500 {
		t.Errorf("topup: %+v", topup)
	}

	r.Tickets = r.Tickets[1:]
	if groups := r.ItemGroups(); len(groups) != 1 || groups[0].Label != "Tiket" {
		t.Errorf("an empty category stays out: %+v", groups)
	}
}

func TestPeriodLabel(t *testing.T) {
	cases := map[[2]string]string{
		{"2026-10-05", "daily"}:  "Senin, 05-10-2026",
		{"2026-10-05", "weekly"}: "05-10-2026 - 11-10-2026",
		{"2026-10", "monthly"}:   "Oktober 2026",
	}
	for in, want := range cases {
		if got := PeriodLabel(in[0], in[1]); got != want {
			t.Errorf("PeriodLabel(%v) = %q, want %q", in, got, want)
		}
	}
}
