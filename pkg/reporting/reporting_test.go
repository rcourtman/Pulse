package reporting

import "testing"

type fakeEngine struct {
	called bool
}

func (f *fakeEngine) Generate(req MetricReportRequest) ([]byte, string, error) {
	f.called = true
	return []byte("ok"), "text/plain", nil
}

func TestSetGetEngine(t *testing.T) {
	engine := &fakeEngine{}
	SetEngine(engine)
	if GetEngine() != engine {
		t.Fatal("expected engine to be set")
	}

	SetEngine(nil)
	if GetEngine() != nil {
		t.Fatal("expected engine to be cleared")
	}
}
