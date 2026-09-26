package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Ошибки GitHub, которые что-то значат для владельца (требования 25–26).
var (
	ErrNotConfigured = errors.New("GitHub не настроен: нет FEEDBACK_GITHUB_TOKEN")
	ErrNoIssue       = errors.New("такой задачи нет")
)

// github — REST API задач одного репозитория (требование 35).
type github struct {
	token  string
	repo   string
	apiURL string
	http   *http.Client
}

// Issue — задача GitHub в том виде, в каком она нужна отзывам.
type Issue struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	URL         string `json:"html_url"`
	State       string `json:"state"`
	StateReason string `json:"state_reason"`
	Labels      []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest json.RawMessage `json:"pull_request"`
}

func (i Issue) hasLabel(names ...string) bool {
	for _, l := range i.Labels {
		for _, n := range names {
			if l.Name == n {
				return true
			}
		}
	}
	return false
}

// statusError — GitHub ответил не 2xx.
type statusError struct {
	status  int
	message string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("GitHub ответил %d: %s", e.status, e.message)
}

func (g *github) do(ctx context.Context, method, path string, payload, out any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.apiURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &e)
		return &statusError{status: resp.StatusCode, message: e.Message}
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func (g *github) issuesPath() string { return "/repos/" + g.repo + "/issues" }

func (g *github) create(ctx context.Context, title, body string, labels []string) (Issue, error) {
	var out Issue
	err := g.do(ctx, http.MethodPost, g.issuesPath(), map[string]any{
		"title": title, "body": body, "labels": labels,
	}, &out)
	return out, err
}

func (g *github) issue(ctx context.Context, n int) (Issue, error) {
	var out Issue
	err := g.do(ctx, http.MethodGet, fmt.Sprintf("%s/%d", g.issuesPath(), n), nil, &out)
	return out, notFound(err)
}

func (g *github) addLabel(ctx context.Context, n int, label string) error {
	err := g.do(ctx, http.MethodPost, fmt.Sprintf("%s/%d/labels", g.issuesPath(), n),
		map[string]any{"labels": []string{label}}, nil)
	return notFound(err)
}

// open — открытые задачи с меткой, новые сверху, без PR.
func (g *github) open(ctx context.Context, label string, limit int) ([]Issue, error) {
	q := url.Values{"labels": {label}, "state": {"open"}, "per_page": {fmt.Sprint(limit)}}
	var all []Issue
	if err := g.do(ctx, http.MethodGet, g.issuesPath()+"?"+q.Encode(), nil, &all); err != nil {
		return nil, err
	}
	out := make([]Issue, 0, len(all))
	for _, i := range all {
		if len(i.PullRequest) == 0 || string(i.PullRequest) == "null" {
			out = append(out, i)
		}
	}
	return out, nil
}

func (g *github) check(ctx context.Context) error {
	return g.do(ctx, http.MethodGet, "/repos/"+g.repo, nil, nil)
}

func notFound(err error) error {
	var se *statusError
	if errors.As(err, &se) && se.status == http.StatusNotFound {
		return ErrNoIssue
	}
	return err
}

// trimAPI — адрес API без косой черты в конце.
func trimAPI(u string) string { return strings.TrimRight(u, "/") }
