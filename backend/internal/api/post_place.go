// Геометка поста и расстояние до него: specs/027-post-place.md.
package api

import (
	"math"

	"github.com/gmu-msk/moya_dacha/backend/api/gen"
)

// samePlace — расстояние из postPlaceColumns, когда пункт смотрящего и
// место поста совпадают: тогда это 0, даже без координат (требование 8).
// Отрицательное, чтобы не спутать с разными пунктами в одной точке.
const samePlace = -1

// postPlaceJoin подключает место поста p как pp.
const postPlaceJoin = `LEFT JOIN places pp ON pp.id = p.place_id`

// postPlaceColumns — место поста и расстояние до него от пункта
// смотрящего, в порядке postPlaceScan. Расстояния нет, если у поста нет
// места, у смотрящего нет пункта, пост его собственный или у одного из
// пунктов нет координат (требование 7). Расстояние — по дуге большого
// круга, радиус Земли 6371 км (требование 9).
func postPlaceColumns(viewer string) string {
	return `pp.id, pp.name, pp.area, (
		SELECT CASE
			WHEN vp.id = pp.id THEN -1
			WHEN vp.lat IS NULL OR vp.lon IS NULL OR pp.lat IS NULL OR pp.lon IS NULL THEN NULL
			ELSE 2 * 6371 * asin(least(1, sqrt(
				power(sin(radians(pp.lat - vp.lat) / 2), 2)
				+ cos(radians(vp.lat)) * cos(radians(pp.lat))
				  * power(sin(radians(pp.lon - vp.lon) / 2), 2))))
		END
		FROM users vu JOIN places vp ON vp.id = vu.place_id
		WHERE vu.id = ` + viewer + `::uuid AND pp.id IS NOT NULL AND vu.id <> p.author_id)`
}

// postPlaceScan — то, что прочитано по postPlaceColumns.
type postPlaceScan struct {
	place    placeScan
	distance *float64
}

func (s *postPlaceScan) targets() []any {
	return []any{&s.place.id, &s.place.name, &s.place.area, &s.distance}
}

// apply кладёт место и расстояние в пост.
func (s postPlaceScan) apply(post *gen.Post) {
	post.Place = s.place.value()
	if post.Place == nil || s.distance == nil {
		return
	}
	km := int32(0)
	if *s.distance != samePlace {
		km = roundDistance(*s.distance)
	}
	post.DistanceKm = &km
}

// roundDistance округляет расстояние так, чтобы по нему нельзя было
// вычислить место точнее самого пункта (требование 9): до километра,
// до пяти, до десяти — чем дальше, тем грубее. Половина — вверх.
func roundDistance(km float64) int32 {
	roundTo := func(step float64) float64 { return step * math.Floor(km/step+0.5) }
	switch {
	case km < 10:
		return int32(math.Max(1, roundTo(1)))
	case km < 100:
		return int32(roundTo(5))
	default:
		return int32(roundTo(10))
	}
}
