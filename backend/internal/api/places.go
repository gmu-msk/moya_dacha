// Населённый пункт в профиле: specs/025-places.md.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// DefaultPlacesURL — адрес подсказок DaData по умолчанию (требование 6).
const DefaultPlacesURL = "https://suggestions.dadata.ru/suggestions/api/4_1/rs"

const (
	minPlaceQuery = 2
	maxPlaceQuery = 100
	// maxPlaces — сколько подсказок отдаётся приложению (требование 1),
	// placesAsked — сколько просим у справочника: часть отсеется (7).
	maxPlaces     = 10
	placesAsked   = 20
	placesTimeout = 5 * time.Second
)

// placeLevels — уровни ФИАС, которые годятся в пункт (требование 7):
// город, населённый пункт, планировочная структура (СНТ в ГАР) и
// дополнительная территория (СНТ в старом ФИАС).
var placeLevels = map[string]bool{"4": true, "6": true, "65": true, "90": true}

// GetPlaces отдаёт подсказки населённых пунктов и запоминает их, чтобы
// потом один из них можно было выбрать в профиле.
func (s *Server) GetPlaces(ctx context.Context, request gen.GetPlacesRequestObject) (gen.GetPlacesResponseObject, error) {
	if _, ok := sessionFrom(ctx); !ok {
		return gen.GetPlaces401JSONResponse(errUnauthorized), nil
	}
	query := strings.TrimSpace(request.Params.Q)
	if n := utf8.RuneCountInString(query); n < minPlaceQuery || n > maxPlaceQuery {
		return gen.GetPlaces400JSONResponse(errInvalidQuery), nil
	}
	if s.cfg.PlacesKey == "" {
		return gen.GetPlaces503JSONResponse(errPlacesUnavailable), nil
	}

	places, err := s.suggestPlaces(ctx, query)
	if err != nil {
		slog.Warn("подсказки DaData не пришли", "err", err)
		return gen.GetPlaces503JSONResponse(errPlacesUnavailable), nil
	}

	for _, p := range places {
		if _, err := s.db.Exec(ctx, `
			INSERT INTO places (id, name, area, updated_at) VALUES ($1, $2, $3, now())
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, area = EXCLUDED.area, updated_at = now()`,
			p.Id, p.Name, p.Area); err != nil {
			return nil, err
		}
	}
	return gen.GetPlaces200JSONResponse(gen.PlaceList{Places: places}), nil
}

// SetPlace ставит или убирает пункт в профиле. Справочник не спрашивается:
// годится только пункт, который сервер уже отдавал (требование 9).
func (s *Server) SetPlace(ctx context.Context, request gen.SetPlaceRequestObject) (gen.SetPlaceResponseObject, error) {
	current, ok := sessionFrom(ctx)
	if !ok {
		return gen.SetPlace401JSONResponse(errUnauthorized), nil
	}
	if request.Body == nil {
		return gen.SetPlace400JSONResponse(errEmptyRequest), nil
	}

	var placeID *string
	if id := request.Body.PlaceId; id != nil && strings.TrimSpace(*id) != "" {
		trimmed := strings.TrimSpace(*id)
		placeID = &trimmed
	}

	user, err := s.scanUser(s.db.QueryRow(ctx, `
		UPDATE users SET place_id = $2
		WHERE id = $1 AND ($2::text IS NULL OR EXISTS (SELECT 1 FROM places WHERE id = $2))
		RETURNING `+userColumns, current.user.Id, placeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.SetPlace400JSONResponse(errUnknownPlace), nil
	}
	if err != nil {
		return nil, err
	}
	return gen.SetPlace200JSONResponse(user), nil
}

// placeScan — пункт, прочитанный по placeColumns; у человека без пункта
// все три поля пусты.
type placeScan struct{ id, name, area *string }

func (p placeScan) value() *gen.Place {
	if p.id == nil || p.name == nil {
		return nil
	}
	place := gen.Place{Id: *p.id, Name: *p.name}
	if p.area != nil {
		place.Area = *p.area
	}
	return &place
}

// dadataSuggestion — подсказка DaData; из неё нужны только три поля
// (требование 7). fias_level приходит строкой, но может и числом.
type dadataSuggestion struct {
	Value string `json:"value"`
	Data  struct {
		FiasID    string          `json:"fias_id"`
		FiasLevel json.RawMessage `json:"fias_level"`
	} `json:"data"`
}

// suggestPlaces спрашивает DaData и разбирает ответ по требованиям 6–8.
func (s *Server) suggestPlaces(ctx context.Context, query string) ([]gen.Place, error) {
	base := s.cfg.PlacesURL
	if base == "" {
		base = DefaultPlacesURL
	}
	body, err := json.Marshal(map[string]any{"query": query, "count": placesAsked})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, placesTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(base, "/")+"/suggest/address", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+s.cfg.PlacesKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DaData ответила %d", resp.StatusCode)
	}

	var parsed struct {
		Suggestions []dadataSuggestion `json:"suggestions"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("ответ DaData не разобран: %w", err)
	}

	places := []gen.Place{}
	seen := map[string]bool{}
	for _, sg := range parsed.Suggestions {
		id := strings.TrimSpace(sg.Data.FiasID)
		if id == "" || seen[id] || !placeLevels[fiasLevel(sg.Data.FiasLevel)] {
			continue
		}
		name, area, ok := splitPlaceValue(sg.Value)
		if !ok {
			continue
		}
		seen[id] = true
		places = append(places, gen.Place{Id: id, Name: name, Area: area})
		if len(places) == maxPlaces {
			break
		}
	}
	return places, nil
}

// fiasLevel читает уровень ФИАС и строкой, и числом.
func fiasLevel(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return ""
}

// splitPlaceValue делит адрес подсказки на название пункта и то, что выше
// него, от ближнего к дальнему (требование 8).
func splitPlaceValue(value string) (name, area string, ok bool) {
	var parts []string
	for _, part := range strings.Split(value, ", ") {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return "", "", false
	}
	above := make([]string, 0, len(parts)-1)
	for i := len(parts) - 2; i >= 0; i-- {
		above = append(above, parts[i])
	}
	return parts[len(parts)-1], strings.Join(above, ", "), true
}

var (
	errInvalidQuery = gen.Error{
		Code:    "invalid_query",
		Message: "Наберите от 2 до 100 символов",
	}
	errPlacesUnavailable = gen.Error{
		Code:    "places_unavailable",
		Message: "Подсказки сейчас недоступны",
	}
	errUnknownPlace = gen.Error{
		Code:    "unknown_place",
		Message: "Выберите пункт из подсказок",
	}
)
