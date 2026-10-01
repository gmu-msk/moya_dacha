// Населённый пункт по геолокации: specs/026-places-nearby.md.
package api

import (
	"context"
	"log/slog"
	"math"
	"strings"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// nearbyRadius — в каком радиусе от точки DaData ищет адреса, в метрах
// (требование 6).
const nearbyRadius = 1000

// GetPlacesNearby отдаёт пункты рядом с точкой и запоминает их, как
// подсказки по названию. Координаты никуда не пишутся (требование 5).
func (s *Server) GetPlacesNearby(ctx context.Context, request gen.GetPlacesNearbyRequestObject) (gen.GetPlacesNearbyResponseObject, error) {
	if _, ok := sessionFrom(ctx); !ok {
		return gen.GetPlacesNearby401JSONResponse(errUnauthorized), nil
	}
	lat, lon := request.Params.Lat, request.Params.Lon
	if !inRange(lat, 90) || !inRange(lon, 180) {
		return gen.GetPlacesNearby400JSONResponse(errInvalidLocation), nil
	}
	if s.cfg.PlacesKey == "" {
		return gen.GetPlacesNearby503JSONResponse(errPlacesUnavailable), nil
	}

	suggestions, err := s.askDadata(ctx, "/geolocate/address", map[string]any{
		"lat": lat, "lon": lon, "radius_meters": nearbyRadius, "count": placesAsked,
	})
	if err != nil {
		slog.Warn("пункты рядом от DaData не пришли", "err", err)
		return gen.GetPlacesNearby503JSONResponse(errPlacesUnavailable), nil
	}

	places := []gen.Place{}
	seen := map[string]bool{}
	for _, sg := range suggestions {
		place, ok := nearbyPlace(sg)
		if !ok || seen[place.Id] {
			continue
		}
		seen[place.Id] = true
		places = append(places, place)
		if len(places) == maxPlaces {
			break
		}
	}

	if err := s.rememberPlaces(ctx, places); err != nil {
		return nil, err
	}
	return gen.GetPlacesNearby200JSONResponse(gen.PlaceList{Places: places}), nil
}

// inRange — число конечно и по модулю не больше limit.
func inRange(v float64, limit float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -limit && v <= limit
}

// nearbyPlace достаёт пункт из подсказки-дома (требования 7–8):
// населённый пункт (туда же DaData кладёт СНТ), а без него — город.
func nearbyPlace(sg dadataSuggestion) (gen.Place, bool) {
	d := sg.Data
	var id, name string
	var above []string
	switch {
	case strings.TrimSpace(d.SettlementFiasID) != "":
		id, name = d.SettlementFiasID, d.SettlementWithType
		above = []string{d.CityWithType, d.AreaWithType, d.RegionWithType}
	case strings.TrimSpace(d.CityFiasID) != "":
		id, name = d.CityFiasID, d.CityWithType
		above = []string{d.AreaWithType, d.RegionWithType}
	default:
		return gen.Place{}, false
	}
	id, name = strings.TrimSpace(id), strings.TrimSpace(name)
	if name == "" {
		return gen.Place{}, false
	}

	var area []string
	taken := map[string]bool{name: true}
	for _, part := range above {
		part = strings.TrimSpace(part)
		if part == "" || taken[part] {
			continue
		}
		taken[part] = true
		area = append(area, part)
	}
	return gen.Place{Id: id, Name: name, Area: strings.Join(area, ", ")}, true
}

var errInvalidLocation = gen.Error{
	Code:    "invalid_location",
	Message: "Не удалось понять, где вы",
}
