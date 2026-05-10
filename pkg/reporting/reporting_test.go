package reporting

import "testing"

type fakeEngine struct {
	called bool
}

func (f *fakeEngine) Generate(req MetricReportRequest) ([]byte, string, error) {
	f.called = true
	return []byte("ok"), "text/plain", nil
}

func (f *fakeEngine) GenerateMulti(req MultiReportRequest) ([]byte, string, error) {
	f.called = true
	return []byte("ok"), "text/plain", nil
}

func (f *fakeEngine) NarrativeFor(req MetricReportRequest) (*Narrative, error) {
	f.called = true
	return &Narrative{Source: NarrativeSourceHeuristic}, nil
}

func (f *fakeEngine) FleetNarrativeFor(req MultiReportRequest) (*FleetNarrative, error) {
	f.called = true
	return &FleetNarrative{Source: NarrativeSourceHeuristic}, nil
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
