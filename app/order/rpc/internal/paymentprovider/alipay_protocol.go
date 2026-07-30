package paymentprovider

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

func canonicalValues(values url.Values, excluded ...string) string {
	skip := make(map[string]struct{}, len(excluded))
	for _, key := range excluded {
		skip[key] = struct{}{}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if _, found := skip[key]; found || strings.TrimSpace(values.Get(key)) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	return strings.Join(parts, "&")
}

func rsa2Sign(privateKeyText, content string) (string, error) {
	privateKey, err := parsePrivateKey(privateKeyText)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(content))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign alipay request: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func verifyNotification(publicKeyText string, values url.Values) error {
	return verifyRSA2(publicKeyText, canonicalValues(values, "sign", "sign_type"), values.Get("sign"))
}

func verifyRSA2(publicKeyText, content, encodedSignature string) error {
	publicKey, err := parsePublicKey(publicKeyText)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedSignature))
	if err != nil {
		return fmt.Errorf("decode alipay signature: %w", err)
	}
	digest := sha256.Sum256([]byte(content))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return fmt.Errorf("verify alipay signature: %w", err)
	}
	return nil
}

func decodeSignedResponse(publicKey string, body []byte, responseKey string, target any) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode alipay response: %w", err)
	}
	raw, found := envelope[responseKey]
	if !found {
		return fmt.Errorf("alipay response missing %s", responseKey)
	}
	var signature string
	if err := json.Unmarshal(envelope["sign"], &signature); err != nil {
		return errors.New("alipay response signature is missing")
	}
	if err := verifyRSA2(publicKey, string(raw), signature); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode %s: %w", responseKey, err)
	}
	return nil
}

func parsePrivateKey(text string) (*rsa.PrivateKey, error) {
	der, err := decodeKey(text)
	if err != nil {
		return nil, err
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	return nil, errors.New("alipay private key must be RSA PKCS8 or PKCS1")
}

func parsePublicKey(text string) (*rsa.PublicKey, error) {
	der, err := decodeKey(text)
	if err != nil {
		return nil, err
	}
	if key, err := x509.ParsePKIXPublicKey(der); err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
	}
	if key, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return key, nil
	}
	return nil, errors.New("alipay public key must be RSA PKIX or PKCS1")
}

func decodeKey(text string) ([]byte, error) {
	text = strings.TrimSpace(strings.ReplaceAll(text, `\n`, "\n"))
	if block, _ := pem.Decode([]byte(text)); block != nil {
		return block.Bytes, nil
	}
	der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(text), ""))
	if err != nil {
		return nil, fmt.Errorf("decode alipay key: %w", err)
	}
	return der, nil
}
