package alerts

// AlertPayload getter methods — allow *Alert to satisfy aicontracts.AlertPayload
// without importing pkg/aicontracts (which would create a circular dependency).

func (a *Alert) GetID() string                       { return a.ID }
func (a *Alert) GetType() string                     { return a.Type }
func (a *Alert) GetResourceID() string               { return a.ResourceID }
func (a *Alert) GetResourceName() string             { return a.ResourceName }
func (a *Alert) GetNode() string                     { return a.Node }
func (a *Alert) GetInstance() string                 { return a.Instance }
func (a *Alert) GetMessage() string                  { return a.Message }
func (a *Alert) GetValue() float64                   { return a.Value }
func (a *Alert) GetThreshold() float64               { return a.Threshold }
func (a *Alert) GetMetadata() map[string]interface{} { return a.Metadata }
