package be

import "testing"

func novaTestnaKartica() *Kartica {
	return &Kartica{
		PIN:   "1234",
		Limit: 500000,
		counters: map[string]int{
			"pp": 0, "pr": 0, "ap": 0, "ar": 0,
			"kp": 0, "kr": 0, "op": 0, "or": 0,
			"rp": 0, "rr": 0,
		},
	}
}

func TestCmdSignBezPinaOdbija(t *testing.T) {
	k := novaTestnaKartica()
	resp := k.cmdSign("Normal", "Sale", 100)
	if resp["status"] != "error" {
		t.Fatalf("cmdSign bez verifikovanog PIN-a mora vratiti status=error, dobijeno: %v", resp)
	}
	if resp["code"] != "2101" {
		t.Errorf("code = %v, očekivano 2101", resp["code"])
	}
	if k.totalCounter != 0 {
		t.Errorf("totalCounter se povećao iako PIN nije verifikovan: %d", k.totalCounter)
	}
}

func TestCmdSignPogresanPinOdbija(t *testing.T) {
	k := novaTestnaKartica()
	if resp := k.cmdVerifyPin("0000"); resp["status"] != "error" {
		t.Fatalf("cmdVerifyPin sa pogrešnim PIN-om mora vratiti error, dobijeno: %v", resp)
	}
	if k.pinUnesen {
		t.Fatal("pinUnesen ne sme biti true posle pogrešnog PIN-a")
	}
	if resp := k.cmdSign("Normal", "Sale", 100); resp["status"] != "error" {
		t.Fatalf("cmdSign posle pogrešnog PIN-a mora ostati blokiran, dobijeno: %v", resp)
	}
}

func TestCmdSignPoslePravogPinaUspeva(t *testing.T) {
	k := novaTestnaKartica()
	if resp := k.cmdVerifyPin("1234"); resp["status"] != "ok" {
		t.Fatalf("cmdVerifyPin sa ispravnim PIN-om mora vratiti ok, dobijeno: %v", resp)
	}
	resp := k.cmdSign("Normal", "Sale", 100)
	if resp["status"] != "ok" {
		t.Fatalf("cmdSign posle ispravnog PIN-a mora uspeti, dobijeno: %v", resp)
	}
	if k.totalCounter != 1 {
		t.Errorf("totalCounter = %d, očekivano 1", k.totalCounter)
	}
	if k.unreadAmount != 100 {
		t.Errorf("unreadAmount = %v, očekivano 100", k.unreadAmount)
	}
}

func TestCmdSignBlokiraNaLimitu(t *testing.T) {
	k := novaTestnaKartica()
	k.pinUnesen = true
	k.unreadAmount = k.Limit
	resp := k.cmdSign("Normal", "Sale", 1)
	if resp["status"] != "blocked" {
		t.Fatalf("cmdSign na limitu mora vratiti status=blocked, dobijeno: %v", resp)
	}
}

func TestCmdSignRefundSmanjujeUnreadAmount(t *testing.T) {
	k := novaTestnaKartica()
	k.pinUnesen = true
	k.unreadAmount = 500
	k.cmdSign("Normal", "Refund", 200)
	if k.unreadAmount != 300 {
		t.Errorf("unreadAmount posle refunda = %v, očekivano 300", k.unreadAmount)
	}
}

func TestCmdSignRefundNeIdeUNegativu(t *testing.T) {
	k := novaTestnaKartica()
	k.pinUnesen = true
	k.unreadAmount = 50
	k.cmdSign("Normal", "Refund", 200)
	if k.unreadAmount != 0 {
		t.Errorf("unreadAmount ne sme biti negativan, dobijeno %v", k.unreadAmount)
	}
}

func TestCmdResetAuditRadiBezPina(t *testing.T) {
	// namerno: admin panel (BeResetAudit) nikad ne zove verify_pin, v. komentar
	// uz cmdResetAudit u kartica.go — reset ne sme zahtevati PIN.
	k := novaTestnaKartica()
	k.unreadAmount = 12345
	resp := k.cmdResetAudit()
	if resp["status"] != "ok" {
		t.Fatalf("cmdResetAudit mora uspeti i bez PIN-a, dobijeno: %v", resp)
	}
	if k.unreadAmount != 0 {
		t.Errorf("unreadAmount posle reset_audit = %v, očekivano 0", k.unreadAmount)
	}
}

func TestCmdVerifyPinKonstantnoVreme(t *testing.T) {
	k := novaTestnaKartica()
	if resp := k.cmdVerifyPin("1234"); resp["status"] != "ok" {
		t.Fatalf("očekivan uspeh sa ispravnim PIN-om, dobijeno: %v", resp)
	}
	if !k.pinUnesen {
		t.Fatal("pinUnesen mora biti true posle ispravnog PIN-a")
	}
}

func TestCmdStatusPrijavljujePinRequired(t *testing.T) {
	k := novaTestnaKartica()
	resp := k.cmdStatus()
	if resp["pin_required"] != true {
		t.Fatalf("pin_required mora biti true pre verifikacije, dobijeno: %v", resp)
	}
	k.cmdVerifyPin("1234")
	resp = k.cmdStatus()
	if resp["pin_required"] != false {
		t.Fatalf("pin_required mora biti false posle verifikacije, dobijeno: %v", resp)
	}
}
