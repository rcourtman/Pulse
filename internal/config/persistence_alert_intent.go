package config

import (
	"errors"
	"io/fs"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func (c *ConfigPersistence) SaveAlertIntentPolicies(document alerts.AlertIntentPolicyDocument) error {
	if err := alerts.ValidateAlertIntentPolicyDocument(document); err != nil {
		return err
	}
	document = alerts.NormalizeAlertIntentPolicyDocument(document)
	return saveJSON(c, c.alertIntentFile, document, false)
}

func (c *ConfigPersistence) LoadAlertIntentPolicies() (*alerts.AlertIntentPolicyDocument, error) {
	document := alerts.NewAlertIntentPolicyDocument()
	if err := loadJSON(c, c.alertIntentFile, false, &document); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &document, nil
		}
		return nil, err
	}
	document = alerts.NormalizeAlertIntentPolicyDocument(document)
	if err := alerts.ValidateAlertIntentPolicyDocument(document); err != nil {
		return nil, err
	}
	return &document, nil
}
