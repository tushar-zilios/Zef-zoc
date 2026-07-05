package gcs

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type ServiceAccountKey struct {
	Type         string `json:"type"`
	ProjectID    string `json:"project_id"`
	PrivateKeyID string `json:"private_key_id"`
	PrivateKey   string `json:"private_key"`
	ClientEmail  string `json:"client_email"`
	TokenURI     string `json:"token_uri"`
}

type Client struct {
	key    *ServiceAccountKey
	bucket string
}

func NewClient(saJSON, bucket string) (*Client, error) {
	if saJSON == "" || bucket == "" {
		return nil, fmt.Errorf("GCS not configured (missing service account JSON or bucket)")
	}
	var key ServiceAccountKey
	if err := json.Unmarshal([]byte(saJSON), &key); err != nil {
		return nil, fmt.Errorf("parse service account JSON: %w", err)
	}
	return &Client{key: &key, bucket: bucket}, nil
}

func (c *Client) accessToken() (string, error) {
	now := time.Now().Unix()
	scope := "https://www.googleapis.com/auth/devstorage.read_write"

	claimsJSON, _ := json.Marshal(map[string]any{
		"iss":   c.key.ClientEmail,
		"scope": scope,
		"aud":   c.key.TokenURI,
		"exp":   now + 3600,
		"iat":   now,
	})

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := header + "." + payload

	block, _ := pem.Decode([]byte(c.key.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}
	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	rsaKey, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("expected RSA private key")
	}

	h := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, h[:])
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}

	jwtStr := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)

	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {jwtStr},
	}
	resp, err := http.PostForm(c.key.TokenURI, form)
	if err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tok.Error != "" {
		return "", fmt.Errorf("GCP token error: %s", tok.Error)
	}
	return tok.AccessToken, nil
}

// UploadObject creates/overwrites an object in the bucket with the given content.
// Returns the storage key (object name) on success.
func (c *Client) UploadObject(objectName string, content []byte, contentType string) (string, error) {
	token, err := c.accessToken()
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("https://storage.googleapis.com/upload/storage/v1/b/%s/o?uploadType=media&name=%s",
		c.bucket, url.QueryEscape(objectName))

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("GCS upload failed (%d): %s", resp.StatusCode, string(body))
	}

	return fmt.Sprintf("gs://%s/%s", c.bucket, objectName), nil
}

// PublicURL returns the browser-loadable HTTPS URL for an object, assuming
// the bucket grants public read access (standard for served editor images).
func (c *Client) PublicURL(objectName string) string {
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", c.bucket, url.PathEscape(objectName))
}
