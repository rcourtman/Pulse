package reporting

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"time"
)

// Backup health data is a shared evidence contract. Paid rendering is supplied
// through the enterprise extension; collection and storage remain canonical.
type BackupHealthData struct {
	Title         string
	GeneratedAt   time.Time
	Brand         *ReportBrand
	Summary       BackupHealthSummary
	HistoryNotice string
}

type BackupHealthSummary = model.BackupHealthSummary
type BackupOutcomeCounts = model.BackupOutcomeCounts
type BackupHealthWorkload = model.BackupHealthWorkload
type BackupHealthWorkloadResult = model.BackupHealthWorkloadResult
type BackupHealthDay = model.BackupHealthDay
type BackupHealthFailure = model.BackupHealthFailure
