package paymentprovider

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"testing"
)

func TestCanonicalValuesSortsAndExcludesSignatureFields(t *testing.T) {
	values := url.Values{
		"trade_status": {"TRADE_SUCCESS"},
		"out_trade_no": {"FM-100"},
		"sign_type":    {"RSA2"},
		"sign":         {"ignored"},
		"empty":        {""},
	}

	got := canonicalValues(values, "sign", "sign_type")
	want := "out_trade_no=FM-100&trade_status=TRADE_SUCCESS"
	if got != want {
		t.Fatalf("canonicalValues() = %q, want %q", got, want)
	}
}

func TestRSA2SignAndVerifyNotification(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	privatePEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}))
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	values := url.Values{
		"app_id":       {"sandbox-app"},
		"out_trade_no": {"FM-100"},
		"trade_no":     {"202607300001"},
		"trade_status": {"TRADE_SUCCESS"},
		"total_amount": {"12.34"},
		"sign_type":    {"RSA2"},
	}

	signature, err := rsa2Sign(privatePEM, canonicalValues(values, "sign", "sign_type"))
	if err != nil {
		t.Fatalf("rsa2Sign() error = %v", err)
	}
	values.Set("sign", signature)

	if err := verifyNotification(publicPEM, values); err != nil {
		t.Fatalf("verifyNotification() error = %v", err)
	}
	values.Set("total_amount", "12.35")
	if err := verifyNotification(publicPEM, values); err == nil {
		t.Fatal("verifyNotification() accepted a modified amount")
	}
}

func TestParseKeyAcceptsRawBase64(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := parsePrivateKey(base64.StdEncoding.EncodeToString(privateDER))
	if err != nil {
		t.Fatalf("parsePrivateKey() error = %v", err)
	}
	if parsed.N.Cmp(privateKey.N) != 0 {
		t.Fatal("parsed private key does not match")
	}
}
