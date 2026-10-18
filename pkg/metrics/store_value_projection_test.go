package metrics

import (
	"fmt"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	pdb "github.com/rcourtman/pulse-go-rewrite/pkg/db"
	"testing"
	"time"
)

func assertMetricValueProjection(t *testing.T, full map[string]map[string][]MetricPoint, values map[string]map[string][]models.MetricPoint) {
	t.Helper()
	if len(full) != len(values) {
		t.Fatalf("resource scope changed: full=%v values=%v", full, values)
	}
	for id, series := range full {
		projected, ok := values[id]
		if !ok || len(series) != len(projected) {
			t.Fatalf("metric scope changed for %s: full=%v values=%v", id, series, projected)
		}
		for metric, points := range series {
			got, ok := projected[metric]
			if !ok || len(got) != len(points) {
				t.Fatalf("series shape changed for %s/%s: full=%v values=%v", id, metric, points, got)
			}
			for i, point := range points {
				if point.Value != got[i].Value || !point.Timestamp.Equal(got[i].Timestamp) {
					t.Fatalf("observation changed for %s/%s[%d]: full=%+v values=%+v", id, metric, i, point, got[i])
				}
			}
		}
	}
}

func TestMetricValueProjectionReadsCurrentDetachedSnapshot(t *testing.T) {
	db := newPlanTestDB(t)
	db.SetMaxOpenConns(1)
	store := &Store{db: pdb.Wrap(db, "value-snapshot-test")}
	end := time.Unix(2000000040, 0)
	start := end.Add(-3 * time.Hour)
	for _, step := range []int64{0, 120} {
		t.Run(fmt.Sprint(step), func(t *testing.T) {
			if _, err := db.Exec("DELETE FROM metrics"); err != nil {
				t.Fatal(err)
			}
			read := func(want *float64) {
				t.Helper()
				full, err := store.QueryAll("node", "a", start, end, step)
				if err != nil {
					t.Fatal(err)
				}
				values, err := store.QueryMetricValues("node", "a", start, end, step)
				if err != nil {
					t.Fatal(err)
				}
				assertMetricValueProjection(t, map[string]map[string][]MetricPoint{"a": full}, map[string]map[string][]models.MetricPoint{"a": values})
				if want == nil {
					if len(values) != 0 {
						t.Fatalf("invented data: %+v", values)
					}
					return
				}
				if len(values["cpu"]) != 1 || values["cpu"][0].Value != *want {
					t.Fatalf("stale data: %+v", values)
				}
				values["cpu"][0].Value = -999
				values["cpu"][0].Timestamp = time.Time{}
				again, err := store.QueryMetricValues("node", "a", start, end, step)
				if err != nil || len(again["cpu"]) != 1 || again["cpu"][0].Value != *want || again["cpu"][0].Timestamp.IsZero() {
					t.Fatalf("caller mutation reached later read: %+v %v", again, err)
				}
			}
			read(nil)
			if _, err := db.Exec(`INSERT INTO metrics(resource_type,resource_id,metric_type,tier,timestamp,value) VALUES ('node','a','cpu','raw',?,23)`, end.Add(-40*time.Second).Unix()); err != nil {
				t.Fatal(err)
			}
			raw := 23.0
			read(&raw)
			if _, err := db.Exec(`INSERT INTO metrics(resource_type,resource_id,metric_type,tier,timestamp,value) VALUES ('node','a','cpu','minute',?,17)`, end.Add(-time.Minute).Unix()); err != nil {
				t.Fatal(err)
			}
			preferred := 17.0
			read(&preferred)
			if _, err := db.Exec("DELETE FROM metrics"); err != nil {
				t.Fatal(err)
			}
			read(nil)
		})
	}
}
