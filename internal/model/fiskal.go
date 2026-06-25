package model

// FiskalniRacun je veza između prodajnog naloga i odgovora fiskalne kase (Teron L-PFR).
// Polja prate tačno ono što Fisk server vraća u JSON odgovoru (vidi NtechFisk.md §3.2).
type FiskalniRacun struct {
	ID                int64
	ProdajaID         int64   // 0 ako nije vezan za prodaju (npr. servisni nalog)
	ServisID          int64   // 0 ako nije vezan za servis (npr. prodajni nalog)
	TipRacuna         string  // "Normal", "Advance", "Copy", "Training"
	TipTransakcije    string  // "Sale", "Refund"
	PfrBroj           string  // invoiceNumber — jedinstven broj fiskalnog računa
	PfrVreme          string  // sdcDateTime — PFR vreme izdavanja
	Brojac            string  // invoiceCounter (npr. "1/1ПП")
	EkstenzijaBrojaca string  // invoiceCounterExtension (npr. "ПП")
	UrlVerifikacija   string  // verificationUrl — link ka PURS proveri
	QRKod             string  // verificationQRCode — base64 PNG, direktno u <img src>
	PoreskeStavke     string  // JSON niz taxItems[] iz odgovora
	UkupnoZaNaplatu   float64 // totalAmount
	UkupanPorez       float64 // totalTax
	SiroviOdgovor     string  // ceo JSON odgovor — sadrži journal, taxItems i ostalo za audit
	Potpisao          string  // signedBy — identifikator PFR-a koji je potpisao
	Zatrazio          string  // requestedBy — identifikator ESIR-a
	Poruka            string  // messages (npr. "Success")
	Storniran         bool    // true ako je račun naknadno storniran
	VremeKreiranja    string
}
