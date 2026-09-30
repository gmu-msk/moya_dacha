package push

import (
	"bytes"
	"context"
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
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrNotConfigured — ключ Firebase не задан.
var ErrNotConfigured = errors.New("ключ Firebase не задан")

// scope — право слать сообщения FCM.
const scope = "https://www.googleapis.com/auth/firebase.messaging"

// defaultTokenURI — куда менять JWT на токен, если в ключе не сказано.
const defaultTokenURI = "https://oauth2.googleapis.com/token"

// serviceAccount — нужное из ключа сервисного аккаунта.
type serviceAccount struct {
	ProjectID    string `json:"project_id"`
	PrivateKeyID string `json:"private_key_id"`
	PrivateKey   string `json:"private_key"`
	ClientEmail  string `json:"client_email"`
	TokenURI     string `json:"token_uri"`

	signer *rsa.PrivateKey
}

func parseServiceAccount(raw []byte) (*serviceAccount, error) {
	var sa serviceAccount
	if err := json.Unmarshal(raw, &sa); err != nil {
		return nil, fmt.Errorf("не JSON: %w", err)
	}
	if sa.ProjectID == "" || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, errors.New("нет project_id, client_email или private_key")
	}
	if sa.TokenURI == "" {
		sa.TokenURI = defaultTokenURI
	}
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, errors.New("private_key не PEM")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private_key не RSA")
		}
		sa.signer = rk
	} else if rk, err2 := x509.ParsePKCS1PrivateKey(block.Bytes); err2 == nil {
		sa.signer = rk
	} else {
		return nil, fmt.Errorf("private_key не разобран: %w", err)
	}
	return &sa, nil
}

// network — дорога до Google: напрямую, а если соединение не открылось
// и туннель задан, — туннелем, и дальше только им (требование 15).
type network struct {
	direct   *http.Client
	proxied  *http.Client // nil — туннеля нет
	viaProxy atomic.Bool
}

func newNetwork(proxy string) *network {
	n := &network{direct: &http.Client{Timeout: 30 * time.Second}}
	if proxy == "" {
		return n
	}
	u, err := url.Parse(proxy)
	if err != nil {
		slog.Error("пуши: PUSH_PROXY не разобран, иду только напрямую", "err", err)
		return n
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(u)
	n.proxied = &http.Client{Timeout: 30 * time.Second, Transport: transport}
	return n
}

// do выполняет запрос, который собирает build: тело запроса читается
// один раз, а при переходе на туннель запрос уходит второй раз.
func (n *network) do(ctx context.Context, build func(context.Context) (*http.Request, error)) (*http.Response, error) {
	if n.viaProxy.Load() {
		return n.send(ctx, n.proxied, build)
	}
	resp, err := n.send(ctx, n.direct, build)
	if err == nil || n.proxied == nil || !unreachable(err) || ctx.Err() != nil {
		return resp, err
	}
	slog.Warn("пуши: Google напрямую не открывается, дальше через туннель", "err", err)
	n.viaProxy.Store(true)
	return n.send(ctx, n.proxied, build)
}

func (n *network) send(ctx context.Context, client *http.Client, build func(context.Context) (*http.Request, error)) (*http.Response, error) {
	req, err := build(ctx)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// unreachable — соединение не открылось вовсе: адрес не резолвится, порт
// закрыт или соединение не установилось за таймаут.
func unreachable(err error) bool {
	var op *net.OpError
	if errors.As(err, &op) && op.Op == "dial" {
		return true
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return true
	}
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout()
}

// tokenSource — OAuth-токен сервисного аккаунта, живёт до истечения.
type tokenSource struct {
	key *serviceAccount
	net *network

	mu      sync.Mutex
	access  string
	expires time.Time
}

func (ts *tokenSource) token(ctx context.Context) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.access != "" && time.Now().Before(ts.expires) {
		return ts.access, nil
	}
	assertion, err := ts.assertion(time.Now())
	if err != nil {
		return "", err
	}
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}.Encode()
	resp, err := ts.net.do(ctx, func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.key.TokenURI, strings.NewReader(form))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		return req, err
	})
	if err != nil {
		return "", &temporaryError{err}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("OAuth-токен: %s: %s", resp.Status, strings.TrimSpace(string(body)))
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return "", &temporaryError{err}
		}
		return "", err
	}
	var got struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &got); err != nil || got.AccessToken == "" {
		return "", fmt.Errorf("OAuth-токен: ответ не разобран: %s", strings.TrimSpace(string(body)))
	}
	if got.ExpiresIn <= 0 {
		got.ExpiresIn = 3600
	}
	ts.access = got.AccessToken
	// Минута запаса: токен не должен истечь по дороге.
	ts.expires = time.Now().Add(time.Duration(got.ExpiresIn)*time.Second - time.Minute)
	return ts.access, nil
}

// forget — токен отвергнут: следующий запрос возьмёт новый.
func (ts *tokenSource) forget() {
	ts.mu.Lock()
	ts.access = ""
	ts.mu.Unlock()
}

// assertion — JWT, подписанный ключом аккаунта (RS256).
func (ts *tokenSource) assertion(now time.Time) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": ts.key.PrivateKeyID})
	claims, _ := json.Marshal(map[string]any{
		"iss":   ts.key.ClientEmail,
		"scope": scope,
		"aud":   ts.key.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	enc := base64.RawURLEncoding
	unsigned := enc.EncodeToString(header) + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, ts.key.signer, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + enc.EncodeToString(sig), nil
}

// Сообщение FCM HTTP v1.
type fcmMessage struct {
	Token        string            `json:"token"`
	Notification *fcmNotification  `json:"notification,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
	Android      *fcmAndroid       `json:"android,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmAndroid struct {
	Priority     string                  `json:"priority,omitempty"`
	Notification *fcmAndroidNotification `json:"notification,omitempty"`
}

type fcmAndroidNotification struct {
	Tag string `json:"tag,omitempty"`
}

// fcmError — отказ FCM.
type fcmError struct {
	status int
	code   string // errorCode из details, например UNREGISTERED
	state  string // status из error, например INVALID_ARGUMENT
	text   string
}

func (e *fcmError) Error() string {
	return fmt.Sprintf("FCM %d %s %s: %s", e.status, e.state, e.code, e.text)
}

type temporaryError struct{ err error }

func (e *temporaryError) Error() string { return e.err.Error() }
func (e *temporaryError) Unwrap() error { return e.err }

// isGone — токена телефона у FCM больше нет (требование 13).
func isGone(err error) bool {
	var fe *fcmError
	if !errors.As(err, &fe) {
		return false
	}
	return fe.code == "UNREGISTERED" || fe.status == http.StatusNotFound ||
		(fe.status == http.StatusBadRequest && fe.state == "INVALID_ARGUMENT")
}

// isTemporary — повтор может помочь: сеть, 429, 5xx, отвергнутый
// OAuth-токен.
func isTemporary(err error) bool {
	var te *temporaryError
	if errors.As(err, &te) {
		return true
	}
	var fe *fcmError
	if errors.As(err, &fe) {
		return fe.status == http.StatusTooManyRequests || fe.status >= 500 ||
			fe.status == http.StatusUnauthorized
	}
	return false
}

// send отправляет одно сообщение.
func (s *Sender) send(ctx context.Context, m fcmMessage) error {
	access, err := s.auth.token(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]fcmMessage{"message": m})
	if err != nil {
		return err
	}
	endpoint := s.cfg.APIURL + "/v1/projects/" + url.PathEscape(s.key.ProjectID) + "/messages:send"
	resp, err := s.net.do(ctx, func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+access)
		}
		return req, err
	})
	if err != nil {
		return &temporaryError{err}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	fe := &fcmError{status: resp.StatusCode, text: strings.TrimSpace(string(raw))}
	var parsed struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &parsed) == nil {
		fe.state = parsed.Error.Status
		if parsed.Error.Message != "" {
			fe.text = parsed.Error.Message
		}
		for _, d := range parsed.Error.Details {
			if d.ErrorCode != "" {
				fe.code = d.ErrorCode
			}
		}
	}
	if resp.StatusCode == http.StatusUnauthorized {
		s.auth.forget()
	}
	return fe
}
