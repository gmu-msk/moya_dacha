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
	"strconv"
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

	places, points, err := s.suggestPlaces(ctx, query)
	if err != nil {
		slog.Warn("подсказки DaData не пришли", "err", err)
		return gen.GetPlaces503JSONResponse(errPlacesUnavailable), nil
	}

	if err := s.rememberPlaces(ctx, places, points, true); err != nil {
		return nil, err
	}
	return gen.GetPlaces200JSONResponse(gen.PlaceList{Places: places}), nil
}

// geoPoint — координаты пункта из подсказки (specs/027-post-place.md,
// требования 10–11).
type geoPoint struct{ lat, lon float64 }

// rememberPlaces запоминает отданные пункты (требование 5): новый
// добавляется, у известного обновляются название и уточнение. Координаты
// из points: центр пункта (center) заменяет прежние, точка дома в пункте
// пишется, только если координат ещё нет (027, требование 11).
func (s *Server) rememberPlaces(ctx context.Context, places []gen.Place, points map[string]geoPoint, center bool) error {
	for _, p := range places {
		var lat, lon *float64
		if point, ok := points[p.Id]; ok {
			lat, lon = &point.lat, &point.lon
		}
		if _, err := s.db.Exec(ctx, `
			INSERT INTO places (id, name, area, lat, lon, updated_at) VALUES ($1, $2, $3, $4, $5, now())
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, area = EXCLUDED.area, updated_at = now(),
				lat = CASE WHEN `+keepPoint(center)+` THEN places.lat ELSE EXCLUDED.lat END,
				lon = CASE WHEN `+keepPoint(center)+` THEN places.lon ELSE EXCLUDED.lon END`,
			p.Id, p.Name, p.Area, lat, lon); err != nil {
			return err
		}
	}
	return nil
}

// keepPoint — когда у известного пункта остаются прежние координаты:
// новых нет, а для точки дома — и когда прежние уже есть.
func keepPoint(center bool) string {
	if center {
		return `EXCLUDED.lat IS NULL`
	}
	return `(EXCLUDED.lat IS NULL OR places.lat IS NOT NULL)`
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

// dadataSuggestion — подсказка DaData. Подсказкам по названию нужны
// value, fias_id и fias_level (specs/025-places.md, требование 7),
// пунктам рядом — поля пункта (specs/026-places-nearby.md, 7–8), обоим —
// координаты (specs/027-post-place.md, 11). fias_level и координаты
// приходят строкой, но могут и числом.
type dadataSuggestion struct {
	Value string `json:"value"`
	Data  struct {
		FiasID             string          `json:"fias_id"`
		FiasLevel          json.RawMessage `json:"fias_level"`
		SettlementFiasID   string          `json:"settlement_fias_id"`
		SettlementWithType string          `json:"settlement_with_type"`
		CityFiasID         string          `json:"city_fias_id"`
		CityWithType       string          `json:"city_with_type"`
		AreaWithType       string          `json:"area_with_type"`
		RegionWithType     string          `json:"region_with_type"`
		GeoLat             json.RawMessage `json:"geo_lat"`
		GeoLon             json.RawMessage `json:"geo_lon"`
	} `json:"data"`
}

// suggestPlaces спрашивает DaData и разбирает ответ по требованиям 6–8.
// Вместе с пунктами отдаёт их координаты, где они есть.
func (s *Server) suggestPlaces(ctx context.Context, query string) ([]gen.Place, map[string]geoPoint, error) {
	suggestions, err := s.askDadata(ctx, "/suggest/address",
		map[string]any{"query": query, "count": placesAsked})
	if err != nil {
		return nil, nil, err
	}

	points := map[string]geoPoint{}
	places := []gen.Place{}
	seen := map[string]bool{}
	for _, sg := range suggestions {
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
		if point, ok := sg.point(); ok {
			points[id] = point
		}
		if len(places) == maxPlaces {
			break
		}
	}
	return places, points, nil
}

// point — координаты подсказки, если пришли обе и в пределах
// (specs/027-post-place.md, требование 11).
func (sg dadataSuggestion) point() (geoPoint, bool) {
	lat, okLat := coordinate(sg.Data.GeoLat)
	lon, okLon := coordinate(sg.Data.GeoLon)
	if !okLat || !okLon || !inRange(lat, 90) || !inRange(lon, 180) {
		return geoPoint{}, false
	}
	return geoPoint{lat: lat, lon: lon}, true
}

// coordinate читает координату и строкой, и числом; null и пустое — нет.
func coordinate(raw json.RawMessage) (float64, bool) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		v, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		return v, err == nil
	}
	var number float64
	if json.Unmarshal(raw, &number) == nil && string(raw) != "null" {
		return number, true
	}
	return 0, false
}

// askDadata отправляет запрос в DaData и отдаёт подсказки из ответа. Нет
// поля suggestions — подсказок нет. Тело запроса в лог не пишется: в нём
// бывают координаты человека (specs/026-places-nearby.md, требование 5).
func (s *Server) askDadata(ctx context.Context, path string, payload any) ([]dadataSuggestion, error) {
	base := s.cfg.PlacesURL
	if base == "" {
		base = DefaultPlacesURL
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, placesTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(base, "/")+path, bytes.NewReader(body))
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
	return parsed.Suggestions, nil
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
