package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

const loginURL = "https://www.cosmos.so/api/login"

var sessionHeaders = map[string]string{
	"Content-Type":  "application/json",
	"Origin":        "https://www.cosmos.so",
	"Referer":       "https://www.cosmos.so/",
	"x-client-name": "cosmos-web",
	"User-Agent":    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Accept":        "application/graphql-response+json,application/json;q=0.9",
}

type Data struct {
	Token   string            `json:"token,omitempty"`
	Cookies map[string]string `json:"cookies,omitempty"`
}

func authFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cosmos", "auth.json")
}

func Save(data Data) error {
	path := authFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func Load() (*Data, error) {
	b, err := os.ReadFile(authFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var data Data
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, nil
	}
	return &data, nil
}

func Clear() error {
	err := os.Remove(authFile())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func Login(email, password string) (Data, error) {
	body, _ := json.Marshal(map[string]string{
		"identifier": email,
		"password":   password,
	})

	req, _ := http.NewRequest("POST", loginURL, bytes.NewReader(body))
	for k, v := range sessionHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Data{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		Token   string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Data{}, fmt.Errorf("decoding response: %w", err)
	}
	if !result.Success {
		msg := result.Error
		if msg == "" {
			msg = "login failed"
		}
		return Data{}, fmt.Errorf("%s", msg)
	}

	data := Data{}
	if result.Token != "" {
		data.Token = result.Token
	}

	cookies := map[string]string{}
	for _, c := range resp.Cookies() {
		cookies[c.Name] = c.Value
	}
	if len(cookies) > 0 {
		data.Cookies = cookies
	}

	if data.Token == "" && len(data.Cookies) == 0 {
		return Data{}, fmt.Errorf("login succeeded but no credentials were returned")
	}

	return data, nil
}

func NewClient(token string) *http.Client {
	return &http.Client{
		Transport: &authTransport{token: token},
	}
}

type authTransport struct {
	token string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range sessionHeaders {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}

	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	} else if data, _ := Load(); data != nil {
		if data.Token != "" {
			req.Header.Set("Authorization", "Bearer "+data.Token)
		}
		if len(data.Cookies) > 0 {
			for name, value := range data.Cookies {
				req.AddCookie(&http.Cookie{Name: name, Value: value})
			}
		}
	}

	return http.DefaultTransport.RoundTrip(req)
}
