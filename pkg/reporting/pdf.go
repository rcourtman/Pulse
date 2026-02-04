package reporting

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"
)

// Color scheme - professional dark blue theme
var (
	colorPrimary     = [3]int{30, 58, 95}    // Dark navy
	colorSecondary   = [3]int{52, 152, 219}  // Bright blue
	colorAccent      = [3]int{46, 204, 113}  // Green
	colorWarning     = [3]int{241, 196, 15}  // Yellow
	colorDanger      = [3]int{231, 76, 60}   // Red
	colorTextDark    = [3]int{44, 62, 80}    // Dark text
	colorTextMuted   = [3]int{127, 140, 141} // Muted text
	colorBackground  = [3]int{248, 249, 250} // Light gray bg
	colorTableHeader = [3]int{30, 58, 95}    // Navy header
	colorTableAlt    = [3]int{241, 245, 249} // Alternating row
	colorGridLine    = [3]int{220, 220, 220} // Chart grid
)

// PDFGenerator handles PDF report generation.
type PDFGenerator struct{}

// NewPDFGenerator creates a new PDF generator.
func NewPDFGenerator() *PDFGenerator {
	return &PDFGenerator{}
}

// Generate creates a PDF report from the provided data.
func (g *PDFGenerator) Generate(data *ReportData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.SetAutoPageBreak(true, 25)

	// Cover page
	g.writeCoverPage(pdf, data)

	// Executive Summary page
	pdf.AddPage()
	g.addPageHeader(pdf, data, "Executive Summary")
	g.writeExecutiveSummary(pdf, data)

	// Resource details page (if enrichment data available)
	if data.Resource != nil {
		pdf.AddPage()
		g.addPageHeader(pdf, data, "Resource Details")
		g.writeResourceDetails(pdf, data)

		// Storage section for nodes
		if len(data.Storage) > 0 {
			g.writeStorageSection(pdf, data)
		}

		// Physical disks section for nodes
		if len(data.Disks) > 0 {
			g.writeDisksSection(pdf, data)
		}

		// Backups section for VMs/containers
		if len(data.Backups) > 0 {
			g.writeBackupsSection(pdf, data)
		}
	}

	// Alerts section (if any)
	if len(data.Alerts) > 0 {
		if pdf.GetY() > 180 {
			pdf.AddPage()
			g.addPageHeader(pdf, data, "Alerts")
		}
		g.writeAlertsSection(pdf, data)
	}

	// Summary page with metrics
	pdf.AddPage()
	g.addPageHeader(pdf, data, "Performance Summary")
	g.writeSummarySection(pdf, data)

	// Charts page(s)
	g.writeChartsSection(pdf, data)

	// Data table page(s)
	g.writeDataSection(pdf, data)

	// Add page numbers to all pages except cover
	g.addPageNumbers(pdf)

	// Output to buffer
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("PDF output error: %w", err)
	}

	return buf.Bytes(), nil
}

// writeCoverPage creates a professional cover page.
func (g *PDFGenerator) writeCoverPage(pdf *fpdf.Fpdf, data *ReportData) {
	pdf.AddPage()

	pageWidth, pageHeight := pdf.GetPageSize()

	// Top accent bar
	pdf.SetFillColor(colorPrimary[0], colorPrimary[1], colorPrimary[2])
	pdf.Rect(0, 0, pageWidth, 8, "F")

	// Pulse branding area
	pdf.SetY(50)
	pdf.SetFont("Arial", "B", 32)
	pdf.SetTextColor(colorPrimary[0], colorPrimary[1], colorPrimary[2])
	pdf.CellFormat(0, 15, "PULSE", "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "", 12)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 8, "Infrastructure Monitoring", "", 1, "C", false, 0, "")

	// Main title
	pdf.SetY(100)
	pdf.SetFont("Arial", "B", 28)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 12, "Performance Report", "", 1, "C", false, 0, "")

	// Resource info box
	pdf.SetY(130)
	boxX := 40.0
	boxWidth := pageWidth - 80
	boxHeight := 50.0

	pdf.SetFillColor(colorBackground[0], colorBackground[1], colorBackground[2])
	pdf.SetDrawColor(colorGridLine[0], colorGridLine[1], colorGridLine[2])
	pdf.RoundedRect(boxX, pdf.GetY(), boxWidth, boxHeight, 3, "1234", "FD")

	// Resource details inside box
	pdf.SetY(pdf.GetY() + 10)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 7, "RESOURCE", "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "B", 16)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 10, data.ResourceID, "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "", 11)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 7, GetResourceTypeDisplayName(data.ResourceType), "", 1, "C", false, 0, "")

	// Time period
	pdf.SetY(200)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 7, "REPORTING PERIOD", "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "", 12)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	periodStr := fmt.Sprintf("%s  -  %s",
		data.Start.Format("January 2, 2006 15:04"),
		data.End.Format("January 2, 2006 15:04"))
	pdf.CellFormat(0, 8, periodStr, "", 1, "C", false, 0, "")

	// Duration
	duration := data.End.Sub(data.Start)
	durationStr := formatDuration(duration)
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 6, fmt.Sprintf("(%s)", durationStr), "", 1, "C", false, 0, "")

	// Bottom section
	pdf.SetY(pageHeight - 50)
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 6, fmt.Sprintf("Generated: %s", data.GeneratedAt.Format("January 2, 2006 at 15:04 MST")), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Data Points: %d", data.TotalPoints), "", 1, "C", false, 0, "")

	// Bottom accent bar
	pdf.SetFillColor(colorPrimary[0], colorPrimary[1], colorPrimary[2])
	pdf.Rect(0, pageHeight-8, pageWidth, 8, "F")
}

// writeExecutiveSummary writes the executive summary with health status
func (g *PDFGenerator) writeExecutiveSummary(pdf *fpdf.Fpdf, data *ReportData) {
	pageWidth, _ := pdf.GetPageSize()

	// Determine overall health status
	healthStatus := "HEALTHY"
	healthColor := colorAccent // Green
	healthMessage := "All systems operating normally"

	activeAlerts := 0
	criticalAlerts := 0
	warningAlerts := 0
	for _, alert := range data.Alerts {
		if alert.ResolvedTime == nil {
			activeAlerts++
			if alert.Level == "critical" {
				criticalAlerts++
			} else {
				warningAlerts++
			}
		}
	}

	if criticalAlerts > 0 {
		healthStatus = "CRITICAL"
		healthColor = colorDanger
		if criticalAlerts == 1 {
			healthMessage = "1 critical issue requires immediate attention"
		} else {
			healthMessage = fmt.Sprintf("%d critical issues require immediate attention", criticalAlerts)
		}
	} else if warningAlerts > 0 {
		healthStatus = "WARNING"
		healthColor = colorWarning
		if warningAlerts == 1 {
			healthMessage = "1 warning detected - review recommended"
		} else {
			healthMessage = fmt.Sprintf("%d warnings detected - review recommended", warningAlerts)
		}
	}

	// Health Status Card
	cardX := 20.0
	cardWidth := pageWidth - 40
	cardHeight := 35.0

	pdf.SetFillColor(healthColor[0], healthColor[1], healthColor[2])
	pdf.RoundedRect(cardX, pdf.GetY(), cardWidth, cardHeight, 3, "1234", "F")

	// Health status text
	pdf.SetXY(cardX, pdf.GetY()+8)
	pdf.SetFont("Arial", "B", 24)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(cardWidth, 12, healthStatus, "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(cardWidth, 8, healthMessage, "", 1, "C", false, 0, "")

	pdf.SetY(pdf.GetY() + 15)

	// Quick Stats - simple table format (avoids fpdf positioning bugs)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 8, "Quick Stats", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Calculate stats
	var avgCPU, avgMem, avgDisk float64
	if stats, ok := data.Summary.ByMetric["cpu"]; ok {
		avgCPU = stats.Avg
	}
	if stats, ok := data.Summary.ByMetric["memory"]; ok {
		avgMem = stats.Avg
	}
	if stats, ok := data.Summary.ByMetric["disk"]; ok {
		avgDisk = stats.Avg
	} else if stats, ok := data.Summary.ByMetric["usage"]; ok {
		avgDisk = stats.Avg
	}

	// Simple table header - darker text for better visibility
	colWidth := 42.5
	pdf.SetFillColor(colorBackground[0], colorBackground[1], colorBackground[2])
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(colWidth, 7, "CPU", "0", 0, "C", true, 0, "")
	pdf.CellFormat(colWidth, 7, "Memory", "0", 0, "C", true, 0, "")
	pdf.CellFormat(colWidth, 7, "Disk", "0", 0, "C", true, 0, "")
	pdf.CellFormat(colWidth, 7, "Alerts", "0", 1, "C", true, 0, "")

	// Values row - large numbers
	pdf.SetFont("Arial", "B", 16)
	pdf.SetTextColor(getStatColor(avgCPU)[0], getStatColor(avgCPU)[1], getStatColor(avgCPU)[2])
	pdf.CellFormat(colWidth, 9, fmt.Sprintf("%.1f%%", avgCPU), "0", 0, "C", false, 0, "")
	pdf.SetTextColor(getStatColor(avgMem)[0], getStatColor(avgMem)[1], getStatColor(avgMem)[2])
	pdf.CellFormat(colWidth, 9, fmt.Sprintf("%.1f%%", avgMem), "0", 0, "C", false, 0, "")
	pdf.SetTextColor(getStatColor(avgDisk)[0], getStatColor(avgDisk)[1], getStatColor(avgDisk)[2])
	pdf.CellFormat(colWidth, 9, fmt.Sprintf("%.1f%%", avgDisk), "0", 0, "C", false, 0, "")
	pdf.SetTextColor(getAlertCountColor(activeAlerts)[0], getAlertCountColor(activeAlerts)[1], getAlertCountColor(activeAlerts)[2])
	pdf.CellFormat(colWidth, 9, fmt.Sprintf("%d", activeAlerts), "0", 1, "C", false, 0, "")

	// Labels row with trend indicators
	pdf.SetFont("Arial", "", 7)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])

	// Calculate trends (compare first half avg to second half avg)
	cpuTrend := g.calculateTrend(data, "cpu")
	memTrend := g.calculateTrend(data, "memory")
	diskTrend := g.calculateTrend(data, "disk")
	if diskTrend == "" {
		diskTrend = g.calculateTrend(data, "usage")
	}

	pdf.CellFormat(colWidth, 5, "avg "+cpuTrend, "0", 0, "C", false, 0, "")
	pdf.CellFormat(colWidth, 5, "avg "+memTrend, "0", 0, "C", false, 0, "")
	pdf.CellFormat(colWidth, 5, "avg "+diskTrend, "0", 0, "C", false, 0, "")
	pdf.CellFormat(colWidth, 5, "active now", "0", 1, "C", false, 0, "")

	pdf.Ln(5)

	// Key Observations section
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 8, "Key Observations", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	observations := g.generateObservations(data)
	pdf.SetFont("Arial", "", 10)
	for _, obs := range observations {
		// Draw colored bullet circle
		bulletX := pdf.GetX() + 3
		bulletY := pdf.GetY() + 3
		pdf.SetFillColor(obs.color[0], obs.color[1], obs.color[2])
		pdf.Circle(bulletX, bulletY, 2, "F")
		pdf.SetX(pdf.GetX() + 8)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(0, 6, obs.text, "", 1, "L", false, 0, "")
		pdf.Ln(1)
	}

	// Active Alerts summary (if any)
	if activeAlerts > 0 {
		pdf.Ln(5)
		pdf.SetFont("Arial", "B", 11)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(0, 8, "Active Alerts", "", 1, "L", false, 0, "")
		pdf.Ln(2)

		pdf.SetFont("Arial", "", 9)
		alertCount := 0
		for _, alert := range data.Alerts {
			if alert.ResolvedTime == nil && alertCount < 5 {
				if alert.Level == "critical" {
					pdf.SetTextColor(colorDanger[0], colorDanger[1], colorDanger[2])
					pdf.CellFormat(8, 5, "!", "", 0, "C", false, 0, "")
				} else {
					pdf.SetTextColor(colorWarning[0], colorWarning[1], colorWarning[2])
					pdf.CellFormat(8, 5, "!", "", 0, "C", false, 0, "")
				}
				pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
				msg := alert.Message
				if len(msg) > 70 {
					msg = msg[:67] + "..."
				}
				pdf.CellFormat(0, 5, msg, "", 1, "L", false, 0, "")
				alertCount++
			}
		}
		if activeAlerts > 5 {
			pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
			pdf.CellFormat(0, 5, fmt.Sprintf("... and %d more alerts", activeAlerts-5), "", 1, "L", false, 0, "")
		}
	}

	// Recommendations section
	recommendations := g.generateRecommendations(data, criticalAlerts, warningAlerts)
	if len(recommendations) > 0 {
		pdf.Ln(5)
		pdf.SetFont("Arial", "B", 11)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(0, 8, "Recommended Actions", "", 1, "L", false, 0, "")
		pdf.Ln(2)

		pdf.SetFont("Arial", "", 9)
		for i, rec := range recommendations {
			if i >= 4 {
				break // Limit to 4 recommendations
			}
			pdf.SetTextColor(colorSecondary[0], colorSecondary[1], colorSecondary[2])
			pdf.CellFormat(6, 5, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")
			pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
			pdf.CellFormat(0, 5, rec, "", 1, "L", false, 0, "")
			pdf.Ln(1)
		}
	}

	pdf.Ln(10)
}

// observation represents a key observation for the executive summary
type observation struct {
	icon  string
	text  string
	color [3]int
}

// generateObservations analyzes the data and generates key observations
func (g *PDFGenerator) generateObservations(data *ReportData) []observation {
	var obs []observation

	// Analyze CPU
	if stats, ok := data.Summary.ByMetric["cpu"]; ok {
		if stats.Max > 90 {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("CPU peaked at %.1f%% - potential capacity constraint", stats.Max),
				color: colorDanger,
			})
		} else if stats.Avg < 20 {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("CPU averaging %.1f%% - resource is underutilized", stats.Avg),
				color: colorAccent,
			})
		} else {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("CPU usage normal (avg %.1f%%, max %.1f%%)", stats.Avg, stats.Max),
				color: colorAccent,
			})
		}
	}

	// Analyze Memory
	if stats, ok := data.Summary.ByMetric["memory"]; ok {
		if stats.Avg > 85 {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("Memory consistently high at %.1f%% avg - consider scaling", stats.Avg),
				color: colorDanger,
			})
		} else if stats.Max > 95 {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("Memory peaked at %.1f%% - near capacity", stats.Max),
				color: colorWarning,
			})
		} else {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("Memory usage healthy (avg %.1f%%)", stats.Avg),
				color: colorAccent,
			})
		}
	}

	// Analyze Disk
	diskKey := "disk"
	if _, hasDisk := data.Summary.ByMetric["disk"]; !hasDisk {
		if _, hasUsage := data.Summary.ByMetric["usage"]; hasUsage {
			diskKey = "usage"
		}
	}
	if stats, ok := data.Summary.ByMetric[diskKey]; ok {
		if stats.Avg > 85 {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("Disk at %.1f%% - plan capacity expansion", stats.Avg),
				color: colorDanger,
			})
		} else if stats.Avg > 70 {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("Disk at %.1f%% - monitor growth trend", stats.Avg),
				color: colorWarning,
			})
		} else {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("Disk usage acceptable at %.1f%%", stats.Avg),
				color: colorAccent,
			})
		}
	}

	// Alert summary
	resolved := 0
	for _, alert := range data.Alerts {
		if alert.ResolvedTime != nil {
			resolved++
		}
	}
	if resolved > 0 {
		obs = append(obs, observation{
			icon:  "-",
			text:  fmt.Sprintf("%d alerts were triggered and resolved during this period", resolved),
			color: colorSecondary,
		})
	}

	// Physical disk health check (WearLevel = SSD life remaining, 100% = healthy, 0% = end of life)
	for _, disk := range data.Disks {
		if disk.WearLevel > 0 && disk.WearLevel <= 10 {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("CRITICAL: Disk %s has only %d%% life remaining - replace immediately", disk.Device, disk.WearLevel),
				color: colorDanger,
			})
		} else if disk.WearLevel > 0 && disk.WearLevel <= 30 {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("Disk %s has %d%% life remaining - plan replacement", disk.Device, disk.WearLevel),
				color: colorWarning,
			})
		}
		// Check disk health
		if disk.Health == "FAILED" {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("CRITICAL: Disk %s SMART health check FAILED", disk.Device),
				color: colorDanger,
			})
		}
	}

	// Uptime observation
	if data.Resource != nil && data.Resource.Uptime > 0 {
		uptimeDays := data.Resource.Uptime / 86400
		if uptimeDays > 90 {
			obs = append(obs, observation{
				icon:  "-",
				text:  fmt.Sprintf("System uptime: %d days - consider scheduling maintenance", uptimeDays),
				color: colorWarning,
			})
		}
	}

	// If no observations, add a default one
	if len(obs) == 0 {
		obs = append(obs, observation{
			icon:  "-",
			text:  "Insufficient data for detailed analysis",
			color: colorTextMuted,
		})
	}

	return obs
}

// calculateTrend compares first half to second half of data points
func (g *PDFGenerator) calculateTrend(data *ReportData, metricType string) string {
	points, ok := data.Metrics[metricType]
	if !ok || len(points) < 10 {
		return ""
	}

	// Calculate average of first half and second half
	mid := len(points) / 2
	var firstSum, secondSum float64
	for i := 0; i < mid; i++ {
		firstSum += points[i].Value
	}
	for i := mid; i < len(points); i++ {
		secondSum += points[i].Value
	}
	firstAvg := firstSum / float64(mid)
	secondAvg := secondSum / float64(len(points)-mid)

	// Calculate percentage change
	if firstAvg == 0 {
		return ""
	}
	change := ((secondAvg - firstAvg) / firstAvg) * 100

	// Only show trend if significant (>5% change)
	if change > 5 {
		return "(trending up)"
	} else if change < -5 {
		return "(trending down)"
	}
	return "(stable)"
}

// generateRecommendations creates actionable recommendations based on data
func (g *PDFGenerator) generateRecommendations(data *ReportData, criticalAlerts, warningAlerts int) []string {
	var recs []string

	// Critical disk health - highest priority (WearLevel = life remaining, 100% = healthy)
	for _, disk := range data.Disks {
		if disk.WearLevel > 0 && disk.WearLevel <= 10 {
			recs = append(recs, fmt.Sprintf("Replace disk %s immediately (only %d%% life remaining)", disk.Device, disk.WearLevel))
		} else if disk.WearLevel > 0 && disk.WearLevel <= 30 {
			recs = append(recs, fmt.Sprintf("Schedule replacement for disk %s within 3-6 months (%d%% life remaining)", disk.Device, disk.WearLevel))
		}
		if disk.Health == "FAILED" {
			recs = append(recs, fmt.Sprintf("Investigate and replace disk %s - SMART health check failed", disk.Device))
		}
	}

	// Critical alerts need attention
	if criticalAlerts > 0 {
		recs = append(recs, "Investigate and resolve critical alerts immediately")
	}

	// High resource usage
	if stats, ok := data.Summary.ByMetric["memory"]; ok {
		if stats.Avg > 85 {
			recs = append(recs, "Consider adding memory or optimizing memory-intensive workloads")
		}
	}
	if stats, ok := data.Summary.ByMetric["cpu"]; ok {
		if stats.Max > 90 {
			recs = append(recs, "Review CPU-intensive processes during peak usage periods")
		}
	}

	// Disk space
	diskKey := "disk"
	if _, ok := data.Summary.ByMetric["disk"]; !ok {
		diskKey = "usage"
	}
	if stats, ok := data.Summary.ByMetric[diskKey]; ok {
		if stats.Avg > 85 {
			recs = append(recs, "Clean up disk space or expand storage capacity")
		}
	}

	// Storage pool warnings
	for _, storage := range data.Storage {
		if storage.UsagePerc >= 90 {
			recs = append(recs, fmt.Sprintf("Expand storage pool '%s' (currently at %.0f%% capacity)", storage.Name, storage.UsagePerc))
		}
	}

	// Long uptime
	if data.Resource != nil && data.Resource.Uptime > 0 {
		uptimeDays := data.Resource.Uptime / 86400
		if uptimeDays > 90 {
			recs = append(recs, "Schedule maintenance window to apply pending updates and reboot")
		}
	}

	// Underutilization suggestion
	if stats, ok := data.Summary.ByMetric["cpu"]; ok {
		if stats.Avg < 10 && len(recs) == 0 {
			recs = append(recs, "System is underutilized - consider consolidating workloads")
		}
	}

	// Default good state message
	if len(recs) == 0 {
		recs = append(recs, "No immediate action required - continue monitoring")
	}

	return recs
}

// getStatColor returns color based on percentage value
func getStatColor(val float64) [3]int {
	if val >= 90 {
		return colorDanger
	} else if val >= 75 {
		return colorWarning
	}
	return colorAccent
}

// getAlertCountColor returns color based on alert count
func getAlertCountColor(count int) [3]int {
	if count > 0 {
		return colorDanger
	}
	return colorAccent
}

// addPageHeader adds a consistent header to content pages.
func (g *PDFGenerator) addPageHeader(pdf *fpdf.Fpdf, data *ReportData, section string) {
	pageWidth, _ := pdf.GetPageSize()

	// Top line
	pdf.SetDrawColor(colorPrimary[0], colorPrimary[1], colorPrimary[2])
	pdf.SetLineWidth(0.5)
	pdf.Line(20, 15, pageWidth-20, 15)

	// Header text
	pdf.SetY(18)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(colorPrimary[0], colorPrimary[1], colorPrimary[2])
	pdf.CellFormat(0, 5, "PULSE PERFORMANCE REPORT", "", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 5, data.ResourceID, "", 1, "R", false, 0, "")

	// Section title
	pdf.SetY(30)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 10, section, "", 1, "L", false, 0, "")

	pdf.Ln(5)
}

// writeSummarySection writes the metrics summary with stats cards.
func (g *PDFGenerator) writeSummarySection(pdf *fpdf.Fpdf, data *ReportData) {
	// Time period subtitle
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	periodStr := fmt.Sprintf("Statistics for %s to %s (%s)",
		data.Start.Format("Jan 2, 2006 15:04"),
		data.End.Format("Jan 2, 2006 15:04"),
		formatDuration(data.End.Sub(data.Start)))
	pdf.CellFormat(0, 6, periodStr, "", 1, "L", false, 0, "")
	pdf.Ln(5)

	if len(data.Summary.ByMetric) == 0 {
		pdf.SetFont("Arial", "I", 11)
		pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
		pdf.CellFormat(0, 10, "No metrics data available for this period.", "", 1, "L", false, 0, "")
		return
	}

	// Get sorted metric names
	metricNames := make([]string, 0, len(data.Summary.ByMetric))
	for name := range data.Summary.ByMetric {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	// Stats cards - 2 per row
	cardWidth := 82.0
	cardHeight := 45.0
	cardGap := 6.0
	startX := 20.0
	rowStartY := pdf.GetY()

	for i, metricType := range metricNames {
		stats := data.Summary.ByMetric[metricType]
		unit := GetMetricUnit(metricType)

		col := i % 2
		if col == 0 && i > 0 {
			// Move to next row
			rowStartY = rowStartY + cardHeight + cardGap
			pdf.SetY(rowStartY)
		} else if col == 1 {
			// Second column - return to row start Y
			pdf.SetY(rowStartY)
		}

		cardX := startX + float64(col)*(cardWidth+cardGap)
		cardY := rowStartY

		// Card background
		pdf.SetFillColor(255, 255, 255)
		pdf.SetDrawColor(colorGridLine[0], colorGridLine[1], colorGridLine[2])
		pdf.RoundedRect(cardX, cardY, cardWidth, cardHeight, 2, "1234", "FD")

		// Card header with color bar
		headerColor := getMetricColor(metricType)
		pdf.SetFillColor(headerColor[0], headerColor[1], headerColor[2])
		pdf.Rect(cardX, cardY, cardWidth, 3, "F")

		// Metric name
		pdf.SetXY(cardX+5, cardY+6)
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(cardWidth-10, 6, GetMetricTypeDisplayName(metricType), "", 1, "L", false, 0, "")

		// Current value (large)
		pdf.SetXY(cardX+5, cardY+14)
		pdf.SetFont("Arial", "B", 20)
		pdf.SetTextColor(headerColor[0], headerColor[1], headerColor[2])
		pdf.CellFormat(cardWidth-10, 10, formatValue(stats.Current, unit)+unit, "", 1, "L", false, 0, "")

		// Stats row
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
		statsY := cardY + 28

		// Min
		pdf.SetXY(cardX+5, statsY)
		pdf.CellFormat(25, 5, "Min", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(0, 5, formatValue(stats.Min, unit)+unit, "", 1, "L", false, 0, "")

		// Max
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
		pdf.SetXY(cardX+5, statsY+6)
		pdf.CellFormat(25, 5, "Max", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(0, 5, formatValue(stats.Max, unit)+unit, "", 1, "L", false, 0, "")

		// Avg
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
		pdf.SetXY(cardX+45, statsY)
		pdf.CellFormat(15, 5, "Avg", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(0, 5, formatValue(stats.Avg, unit)+unit, "", 1, "L", false, 0, "")

		// Count
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
		pdf.SetXY(cardX+45, statsY+6)
		pdf.CellFormat(15, 5, "Samples", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(0, 5, fmt.Sprintf("%d", stats.Count), "", 1, "L", false, 0, "")
	}

	// Calculate final Y position based on number of rows
	numRows := (len(metricNames) + 1) / 2 // Round up
	finalY := rowStartY + float64(numRows)*(cardHeight+cardGap)
	pdf.SetY(finalY)
}

// writeChartsSection writes charts for each metric.
func (g *PDFGenerator) writeChartsSection(pdf *fpdf.Fpdf, data *ReportData) {
	if len(data.Metrics) == 0 {
		return
	}

	// Get sorted metric names
	metricNames := make([]string, 0, len(data.Metrics))
	for name := range data.Metrics {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	chartWidth := 170.0
	chartHeight := 55.0

	// Count valid charts
	validCharts := 0
	for _, metricType := range metricNames {
		if len(data.Metrics[metricType]) >= 2 {
			validCharts++
		}
	}
	if validCharts == 0 {
		return
	}

	// Always start charts on a new page with proper header
	pdf.AddPage()
	g.addPageHeader(pdf, data, "Performance Charts")

	for _, metricType := range metricNames {
		points := data.Metrics[metricType]
		if len(points) < 2 {
			continue
		}

		// Check if we need a new page (need space for chart title + chart + labels)
		if pdf.GetY() > 195 {
			pdf.AddPage()
			g.addPageHeader(pdf, data, "Performance Charts")
		}

		// Chart title
		pdf.SetFont("Arial", "B", 11)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		unit := GetMetricUnit(metricType)
		titleStr := GetMetricTypeDisplayName(metricType)
		if unit != "" {
			titleStr = fmt.Sprintf("%s (%s)", titleStr, unit)
		}
		pdf.CellFormat(0, 7, titleStr, "", 1, "L", false, 0, "")

		chartX := 20.0
		chartY := pdf.GetY()

		g.drawChart(pdf, points, chartX, chartY, chartWidth, chartHeight, metricType)

		pdf.SetY(chartY + chartHeight + 12)
	}
}

// drawChart draws a single chart with grid, area fill, and line.
func (g *PDFGenerator) drawChart(pdf *fpdf.Fpdf, points []MetricDataPoint, x, y, width, height float64, metricType string) {
	// Chart background
	pdf.SetFillColor(255, 255, 255)
	pdf.SetDrawColor(colorGridLine[0], colorGridLine[1], colorGridLine[2])
	pdf.SetLineWidth(0.3)
	pdf.Rect(x, y, width, height, "FD")

	// Find min/max for scaling
	minVal, maxVal := points[0].Value, points[0].Value
	for _, p := range points {
		if p.Value < minVal {
			minVal = p.Value
		}
		if p.Value > maxVal {
			maxVal = p.Value
		}
	}

	// Add padding to range
	valRange := maxVal - minVal
	if valRange < 1 {
		valRange = 10
	}
	minVal = math.Max(0, minVal-valRange*0.1)
	maxVal = maxVal + valRange*0.1

	// Draw horizontal grid lines and Y-axis labels
	pdf.SetFont("Arial", "", 7)
	numGridLines := 5
	for i := 0; i <= numGridLines; i++ {
		gridY := y + height - (float64(i)/float64(numGridLines))*height
		val := minVal + (float64(i)/float64(numGridLines))*(maxVal-minVal)

		// Grid line
		pdf.SetDrawColor(colorGridLine[0], colorGridLine[1], colorGridLine[2])
		pdf.SetLineWidth(0.1)
		pdf.Line(x, gridY, x+width, gridY)

		// Y-axis label
		pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
		pdf.SetXY(x-15, gridY-2)
		pdf.CellFormat(12, 5, fmt.Sprintf("%.0f", val), "", 0, "R", false, 0, "")
	}

	// Time calculations
	startTime := points[0].Timestamp.Unix()
	endTime := points[len(points)-1].Timestamp.Unix()
	timeRange := float64(endTime - startTime)
	if timeRange == 0 {
		timeRange = 1
	}

	// Build polygon points for area fill
	chartColor := getMetricColor(metricType)

	// Draw area fill
	pdf.SetFillColor(chartColor[0], chartColor[1], chartColor[2])
	pdf.SetAlpha(0.15, "Normal")

	polyStr := ""
	for i, p := range points {
		xPos := x + 2 + (float64(p.Timestamp.Unix()-startTime)/timeRange)*(width-4)
		yPos := y + height - 2 - ((p.Value-minVal)/(maxVal-minVal))*(height-4)
		yPos = math.Max(y+2, math.Min(y+height-2, yPos))

		if i == 0 {
			polyStr = fmt.Sprintf("%.2f %.2f m ", xPos, y+height-2)
		}
		polyStr += fmt.Sprintf("%.2f %.2f l ", xPos, yPos)
	}
	// Close polygon
	lastX := x + 2 + (float64(points[len(points)-1].Timestamp.Unix()-startTime)/timeRange)*(width-4)
	polyStr += fmt.Sprintf("%.2f %.2f l h f", lastX, y+height-2)

	// Use raw PDF drawing for polygon
	// Actually, fpdf doesn't support arbitrary polygons easily, so we'll use a different approach
	// Draw as many small rectangles to approximate the fill
	pdf.SetAlpha(0.2, "Normal")
	for i := 1; i < len(points); i++ {
		p1 := points[i-1]
		p2 := points[i]

		x1 := x + 2 + (float64(p1.Timestamp.Unix()-startTime)/timeRange)*(width-4)
		x2 := x + 2 + (float64(p2.Timestamp.Unix()-startTime)/timeRange)*(width-4)
		y1 := y + height - 2 - ((p1.Value-minVal)/(maxVal-minVal))*(height-4)
		y2 := y + height - 2 - ((p2.Value-minVal)/(maxVal-minVal))*(height-4)

		y1 = math.Max(y+2, math.Min(y+height-2, y1))
		y2 = math.Max(y+2, math.Min(y+height-2, y2))

		// Draw trapezoid approximation using polygon
		pdf.Polygon([]fpdf.PointType{
			{X: x1, Y: y1},
			{X: x2, Y: y2},
			{X: x2, Y: y + height - 2},
			{X: x1, Y: y + height - 2},
		}, "F")
	}

	pdf.SetAlpha(1, "Normal")

	// Draw the line
	pdf.SetDrawColor(chartColor[0], chartColor[1], chartColor[2])
	pdf.SetLineWidth(0.8)

	prevX, prevY := 0.0, 0.0
	for i, p := range points {
		xPos := x + 2 + (float64(p.Timestamp.Unix()-startTime)/timeRange)*(width-4)
		yPos := y + height - 2 - ((p.Value-minVal)/(maxVal-minVal))*(height-4)
		yPos = math.Max(y+2, math.Min(y+height-2, yPos))

		if i > 0 {
			pdf.Line(prevX, prevY, xPos, yPos)
		}
		prevX, prevY = xPos, yPos
	}

	// X-axis labels
	pdf.SetFont("Arial", "", 7)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.SetXY(x, y+height+1)
	pdf.CellFormat(40, 4, points[0].Timestamp.Format("Jan 2 15:04"), "", 0, "L", false, 0, "")
	pdf.SetXY(x+width-40, y+height+1)
	pdf.CellFormat(40, 4, points[len(points)-1].Timestamp.Format("Jan 2 15:04"), "", 0, "R", false, 0, "")
}

// writeDataSection writes the data table.
func (g *PDFGenerator) writeDataSection(pdf *fpdf.Fpdf, data *ReportData) {
	if len(data.Metrics) == 0 {
		return
	}

	// Always start data section on a new page with proper header
	pdf.AddPage()
	g.addPageHeader(pdf, data, "Data Sample")

	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 6, "Showing up to 50 data points. Export as CSV for the complete dataset.", "", 1, "L", false, 0, "")
	pdf.Ln(3)

	// Get sorted metric names
	metricNames := make([]string, 0, len(data.Metrics))
	for name := range data.Metrics {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)

	// Calculate column widths - ensure enough space for metric headers
	timestampWidth := 35.0
	availableWidth := 170.0 - timestampWidth
	metricWidth := availableWidth / float64(len(metricNames))
	if metricWidth < 30 {
		metricWidth = 30
	}

	// Table header
	pdf.SetFillColor(colorTableHeader[0], colorTableHeader[1], colorTableHeader[2])
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 7)

	pdf.CellFormat(timestampWidth, 7, "Timestamp", "1", 0, "C", true, 0, "")
	for _, name := range metricNames {
		displayName := GetMetricTypeDisplayName(name)
		unit := GetMetricUnit(name)
		if unit != "" {
			displayName = fmt.Sprintf("%s (%s)", displayName, unit)
		}
		if len(displayName) > 18 {
			displayName = displayName[:18]
		}
		pdf.CellFormat(metricWidth, 7, displayName, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Collect timestamps
	timestampSet := make(map[int64]bool)
	metricsByTime := make(map[string]map[int64]float64)

	for metricName, points := range data.Metrics {
		metricsByTime[metricName] = make(map[int64]float64)
		for _, p := range points {
			ts := p.Timestamp.Unix()
			timestampSet[ts] = true
			metricsByTime[metricName][ts] = p.Value
		}
	}

	timestamps := make([]int64, 0, len(timestampSet))
	for ts := range timestampSet {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })

	// Limit rows
	if len(timestamps) > 50 {
		timestamps = timestamps[:50]
	}

	// Table rows
	pdf.SetFont("Arial", "", 7)
	fill := false

	for rowIdx, ts := range timestamps {
		// Check page break
		if pdf.GetY() > 260 {
			pdf.AddPage()
			g.addPageHeader(pdf, data, "Data Sample (continued)")
			pdf.Ln(5)

			// Re-draw header
			pdf.SetFillColor(colorTableHeader[0], colorTableHeader[1], colorTableHeader[2])
			pdf.SetTextColor(255, 255, 255)
			pdf.SetFont("Arial", "B", 7)
			pdf.CellFormat(timestampWidth, 7, "Timestamp", "1", 0, "C", true, 0, "")
			for _, name := range metricNames {
				displayName := GetMetricTypeDisplayName(name)
				unit := GetMetricUnit(name)
				if unit != "" {
					displayName = fmt.Sprintf("%s (%s)", displayName, unit)
				}
				if len(displayName) > 18 {
					displayName = displayName[:18]
				}
				pdf.CellFormat(metricWidth, 7, displayName, "1", 0, "C", true, 0, "")
			}
			pdf.Ln(-1)
			pdf.SetFont("Arial", "", 7)
			fill = false
		}

		if fill {
			pdf.SetFillColor(colorTableAlt[0], colorTableAlt[1], colorTableAlt[2])
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		t := time.Unix(ts, 0)
		pdf.CellFormat(timestampWidth, 6, t.Format("Jan 02 15:04:05"), "1", 0, "L", fill, 0, "")

		for _, metricName := range metricNames {
			if val, ok := metricsByTime[metricName][ts]; ok {
				pdf.CellFormat(metricWidth, 6, fmt.Sprintf("%.2f", val), "1", 0, "C", fill, 0, "")
			} else {
				pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
				pdf.CellFormat(metricWidth, 6, "-", "1", 0, "C", fill, 0, "")
				pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
			}
		}
		pdf.Ln(-1)
		fill = !fill
		_ = rowIdx
	}
}

// writeResourceDetails writes resource information section
func (g *PDFGenerator) writeResourceDetails(pdf *fpdf.Fpdf, data *ReportData) {
	if data.Resource == nil {
		return
	}

	res := data.Resource

	// Info grid - 2 columns for short fields, full width for long fields
	pdf.SetFont("Arial", "", 10)
	leftCol := 20.0
	rightCol := 105.0
	labelWidth := 30.0
	valueWidth := 50.0

	// Helper to write a label-value pair in a column
	writeField := func(x float64, label, value string) {
		pdf.SetXY(x, pdf.GetY())
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
		pdf.CellFormat(labelWidth, 6, label, "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		// Truncate long values to fit in column
		if len(value) > 35 {
			value = value[:32] + "..."
		}
		pdf.CellFormat(valueWidth, 6, value, "", 0, "L", false, 0, "")
	}

	// Helper to write a full-width label-value pair
	writeFullWidth := func(label, value string) {
		pdf.SetX(leftCol)
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
		pdf.CellFormat(labelWidth, 6, label, "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		// Allow longer values for full-width fields
		if len(value) > 80 {
			value = value[:77] + "..."
		}
		pdf.CellFormat(0, 6, value, "", 1, "L", false, 0, "")
	}

	startY := pdf.GetY()

	// Left column - basic info
	writeField(leftCol, "Name:", res.Name)
	pdf.SetY(pdf.GetY() + 7)
	writeField(leftCol, "Status:", res.Status)
	pdf.SetY(pdf.GetY() + 7)
	if res.Node != "" {
		writeField(leftCol, "Node:", res.Node)
		pdf.SetY(pdf.GetY() + 7)
	}
	if res.Host != "" {
		writeField(leftCol, "Host:", res.Host)
		pdf.SetY(pdf.GetY() + 7)
	}
	if res.Uptime > 0 {
		writeField(leftCol, "Uptime:", formatUptime(res.Uptime))
		pdf.SetY(pdf.GetY() + 7)
	}

	leftEndY := pdf.GetY()

	// Right column - hardware info
	pdf.SetY(startY)
	if res.CPUCores > 0 {
		coreStr := fmt.Sprintf("%d cores", res.CPUCores)
		if res.CPUSockets > 0 {
			coreStr = fmt.Sprintf("%d cores (%d sockets)", res.CPUCores, res.CPUSockets)
		}
		writeField(rightCol, "Cores:", coreStr)
		pdf.SetY(pdf.GetY() + 7)
	}
	if res.MemoryTotal > 0 {
		writeField(rightCol, "Memory:", formatBytes(float64(res.MemoryTotal)))
		pdf.SetY(pdf.GetY() + 7)
	}
	if res.DiskTotal > 0 {
		writeField(rightCol, "Disk:", formatBytes(float64(res.DiskTotal)))
		pdf.SetY(pdf.GetY() + 7)
	}
	if res.Temperature != nil {
		writeField(rightCol, "CPU Temp:", fmt.Sprintf("%.0fC", *res.Temperature))
		pdf.SetY(pdf.GetY() + 7)
	}
	if len(res.LoadAverage) >= 3 {
		writeField(rightCol, "Load:", fmt.Sprintf("%.2f, %.2f, %.2f", res.LoadAverage[0], res.LoadAverage[1], res.LoadAverage[2]))
		pdf.SetY(pdf.GetY() + 7)
	}

	rightEndY := pdf.GetY()
	if leftEndY > rightEndY {
		pdf.SetY(leftEndY)
	}
	pdf.SetY(pdf.GetY() + 3)

	// Full-width fields for long values
	if res.CPUModel != "" {
		writeFullWidth("CPU:", res.CPUModel)
	}
	if res.KernelVersion != "" {
		writeFullWidth("Kernel:", res.KernelVersion)
	}
	if res.PVEVersion != "" {
		writeFullWidth("PVE:", res.PVEVersion)
	}
	if res.OSName != "" {
		osStr := res.OSName
		if res.OSVersion != "" {
			osStr = fmt.Sprintf("%s %s", res.OSName, res.OSVersion)
		}
		writeFullWidth("OS:", osStr)
	}
	if len(res.IPAddresses) > 0 {
		writeFullWidth("IP:", res.IPAddresses[0])
	}

	// Tags
	if len(res.Tags) > 0 {
		pdf.SetY(pdf.GetY() + 3)
		pdf.SetX(leftCol)
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
		pdf.CellFormat(labelWidth, 6, "Tags:", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		tagStr := ""
		for i, tag := range res.Tags {
			if i > 0 {
				tagStr += ", "
			}
			tagStr += tag
		}
		pdf.CellFormat(0, 6, tagStr, "", 1, "L", false, 0, "")
	}

	pdf.Ln(8)
}

// writeAlertsSection writes the alerts table
func (g *PDFGenerator) writeAlertsSection(pdf *fpdf.Fpdf, data *ReportData) {
	if len(data.Alerts) == 0 {
		return
	}

	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 8, "Alerts During Period", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Table header
	colWidths := []float64{25, 20, 65, 30, 30}
	headers := []string{"Type", "Level", "Message", "Started", "Resolved"}

	pdf.SetFillColor(colorTableHeader[0], colorTableHeader[1], colorTableHeader[2])
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 8)

	for i, header := range headers {
		pdf.CellFormat(colWidths[i], 7, header, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Table rows
	pdf.SetFont("Arial", "", 8)
	fill := false

	for _, alert := range data.Alerts {
		if fill {
			pdf.SetFillColor(colorTableAlt[0], colorTableAlt[1], colorTableAlt[2])
		} else {
			pdf.SetFillColor(255, 255, 255)
		}

		// Type
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(colWidths[0], 6, alert.Type, "1", 0, "L", fill, 0, "")

		// Level with color
		if alert.Level == "critical" {
			pdf.SetTextColor(colorDanger[0], colorDanger[1], colorDanger[2])
		} else {
			pdf.SetTextColor(colorWarning[0], colorWarning[1], colorWarning[2])
		}
		pdf.CellFormat(colWidths[1], 6, alert.Level, "1", 0, "C", fill, 0, "")
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		// Message (truncate if too long)
		msg := alert.Message
		if len(msg) > 45 {
			msg = msg[:42] + "..."
		}
		pdf.CellFormat(colWidths[2], 6, msg, "1", 0, "L", fill, 0, "")

		// Started
		pdf.CellFormat(colWidths[3], 6, alert.StartTime.Format("Jan 02 15:04"), "1", 0, "C", fill, 0, "")

		// Resolved
		if alert.ResolvedTime != nil {
			pdf.SetTextColor(colorAccent[0], colorAccent[1], colorAccent[2])
			pdf.CellFormat(colWidths[4], 6, alert.ResolvedTime.Format("Jan 02 15:04"), "1", 0, "C", fill, 0, "")
		} else {
			pdf.SetTextColor(colorDanger[0], colorDanger[1], colorDanger[2])
			pdf.CellFormat(colWidths[4], 6, "Active", "1", 0, "C", fill, 0, "")
		}
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		pdf.Ln(-1)
		fill = !fill
	}

	pdf.Ln(10)
}

// writeStorageSection writes storage pools table
func (g *PDFGenerator) writeStorageSection(pdf *fpdf.Fpdf, data *ReportData) {
	if len(data.Storage) == 0 {
		return
	}

	// Check if we need a new page
	if pdf.GetY() > 200 {
		pdf.AddPage()
		g.addPageHeader(pdf, data, "Storage Pools")
	}

	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 8, "Storage Pools", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Table header
	colWidths := []float64{35, 25, 20, 30, 30, 30}
	headers := []string{"Name", "Type", "Status", "Used", "Total", "Usage"}

	pdf.SetFillColor(colorTableHeader[0], colorTableHeader[1], colorTableHeader[2])
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 8)

	for i, header := range headers {
		pdf.CellFormat(colWidths[i], 7, header, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Table rows
	pdf.SetFont("Arial", "", 8)
	fill := false

	for _, storage := range data.Storage {
		if fill {
			pdf.SetFillColor(colorTableAlt[0], colorTableAlt[1], colorTableAlt[2])
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		pdf.CellFormat(colWidths[0], 6, storage.Name, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(colWidths[1], 6, storage.Type, "1", 0, "C", fill, 0, "")

		// Status with color
		if storage.Status == "active" || storage.Status == "available" {
			pdf.SetTextColor(colorAccent[0], colorAccent[1], colorAccent[2])
		} else {
			pdf.SetTextColor(colorWarning[0], colorWarning[1], colorWarning[2])
		}
		pdf.CellFormat(colWidths[2], 6, storage.Status, "1", 0, "C", fill, 0, "")
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		pdf.CellFormat(colWidths[3], 6, formatBytes(float64(storage.Used)), "1", 0, "R", fill, 0, "")
		pdf.CellFormat(colWidths[4], 6, formatBytes(float64(storage.Total)), "1", 0, "R", fill, 0, "")

		// Usage with color coding
		usageStr := fmt.Sprintf("%.1f%%", storage.UsagePerc)
		if storage.UsagePerc >= 90 {
			pdf.SetTextColor(colorDanger[0], colorDanger[1], colorDanger[2])
		} else if storage.UsagePerc >= 80 {
			pdf.SetTextColor(colorWarning[0], colorWarning[1], colorWarning[2])
		} else {
			pdf.SetTextColor(colorAccent[0], colorAccent[1], colorAccent[2])
		}
		pdf.CellFormat(colWidths[5], 6, usageStr, "1", 0, "C", fill, 0, "")
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		pdf.Ln(-1)
		fill = !fill
	}

	pdf.Ln(10)
}

// writeDisksSection writes physical disks table
func (g *PDFGenerator) writeDisksSection(pdf *fpdf.Fpdf, data *ReportData) {
	if len(data.Disks) == 0 {
		return
	}

	// Check if we need a new page
	if pdf.GetY() > 200 {
		pdf.AddPage()
		g.addPageHeader(pdf, data, "Physical Disks")
	}

	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 8, "Physical Disks", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Table header
	colWidths := []float64{25, 50, 25, 25, 20, 25}
	headers := []string{"Device", "Model", "Size", "Health", "Temp", "Life"}

	pdf.SetFillColor(colorTableHeader[0], colorTableHeader[1], colorTableHeader[2])
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 8)

	for i, header := range headers {
		pdf.CellFormat(colWidths[i], 7, header, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Table rows
	pdf.SetFont("Arial", "", 8)
	fill := false

	for _, disk := range data.Disks {
		if fill {
			pdf.SetFillColor(colorTableAlt[0], colorTableAlt[1], colorTableAlt[2])
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		pdf.CellFormat(colWidths[0], 6, disk.Device, "1", 0, "L", fill, 0, "")

		// Model (truncate if too long)
		model := disk.Model
		if len(model) > 30 {
			model = model[:27] + "..."
		}
		pdf.CellFormat(colWidths[1], 6, model, "1", 0, "L", fill, 0, "")

		pdf.CellFormat(colWidths[2], 6, formatBytes(float64(disk.Size)), "1", 0, "R", fill, 0, "")

		// Health with color
		if disk.Health == "PASSED" {
			pdf.SetTextColor(colorAccent[0], colorAccent[1], colorAccent[2])
		} else if disk.Health == "FAILED" {
			pdf.SetTextColor(colorDanger[0], colorDanger[1], colorDanger[2])
		} else {
			pdf.SetTextColor(colorWarning[0], colorWarning[1], colorWarning[2])
		}
		pdf.CellFormat(colWidths[3], 6, disk.Health, "1", 0, "C", fill, 0, "")
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		// Temperature
		tempStr := "-"
		if disk.Temperature > 0 {
			tempStr = fmt.Sprintf("%dC", disk.Temperature)
			if disk.Temperature >= 60 {
				pdf.SetTextColor(colorDanger[0], colorDanger[1], colorDanger[2])
			} else if disk.Temperature >= 50 {
				pdf.SetTextColor(colorWarning[0], colorWarning[1], colorWarning[2])
			}
		}
		pdf.CellFormat(colWidths[4], 6, tempStr, "1", 0, "C", fill, 0, "")
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		// SSD Life remaining (100% = healthy, 0% = end of life)
		lifeStr := "-"
		if disk.WearLevel > 0 && disk.WearLevel <= 100 {
			lifeStr = fmt.Sprintf("%d%%", disk.WearLevel)
			if disk.WearLevel <= 10 {
				pdf.SetTextColor(colorDanger[0], colorDanger[1], colorDanger[2])
			} else if disk.WearLevel <= 30 {
				pdf.SetTextColor(colorWarning[0], colorWarning[1], colorWarning[2])
			} else {
				pdf.SetTextColor(colorAccent[0], colorAccent[1], colorAccent[2])
			}
		}
		pdf.CellFormat(colWidths[5], 6, lifeStr, "1", 0, "C", fill, 0, "")
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		pdf.Ln(-1)
		fill = !fill
	}

	pdf.Ln(10)
}

// writeBackupsSection writes backups table
func (g *PDFGenerator) writeBackupsSection(pdf *fpdf.Fpdf, data *ReportData) {
	if len(data.Backups) == 0 {
		return
	}

	// Check if we need a new page
	if pdf.GetY() > 200 {
		pdf.AddPage()
		g.addPageHeader(pdf, data, "Backups")
	}

	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 8, "Backups", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Table header
	colWidths := []float64{25, 35, 45, 35, 30}
	headers := []string{"Type", "Storage", "Date", "Size", "Protected"}

	pdf.SetFillColor(colorTableHeader[0], colorTableHeader[1], colorTableHeader[2])
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 8)

	for i, header := range headers {
		pdf.CellFormat(colWidths[i], 7, header, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Table rows
	pdf.SetFont("Arial", "", 8)
	fill := false

	for _, backup := range data.Backups {
		if fill {
			pdf.SetFillColor(colorTableAlt[0], colorTableAlt[1], colorTableAlt[2])
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		pdf.CellFormat(colWidths[0], 6, backup.Type, "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[1], 6, backup.Storage, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(colWidths[2], 6, backup.Timestamp.Format("2006-01-02 15:04"), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[3], 6, formatBytes(float64(backup.Size)), "1", 0, "R", fill, 0, "")

		// Protected
		if backup.Protected {
			pdf.SetTextColor(colorAccent[0], colorAccent[1], colorAccent[2])
			pdf.CellFormat(colWidths[4], 6, "Yes", "1", 0, "C", fill, 0, "")
		} else {
			pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
			pdf.CellFormat(colWidths[4], 6, "No", "1", 0, "C", fill, 0, "")
		}
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

		pdf.Ln(-1)
		fill = !fill
	}

	pdf.Ln(10)
}

// formatUptime converts seconds to human-readable uptime
func formatUptime(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	mins := (seconds % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

// addPageNumbers adds page numbers to all pages except the first (cover).
func (g *PDFGenerator) addPageNumbers(pdf *fpdf.Fpdf) {
	// Disable auto page break while adding footers to prevent creating new pages
	pdf.SetAutoPageBreak(false, 0)

	totalPages := pdf.PageCount()

	// Iterate through pages 2 to totalPages (skip cover page)
	for i := 2; i <= totalPages; i++ {
		pdf.SetPage(i)
		pageWidth, pageHeight := pdf.GetPageSize()

		pdf.SetY(pageHeight - 15)
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])

		pageNum := i - 1
		totalContent := totalPages - 1
		pdf.CellFormat(0, 5, fmt.Sprintf("Page %d of %d", pageNum, totalContent), "", 0, "C", false, 0, "")

		// Bottom line
		pdf.SetDrawColor(colorGridLine[0], colorGridLine[1], colorGridLine[2])
		pdf.SetLineWidth(0.3)
		pdf.Line(20, pageHeight-20, pageWidth-20, pageHeight-20)
	}
}

// GenerateMulti creates a multi-resource PDF report from the provided data.
func (g *PDFGenerator) GenerateMulti(data *MultiReportData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.SetAutoPageBreak(true, 25)

	// Page 1: Cover page
	g.writeMultiCoverPage(pdf, data)

	// Page 2: Fleet summary
	pdf.AddPage()
	g.addMultiPageHeader(pdf, data, "Fleet Summary")
	g.writeFleetSummary(pdf, data)

	// Pages 3+: Condensed per-resource pages
	for _, rd := range data.Resources {
		pdf.AddPage()
		g.addMultiPageHeader(pdf, data, "Resource Detail")
		g.writeCondensedResourcePage(pdf, rd)
	}

	// Add page numbers to all pages except cover
	g.addMultiPageNumbers(pdf)

	// Output to buffer
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("PDF output error: %w", err)
	}

	return buf.Bytes(), nil
}

// writeMultiCoverPage creates a cover page for multi-resource reports.
func (g *PDFGenerator) writeMultiCoverPage(pdf *fpdf.Fpdf, data *MultiReportData) {
	pdf.AddPage()

	pageWidth, pageHeight := pdf.GetPageSize()

	// Top accent bar
	pdf.SetFillColor(colorPrimary[0], colorPrimary[1], colorPrimary[2])
	pdf.Rect(0, 0, pageWidth, 8, "F")

	// Pulse branding area
	pdf.SetY(50)
	pdf.SetFont("Arial", "B", 32)
	pdf.SetTextColor(colorPrimary[0], colorPrimary[1], colorPrimary[2])
	pdf.CellFormat(0, 15, "PULSE", "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "", 12)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 8, "Infrastructure Monitoring", "", 1, "C", false, 0, "")

	// Main title
	pdf.SetY(100)
	pdf.SetFont("Arial", "B", 28)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 12, data.Title, "", 1, "C", false, 0, "")

	// Subtitle with counts
	pdf.SetY(120)
	pdf.SetFont("Arial", "", 14)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])

	// Calculate duration
	duration := data.End.Sub(data.Start)
	durationStr := formatDuration(duration)
	subtitle := fmt.Sprintf("%d Resources | %s", len(data.Resources), durationStr)
	pdf.CellFormat(0, 8, subtitle, "", 1, "C", false, 0, "")

	// Scope box
	pdf.SetY(140)
	boxX := 40.0
	boxWidth := pageWidth - 80
	boxHeight := 40.0

	pdf.SetFillColor(colorBackground[0], colorBackground[1], colorBackground[2])
	pdf.SetDrawColor(colorGridLine[0], colorGridLine[1], colorGridLine[2])
	pdf.RoundedRect(boxX, pdf.GetY(), boxWidth, boxHeight, 3, "1234", "FD")

	// Count by type
	nodeCount, vmCount, ctCount := 0, 0, 0
	for _, rd := range data.Resources {
		switch rd.ResourceType {
		case "node":
			nodeCount++
		case "vm":
			vmCount++
		case "container":
			ctCount++
		}
	}

	pdf.SetY(pdf.GetY() + 10)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 7, "SCOPE", "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "", 12)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

	var scopeParts []string
	if nodeCount > 0 {
		word := "Nodes"
		if nodeCount == 1 {
			word = "Node"
		}
		scopeParts = append(scopeParts, fmt.Sprintf("%d %s", nodeCount, word))
	}
	if vmCount > 0 {
		word := "VMs"
		if vmCount == 1 {
			word = "VM"
		}
		scopeParts = append(scopeParts, fmt.Sprintf("%d %s", vmCount, word))
	}
	if ctCount > 0 {
		word := "Containers"
		if ctCount == 1 {
			word = "Container"
		}
		scopeParts = append(scopeParts, fmt.Sprintf("%d %s", ctCount, word))
	}

	scopeStr := ""
	for i, part := range scopeParts {
		if i > 0 {
			scopeStr += ", "
		}
		scopeStr += part
	}
	pdf.CellFormat(0, 8, scopeStr, "", 1, "C", false, 0, "")

	// Time period
	pdf.SetY(200)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 7, "REPORTING PERIOD", "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "", 12)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	periodStr := fmt.Sprintf("%s  -  %s",
		data.Start.Format("January 2, 2006 15:04"),
		data.End.Format("January 2, 2006 15:04"))
	pdf.CellFormat(0, 8, periodStr, "", 1, "C", false, 0, "")

	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 6, fmt.Sprintf("(%s)", durationStr), "", 1, "C", false, 0, "")

	// Bottom section
	pdf.SetY(pageHeight - 50)
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 6, fmt.Sprintf("Generated: %s", data.GeneratedAt.Format("January 2, 2006 at 15:04 MST")), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Total Data Points: %d", data.TotalPoints), "", 1, "C", false, 0, "")

	// Bottom accent bar
	pdf.SetFillColor(colorPrimary[0], colorPrimary[1], colorPrimary[2])
	pdf.Rect(0, pageHeight-8, pageWidth, 8, "F")
}

// addMultiPageHeader adds a consistent header to multi-report content pages.
func (g *PDFGenerator) addMultiPageHeader(pdf *fpdf.Fpdf, data *MultiReportData, section string) {
	pageWidth, _ := pdf.GetPageSize()

	// Top line
	pdf.SetDrawColor(colorPrimary[0], colorPrimary[1], colorPrimary[2])
	pdf.SetLineWidth(0.5)
	pdf.Line(20, 15, pageWidth-20, 15)

	// Header text
	pdf.SetY(18)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(colorPrimary[0], colorPrimary[1], colorPrimary[2])
	pdf.CellFormat(0, 5, "PULSE FLEET REPORT", "", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
	pdf.CellFormat(0, 5, fmt.Sprintf("%d Resources", len(data.Resources)), "", 1, "R", false, 0, "")

	// Section title
	pdf.SetY(30)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 10, section, "", 1, "L", false, 0, "")

	pdf.Ln(5)
}

// writeFleetSummary writes the fleet summary table and observations.
func (g *PDFGenerator) writeFleetSummary(pdf *fpdf.Fpdf, data *MultiReportData) {
	pageWidth, _ := pdf.GetPageSize()

	// Determine aggregate health
	healthStatus := "HEALTHY"
	healthColor := colorAccent
	healthMessage := "All systems operating normally"

	totalActive := 0
	totalCritical := 0
	totalWarning := 0
	for _, rd := range data.Resources {
		for _, alert := range rd.Alerts {
			if alert.ResolvedTime == nil {
				totalActive++
				if alert.Level == "critical" {
					totalCritical++
				} else {
					totalWarning++
				}
			}
		}
	}

	if totalCritical > 0 {
		healthStatus = "CRITICAL"
		healthColor = colorDanger
		healthMessage = fmt.Sprintf("%d critical issues across fleet", totalCritical)
	} else if totalWarning > 0 {
		healthStatus = "WARNING"
		healthColor = colorWarning
		healthMessage = fmt.Sprintf("%d warnings across fleet", totalWarning)
	}

	// Health status card
	cardX := 20.0
	cardWidth := pageWidth - 40
	cardHeight := 30.0

	pdf.SetFillColor(healthColor[0], healthColor[1], healthColor[2])
	pdf.RoundedRect(cardX, pdf.GetY(), cardWidth, cardHeight, 3, "1234", "F")

	pdf.SetXY(cardX, pdf.GetY()+6)
	pdf.SetFont("Arial", "B", 20)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(cardWidth, 10, healthStatus, "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(cardWidth, 7, healthMessage, "", 1, "C", false, 0, "")

	pdf.SetY(pdf.GetY() + 12)

	// Summary table
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 8, "Resource Summary", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Table header
	colWidths := []float64{40, 25, 20, 23, 23, 23, 16}
	headers := []string{"Resource", "Type", "Status", "Avg CPU", "Avg Mem", "Avg Disk", "Alerts"}

	pdf.SetFillColor(colorTableHeader[0], colorTableHeader[1], colorTableHeader[2])
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 8)

	for i, header := range headers {
		pdf.CellFormat(colWidths[i], 7, header, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Table rows
	pdf.SetFont("Arial", "", 8)
	fill := false

	// Track highest values for observations
	var highestCPUName string
	var highestCPUVal float64
	var mostAlertsName string
	var mostAlertsCount int

	for _, rd := range data.Resources {
		if fill {
			pdf.SetFillColor(colorTableAlt[0], colorTableAlt[1], colorTableAlt[2])
		} else {
			pdf.SetFillColor(255, 255, 255)
		}

		// Resource name
		resourceName := rd.ResourceID
		if rd.Resource != nil && rd.Resource.Name != "" {
			resourceName = rd.Resource.Name
		}
		if len(resourceName) > 25 {
			resourceName = resourceName[:22] + "..."
		}
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(colWidths[0], 6, resourceName, "1", 0, "L", fill, 0, "")

		// Type
		pdf.CellFormat(colWidths[1], 6, GetResourceTypeDisplayName(rd.ResourceType), "1", 0, "C", fill, 0, "")

		// Status
		status := "N/A"
		if rd.Resource != nil {
			status = rd.Resource.Status
		}
		if status == "online" || status == "running" {
			pdf.SetTextColor(colorAccent[0], colorAccent[1], colorAccent[2])
		} else if status == "stopped" || status == "offline" {
			pdf.SetTextColor(colorDanger[0], colorDanger[1], colorDanger[2])
		} else {
			pdf.SetTextColor(colorWarning[0], colorWarning[1], colorWarning[2])
		}
		pdf.CellFormat(colWidths[2], 6, status, "1", 0, "C", fill, 0, "")

		// Avg CPU
		var avgCPU float64
		if stats, ok := rd.Summary.ByMetric["cpu"]; ok {
			avgCPU = stats.Avg
		}
		pdf.SetTextColor(getStatColor(avgCPU)[0], getStatColor(avgCPU)[1], getStatColor(avgCPU)[2])
		pdf.CellFormat(colWidths[3], 6, fmt.Sprintf("%.1f%%", avgCPU), "1", 0, "C", fill, 0, "")

		if avgCPU > highestCPUVal {
			highestCPUVal = avgCPU
			if rd.Resource != nil && rd.Resource.Name != "" {
				highestCPUName = rd.Resource.Name
			} else {
				highestCPUName = rd.ResourceID
			}
		}

		// Avg Memory
		var avgMem float64
		if stats, ok := rd.Summary.ByMetric["memory"]; ok {
			avgMem = stats.Avg
		}
		pdf.SetTextColor(getStatColor(avgMem)[0], getStatColor(avgMem)[1], getStatColor(avgMem)[2])
		pdf.CellFormat(colWidths[4], 6, fmt.Sprintf("%.1f%%", avgMem), "1", 0, "C", fill, 0, "")

		// Avg Disk
		var avgDisk float64
		if stats, ok := rd.Summary.ByMetric["disk"]; ok {
			avgDisk = stats.Avg
		} else if stats, ok := rd.Summary.ByMetric["usage"]; ok {
			avgDisk = stats.Avg
		}
		pdf.SetTextColor(getStatColor(avgDisk)[0], getStatColor(avgDisk)[1], getStatColor(avgDisk)[2])
		pdf.CellFormat(colWidths[5], 6, fmt.Sprintf("%.1f%%", avgDisk), "1", 0, "C", fill, 0, "")

		// Alerts count
		alertCount := 0
		for _, alert := range rd.Alerts {
			if alert.ResolvedTime == nil {
				alertCount++
			}
		}
		pdf.SetTextColor(getAlertCountColor(alertCount)[0], getAlertCountColor(alertCount)[1], getAlertCountColor(alertCount)[2])
		pdf.CellFormat(colWidths[6], 6, fmt.Sprintf("%d", alertCount), "1", 0, "C", fill, 0, "")

		if alertCount > mostAlertsCount {
			mostAlertsCount = alertCount
			if rd.Resource != nil && rd.Resource.Name != "" {
				mostAlertsName = rd.Resource.Name
			} else {
				mostAlertsName = rd.ResourceID
			}
		}

		pdf.Ln(-1)
		fill = !fill
	}

	pdf.Ln(8)

	// Fleet Observations
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	pdf.CellFormat(0, 8, "Fleet Observations", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])

	if highestCPUName != "" {
		pdf.SetFillColor(colorSecondary[0], colorSecondary[1], colorSecondary[2])
		pdf.Circle(pdf.GetX()+3, pdf.GetY()+3, 2, "F")
		pdf.SetX(pdf.GetX() + 8)
		pdf.CellFormat(0, 6, fmt.Sprintf("Highest CPU: %s (avg %.1f%%)", highestCPUName, highestCPUVal), "", 1, "L", false, 0, "")
		pdf.Ln(1)
	}

	if mostAlertsCount > 0 && mostAlertsName != "" {
		pdf.SetFillColor(colorDanger[0], colorDanger[1], colorDanger[2])
		pdf.Circle(pdf.GetX()+3, pdf.GetY()+3, 2, "F")
		pdf.SetX(pdf.GetX() + 8)
		pdf.CellFormat(0, 6, fmt.Sprintf("Most alerts: %s (%d active)", mostAlertsName, mostAlertsCount), "", 1, "L", false, 0, "")
		pdf.Ln(1)
	}

	if totalActive == 0 {
		pdf.SetFillColor(colorAccent[0], colorAccent[1], colorAccent[2])
		pdf.Circle(pdf.GetX()+3, pdf.GetY()+3, 2, "F")
		pdf.SetX(pdf.GetX() + 8)
		pdf.CellFormat(0, 6, "No active alerts across the fleet", "", 1, "L", false, 0, "")
	}
}

// writeCondensedResourcePage writes a condensed single-page view for one resource.
func (g *PDFGenerator) writeCondensedResourcePage(pdf *fpdf.Fpdf, rd *ReportData) {
	// Resource header
	resourceName := rd.ResourceID
	if rd.Resource != nil && rd.Resource.Name != "" {
		resourceName = rd.Resource.Name
	}

	// Name - measure width while font is still set to bold
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
	nameWidth := pdf.GetStringWidth(resourceName)
	pdf.CellFormat(nameWidth+2, 8, resourceName, "", 0, "L", false, 0, "")

	// Type/status/uptime inline after the name
	typeDisplay := GetResourceTypeDisplayName(rd.ResourceType)
	status := "unknown"
	if rd.Resource != nil {
		status = rd.Resource.Status
	}

	pdf.SetFont("Arial", "", 10)
	if status == "online" || status == "running" {
		pdf.SetTextColor(colorAccent[0], colorAccent[1], colorAccent[2])
	} else if status == "stopped" || status == "offline" {
		pdf.SetTextColor(colorDanger[0], colorDanger[1], colorDanger[2])
	} else {
		pdf.SetTextColor(colorWarning[0], colorWarning[1], colorWarning[2])
	}
	statusStr := fmt.Sprintf("  |  %s  |  %s", typeDisplay, status)

	// Uptime
	if rd.Resource != nil && rd.Resource.Uptime > 0 {
		statusStr += fmt.Sprintf("  |  Uptime: %s", formatUptime(rd.Resource.Uptime))
	}
	pdf.CellFormat(0, 8, statusStr, "", 1, "L", false, 0, "")
	pdf.Ln(3)

	// Stats bar - CPU, Memory, Disk averages and maxes
	pdf.SetFillColor(colorBackground[0], colorBackground[1], colorBackground[2])
	pdf.SetDrawColor(colorGridLine[0], colorGridLine[1], colorGridLine[2])
	barY := pdf.GetY()
	barWidth := 170.0
	barHeight := 22.0
	pdf.RoundedRect(20, barY, barWidth, barHeight, 2, "1234", "FD")

	colW := barWidth / 3.0

	// CPU stats
	var avgCPU, maxCPU, avgMem, maxMem, avgDisk, maxDisk float64
	if stats, ok := rd.Summary.ByMetric["cpu"]; ok {
		avgCPU = stats.Avg
		maxCPU = stats.Max
	}
	if stats, ok := rd.Summary.ByMetric["memory"]; ok {
		avgMem = stats.Avg
		maxMem = stats.Max
	}
	if stats, ok := rd.Summary.ByMetric["disk"]; ok {
		avgDisk = stats.Avg
		maxDisk = stats.Max
	} else if stats, ok := rd.Summary.ByMetric["usage"]; ok {
		avgDisk = stats.Avg
		maxDisk = stats.Max
	}

	// CPU column
	pdf.SetXY(20+2, barY+3)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(colorSecondary[0], colorSecondary[1], colorSecondary[2])
	pdf.CellFormat(colW-4, 5, "CPU", "", 0, "C", false, 0, "")

	pdf.SetXY(20+2, barY+9)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(getStatColor(avgCPU)[0], getStatColor(avgCPU)[1], getStatColor(avgCPU)[2])
	pdf.CellFormat(colW-4, 5, fmt.Sprintf("avg %.1f%% / max %.1f%%", avgCPU, maxCPU), "", 0, "C", false, 0, "")

	// Memory column
	pdf.SetXY(20+colW+2, barY+3)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor([3]int{155, 89, 182}[0], [3]int{155, 89, 182}[1], [3]int{155, 89, 182}[2])
	pdf.CellFormat(colW-4, 5, "Memory", "", 0, "C", false, 0, "")

	pdf.SetXY(20+colW+2, barY+9)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(getStatColor(avgMem)[0], getStatColor(avgMem)[1], getStatColor(avgMem)[2])
	pdf.CellFormat(colW-4, 5, fmt.Sprintf("avg %.1f%% / max %.1f%%", avgMem, maxMem), "", 0, "C", false, 0, "")

	// Disk column
	pdf.SetXY(20+2*colW+2, barY+3)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(colorAccent[0], colorAccent[1], colorAccent[2])
	pdf.CellFormat(colW-4, 5, "Disk", "", 0, "C", false, 0, "")

	pdf.SetXY(20+2*colW+2, barY+9)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(getStatColor(avgDisk)[0], getStatColor(avgDisk)[1], getStatColor(avgDisk)[2])
	pdf.CellFormat(colW-4, 5, fmt.Sprintf("avg %.1f%% / max %.1f%%", avgDisk, maxDisk), "", 0, "C", false, 0, "")

	pdf.SetY(barY + barHeight + 5)

	// Small chart: CPU + Memory overlaid (if we have data)
	cpuPoints := rd.Metrics["cpu"]
	memPoints := rd.Metrics["memory"]
	if len(cpuPoints) >= 2 || len(memPoints) >= 2 {
		chartHeight := 40.0
		chartWidth := 170.0
		chartX := 20.0
		chartY := pdf.GetY()

		// Use CPU data primarily, or memory if no CPU
		primaryPoints := cpuPoints
		if len(primaryPoints) < 2 {
			primaryPoints = memPoints
		}

		if len(primaryPoints) >= 2 {
			// Chart title
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
			pdf.CellFormat(0, 5, "Performance Overview", "", 1, "L", false, 0, "")
			chartY = pdf.GetY()

			g.drawChart(pdf, primaryPoints, chartX, chartY, chartWidth, chartHeight, "cpu")

			// If we have both CPU and memory, overlay memory
			if len(cpuPoints) >= 2 && len(memPoints) >= 2 {
				g.drawChartOverlay(pdf, memPoints, cpuPoints, chartX, chartY, chartWidth, chartHeight)
			}

			// Legend (below chart X-axis labels which are at chartY + chartHeight + 1..+5)
			pdf.SetY(chartY + chartHeight + 7)
			pdf.SetFont("Arial", "", 7)
			if len(cpuPoints) >= 2 {
				pdf.SetTextColor(colorSecondary[0], colorSecondary[1], colorSecondary[2])
				pdf.CellFormat(30, 4, "--- CPU", "", 0, "L", false, 0, "")
			}
			if len(memPoints) >= 2 {
				pdf.SetTextColor(155, 89, 182)
				pdf.CellFormat(30, 4, "--- Memory", "", 0, "L", false, 0, "")
			}
			pdf.Ln(6)
		}
	}

	// Active alerts (up to 3)
	activeAlerts := make([]AlertInfo, 0)
	for _, alert := range rd.Alerts {
		if alert.ResolvedTime == nil {
			activeAlerts = append(activeAlerts, alert)
		}
	}

	if len(activeAlerts) > 0 {
		pdf.Ln(3)
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(0, 6, "Active Alerts", "", 1, "L", false, 0, "")
		pdf.Ln(1)

		pdf.SetFont("Arial", "", 9)
		maxAlerts := 3
		if len(activeAlerts) < maxAlerts {
			maxAlerts = len(activeAlerts)
		}
		for i := 0; i < maxAlerts; i++ {
			alert := activeAlerts[i]
			if alert.Level == "critical" {
				pdf.SetTextColor(colorDanger[0], colorDanger[1], colorDanger[2])
			} else {
				pdf.SetTextColor(colorWarning[0], colorWarning[1], colorWarning[2])
			}
			pdf.CellFormat(6, 5, "!", "", 0, "C", false, 0, "")
			pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
			msg := alert.Message
			if len(msg) > 80 {
				msg = msg[:77] + "..."
			}
			pdf.CellFormat(0, 5, msg, "", 1, "L", false, 0, "")
		}
		if len(activeAlerts) > 3 {
			pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])
			pdf.CellFormat(0, 5, fmt.Sprintf("... and %d more", len(activeAlerts)-3), "", 1, "L", false, 0, "")
		}
	}

	// Storage summary (nodes) or backup summary (VMs/containers)
	if len(rd.Storage) > 0 {
		pdf.Ln(3)
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(0, 6, "Storage Pools", "", 1, "L", false, 0, "")
		pdf.Ln(1)

		pdf.SetFont("Arial", "", 9)
		for _, s := range rd.Storage {
			pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
			line := fmt.Sprintf("%s (%s): %s / %s (%.1f%%)",
				s.Name, s.Type,
				formatBytes(float64(s.Used)),
				formatBytes(float64(s.Total)),
				s.UsagePerc)
			pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
		}
	}

	if len(rd.Backups) > 0 {
		pdf.Ln(3)
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(0, 6, "Backups", "", 1, "L", false, 0, "")
		pdf.Ln(1)

		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(colorTextDark[0], colorTextDark[1], colorTextDark[2])
		pdf.CellFormat(0, 5, fmt.Sprintf("%d backups available", len(rd.Backups)), "", 1, "L", false, 0, "")
		if len(rd.Backups) > 0 {
			latest := rd.Backups[0]
			for _, b := range rd.Backups {
				if b.Timestamp.After(latest.Timestamp) {
					latest = b
				}
			}
			pdf.CellFormat(0, 5, fmt.Sprintf("Latest: %s (%s)", latest.Timestamp.Format("2006-01-02 15:04"), formatBytes(float64(latest.Size))), "", 1, "L", false, 0, "")
		}
	}
}

// drawChartOverlay draws a secondary line on an existing chart using memory data over CPU scale.
func (g *PDFGenerator) drawChartOverlay(pdf *fpdf.Fpdf, overlayPoints []MetricDataPoint, primaryPoints []MetricDataPoint, x, y, width, height float64) {
	if len(overlayPoints) < 2 || len(primaryPoints) < 2 {
		return
	}

	// Use the same time scale as the primary chart
	startTime := primaryPoints[0].Timestamp.Unix()
	endTime := primaryPoints[len(primaryPoints)-1].Timestamp.Unix()
	timeRange := float64(endTime - startTime)
	if timeRange == 0 {
		timeRange = 1
	}

	// Use the primary chart's value range for consistent scaling
	minVal, maxVal := primaryPoints[0].Value, primaryPoints[0].Value
	for _, p := range primaryPoints {
		if p.Value < minVal {
			minVal = p.Value
		}
		if p.Value > maxVal {
			maxVal = p.Value
		}
	}
	// Also include overlay points in the range
	for _, p := range overlayPoints {
		if p.Value < minVal {
			minVal = p.Value
		}
		if p.Value > maxVal {
			maxVal = p.Value
		}
	}

	valRange := maxVal - minVal
	if valRange < 1 {
		valRange = 10
	}
	minVal = math.Max(0, minVal-valRange*0.1)
	maxVal = maxVal + valRange*0.1

	// Draw the overlay line in purple (memory color)
	memColor := [3]int{155, 89, 182}
	pdf.SetDrawColor(memColor[0], memColor[1], memColor[2])
	pdf.SetLineWidth(0.6)

	prevX, prevY := 0.0, 0.0
	for i, p := range overlayPoints {
		xPos := x + 2 + (float64(p.Timestamp.Unix()-startTime)/timeRange)*(width-4)
		yPos := y + height - 2 - ((p.Value-minVal)/(maxVal-minVal))*(height-4)
		yPos = math.Max(y+2, math.Min(y+height-2, yPos))

		if i > 0 {
			pdf.Line(prevX, prevY, xPos, yPos)
		}
		prevX, prevY = xPos, yPos
	}
}

// addMultiPageNumbers adds page numbers to all pages except the first (cover).
func (g *PDFGenerator) addMultiPageNumbers(pdf *fpdf.Fpdf) {
	pdf.SetAutoPageBreak(false, 0)

	totalPages := pdf.PageCount()

	for i := 2; i <= totalPages; i++ {
		pdf.SetPage(i)
		pageWidth, pageHeight := pdf.GetPageSize()

		pdf.SetY(pageHeight - 15)
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(colorTextMuted[0], colorTextMuted[1], colorTextMuted[2])

		pageNum := i - 1
		totalContent := totalPages - 1
		pdf.CellFormat(0, 5, fmt.Sprintf("Page %d of %d", pageNum, totalContent), "", 0, "C", false, 0, "")

		// Bottom line
		pdf.SetDrawColor(colorGridLine[0], colorGridLine[1], colorGridLine[2])
		pdf.SetLineWidth(0.3)
		pdf.Line(20, pageHeight-20, pageWidth-20, pageHeight-20)
	}
}

// getMetricColor returns a color for a metric type.
func getMetricColor(metricType string) [3]int {
	switch metricType {
	case "cpu":
		return colorSecondary // Blue
	case "memory":
		return [3]int{155, 89, 182} // Purple
	case "disk", "usage":
		return colorAccent // Green
	default:
		return colorSecondary
	}
}

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours >= 24 {
		days := hours / 24
		remainingHours := hours % 24
		dayWord := "days"
		if days == 1 {
			dayWord = "day"
		}
		if remainingHours > 0 {
			hourWord := "hours"
			if remainingHours == 1 {
				hourWord = "hour"
			}
			return fmt.Sprintf("%d %s, %d %s", days, dayWord, remainingHours, hourWord)
		}
		return fmt.Sprintf("%d %s", days, dayWord)
	}
	if hours > 0 {
		minutes := int(d.Minutes()) % 60
		hourWord := "hours"
		if hours == 1 {
			hourWord = "hour"
		}
		if minutes > 0 {
			minWord := "minutes"
			if minutes == 1 {
				minWord = "minute"
			}
			return fmt.Sprintf("%d %s, %d %s", hours, hourWord, minutes, minWord)
		}
		return fmt.Sprintf("%d %s", hours, hourWord)
	}
	minutes := int(d.Minutes())
	minWord := "minutes"
	if minutes == 1 {
		minWord = "minute"
	}
	return fmt.Sprintf("%d %s", minutes, minWord)
}
