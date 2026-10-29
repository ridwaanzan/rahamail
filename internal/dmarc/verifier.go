package dmarc

import (
	"bytes"
	"fmt"
	"net"
	"strings"

	"github.com/emersion/go-msgauth/dkim"
)

// AuthResults merepresentasikan status hasil verifikasi SPF, DKIM, dan DMARC pada email masuk
type AuthResults struct {
	SPFStatus    string // pass, fail, softfail, neutral, none
	DKIMStatus   string // pass, fail, neutral, none
	DMARCStatus  string // pass, fail, none
	DMARCPolicy  string // none, quarantine, reject
	HeaderString string
}

// VerifyInboundAuth melakukan verifikasi SPF, DKIM, dan mengevaluasi kebijakan DMARC pengirim
func VerifyInboundAuth(senderIP string, fromEmail string, rawMessage []byte) AuthResults {
	results := AuthResults{
		SPFStatus:   "neutral",
		DKIMStatus:  "none",
		DMARCStatus: "none",
		DMARCPolicy: "none",
	}

	fromParts := strings.Split(fromEmail, "@")
	if len(fromParts) != 2 {
		return results
	}
	fromDomain := strings.ToLower(fromParts[1])

	// 1. Verifikasi SPF (DNS TXT record lookup)
	spfStatus := checkSPF(senderIP, fromDomain)
	results.SPFStatus = spfStatus

	// 2. Verifikasi DKIM (go-msgauth)
	dkimStatus := checkDKIM(rawMessage)
	results.DKIMStatus = dkimStatus

	// 3. Evaluasi DMARC
	dmarcStatus, policy := checkDMARC(fromDomain, spfStatus, dkimStatus)
	results.DMARCStatus = dmarcStatus
	results.DMARCPolicy = policy

	// Format header Authentication-Results sesuai RFC 8601
	results.HeaderString = fmt.Sprintf(
		"Authentication-Results: rahamail; spf=%s (sender IP %s); dkim=%s; dmarc=%s (policy=%s)\r\n",
		results.SPFStatus, senderIP, results.DKIMStatus, results.DMARCStatus, results.DMARCPolicy,
	)

	return results
}

func checkSPF(senderIP, domain string) string {
	if senderIP == "" || senderIP == "127.0.0.1" || strings.HasPrefix(senderIP, "192.168.") || strings.HasPrefix(senderIP, "10.") {
		return "pass"
	}

	txtRecords, err := net.LookupTXT(domain)
	if err != nil || len(txtRecords) == 0 {
		return "none"
	}

	hasSPF := false
	for _, txt := range txtRecords {
		if strings.HasPrefix(txt, "v=spf1") {
			hasSPF = true
			if strings.Contains(txt, "+all") || strings.Contains(txt, "ip4:"+senderIP) {
				return "pass"
			}
			if strings.Contains(txt, "-all") {
				return "fail"
			}
			if strings.Contains(txt, "~all") {
				return "softfail"
			}
			if strings.Contains(txt, "?all") {
				return "neutral"
			}
		}
	}

	if !hasSPF {
		return "none"
	}
	return "neutral"
}

func checkDKIM(rawMessage []byte) string {
	verifications, err := dkim.Verify(bytes.NewReader(rawMessage))
	if err != nil || len(verifications) == 0 {
		return "none"
	}

	for _, v := range verifications {
		if v.Err == nil {
			return "pass"
		}
	}
	return "fail"
}

func checkDMARC(domain, spfStatus, dkimStatus string) (string, string) {
	dmarcDomain := "_dmarc." + domain
	txtRecords, err := net.LookupTXT(dmarcDomain)
	if err != nil || len(txtRecords) == 0 {
		return "none", "none"
	}

	policy := "none"
	found := false
	for _, txt := range txtRecords {
		if strings.HasPrefix(txt, "v=DMARC1") {
			found = true
			if strings.Contains(txt, "p=reject") {
				policy = "reject"
			} else if strings.Contains(txt, "p=quarantine") {
				policy = "quarantine"
			} else {
				policy = "none"
			}
			break
		}
	}

	if !found {
		return "none", "none"
	}

	// Sesuai RFC 7489: DMARC lulus jika SPF pass ATAU DKIM pass
	if spfStatus == "pass" || dkimStatus == "pass" {
		return "pass", policy
	}

	return "fail", policy
}
