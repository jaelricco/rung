package parks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"calisthenics/api/internal/httpx"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Parks come from OpenStreetMap rather than an empty table we ask users to
// fill. Overpass is queried on demand for a bounding box, results are cached
// in Postgres, and user corrections layer on top later.
const overpassURL = "https://overpass-api.de/api/interpreter"

type Service struct {
	pool *pgxpool.Pool
	http *http.Client
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, http: &http.Client{Timeout: 60 * time.Second}}
}

type Park struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Latitude   float64  `json:"latitude"`
	Longitude  float64  `json:"longitude"`
	Equipment  []string `json:"equipment"`
	Surface    *string  `json:"surface"`
	Roofed     *bool    `json:"roofed"`
	Source     string   `json:"source"`
	DistanceKm *float64 `json:"distance_km,omitempty"`
}

// Nearby returns cached parks around a point, and triggers an import first if
// that area has never been fetched.
func (s *Service) Nearby(w http.ResponseWriter, r *http.Request) {
	lat, err := floatParam(r, "lat")
	if err != nil {
		httpx.Fail(w, http.StatusBadRequest, "Pass a latitude as lat.")
		return
	}
	lng, err := floatParam(r, "lng")
	if err != nil {
		httpx.Fail(w, http.StatusBadRequest, "Pass a longitude as lng.")
		return
	}
	radiusKm := 10.0
	if v := r.URL.Query().Get("radius_km"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil && parsed > 0 && parsed <= 100 {
			radiusKm = parsed
		}
	}

	found, err := s.query(r.Context(), lat, lng, radiusKm)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't search for parks.")
		return
	}

	// Cold area: fetch from OpenStreetMap once, then read again.
	if len(found) == 0 && r.URL.Query().Get("refresh") != "false" {
		if err := s.importArea(r.Context(), lat, lng, radiusKm); err != nil {
			log.Printf("overpass import failed: %v", err)
			httpx.JSON(w, http.StatusOK, []Park{})
			return
		}
		found, err = s.query(r.Context(), lat, lng, radiusKm)
		if err != nil {
			httpx.Fail(w, http.StatusInternalServerError, "Couldn't search for parks.")
			return
		}
	}
	httpx.JSON(w, http.StatusOK, found)
}

// Refresh re-imports an area on demand.
func (s *Service) Refresh(w http.ResponseWriter, r *http.Request) {
	lat, err1 := floatParam(r, "lat")
	lng, err2 := floatParam(r, "lng")
	if err1 != nil || err2 != nil {
		httpx.Fail(w, http.StatusBadRequest, "Pass both lat and lng.")
		return
	}
	radiusKm := 10.0
	if v := r.URL.Query().Get("radius_km"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil && parsed > 0 && parsed <= 50 {
			radiusKm = parsed
		}
	}
	if err := s.importArea(r.Context(), lat, lng, radiusKm); err != nil {
		httpx.Fail(w, http.StatusBadGateway, "OpenStreetMap didn't answer: "+err.Error())
		return
	}
	found, err := s.query(r.Context(), lat, lng, radiusKm)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't search for parks.")
		return
	}
	httpx.JSON(w, http.StatusOK, found)
}

func (s *Service) query(ctx context.Context, lat, lng, radiusKm float64) ([]Park, error) {
	rows, err := s.pool.Query(ctx, `
		select id, name,
		       ST_Y(location::geometry), ST_X(location::geometry),
		       equipment, surface, roofed, source,
		       ST_Distance(location, ST_MakePoint($2, $1)::geography) / 1000.0
		from parks
		where ST_DWithin(location, ST_MakePoint($2, $1)::geography, $3)
		order by location <-> ST_MakePoint($2, $1)::geography
		limit 200`, lat, lng, radiusKm*1000)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Park{}
	for rows.Next() {
		var p Park
		var distance float64
		if err := rows.Scan(&p.ID, &p.Name, &p.Latitude, &p.Longitude,
			&p.Equipment, &p.Surface, &p.Roofed, &p.Source, &distance); err != nil {
			return nil, err
		}
		rounded := float64(int(distance*100)) / 100
		p.DistanceKm = &rounded
		out = append(out, p)
	}
	return out, rows.Err()
}

type overpassResult struct {
	Elements []struct {
		Type   string            `json:"type"`
		ID     int64             `json:"id"`
		Lat    float64           `json:"lat"`
		Lon    float64           `json:"lon"`
		Center *struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"center"`
		Tags map[string]string `json:"tags"`
	} `json:"elements"`
}

// importArea asks Overpass for outdoor fitness and calisthenics facilities in a
// radius and upserts them.
func (s *Service) importArea(ctx context.Context, lat, lng, radiusKm float64) error {
	radiusM := int(radiusKm * 1000)
	query := fmt.Sprintf(`
[out:json][timeout:50];
(
  node["leisure"="fitness_station"](around:%d,%f,%f);
  way["leisure"="fitness_station"](around:%d,%f,%f);
  node["fitness_station"](around:%d,%f,%f);
  way["fitness_station"](around:%d,%f,%f);
  node["sport"="calisthenics"](around:%d,%f,%f);
  way["sport"="calisthenics"](around:%d,%f,%f);
);
out center tags;`,
		radiusM, lat, lng, radiusM, lat, lng,
		radiusM, lat, lng, radiusM, lat, lng,
		radiusM, lat, lng, radiusM, lat, lng)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, overpassURL,
		strings.NewReader(url.Values{"data": {query}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "calisthenics-training-app/0.1")

	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("overpass returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	var result overpassResult
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	for _, element := range result.Elements {
		pointLat, pointLng := element.Lat, element.Lon
		if element.Center != nil {
			pointLat, pointLng = element.Center.Lat, element.Center.Lon
		}
		if pointLat == 0 && pointLng == 0 {
			continue
		}

		name := element.Tags["name"]
		if name == "" {
			name = "Outdoor fitness station"
		}
		equipment := equipmentFromTags(element.Tags)

		var surface *string
		if v, ok := element.Tags["surface"]; ok {
			surface = &v
		}
		var roofed *bool
		if v, ok := element.Tags["covered"]; ok {
			b := v == "yes"
			roofed = &b
		}

		_, err := s.pool.Exec(ctx, `
			insert into parks (osm_type, osm_id, name, location, equipment, surface, roofed, source, updated_at)
			values ($1, $2, $3, ST_MakePoint($5, $4)::geography, $6, $7, $8, 'osm', now())
			on conflict (osm_type, osm_id) where osm_id is not null
			do update set name = excluded.name, location = excluded.location,
			              equipment = excluded.equipment, surface = excluded.surface,
			              roofed = excluded.roofed, updated_at = now()`,
			element.Type, element.ID, name, pointLat, pointLng, equipment, surface, roofed)
		if err != nil {
			log.Printf("upsert park %s/%d: %v", element.Type, element.ID, err)
		}
	}
	return nil
}

// equipmentFromTags maps OSM tagging onto the equipment vocabulary the app
// shows. OSM tags this inconsistently, so both schemes are handled.
func equipmentFromTags(tags map[string]string) []string {
	known := map[string]string{
		"horizontal_bar":  "pull_up_bar",
		"horizontal_bars": "pull_up_bar",
		"pull_up_bar":     "pull_up_bar",
		"parallel_bars":   "parallel_bars",
		"push_up_bar":     "parallel_bars",
		"dip_bar":         "dip_bars",
		"rings":           "rings",
		"wall_bars":       "wall_bars",
		"climbing_rope":   "rope",
		"monkey_bar":      "monkey_bars",
		"monkey_bars":     "monkey_bars",
		"sit_up_bench":    "bench",
		"beam":            "beam",
		"hyperextension":  "back_extension",
		"box":             "box",
	}

	seen := map[string]bool{}
	add := func(v string) {
		if mapped, ok := known[v]; ok && !seen[mapped] {
			seen[mapped] = true
		}
	}

	if v, ok := tags["fitness_station"]; ok && v != "yes" {
		for _, part := range strings.Split(v, ";") {
			add(strings.TrimSpace(part))
		}
	}
	for key, value := range tags {
		if strings.HasPrefix(key, "fitness_station:") && (value == "yes" || value == "1") {
			add(strings.TrimPrefix(key, "fitness_station:"))
		}
	}

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

func floatParam(r *http.Request, name string) (float64, error) {
	return strconv.ParseFloat(r.URL.Query().Get(name), 64)
}
