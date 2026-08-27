package notifications

import (
	"fmt"
	"html"
	"strings"
	"time"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// titleCase capitalizes the first letter of each word (simple ASCII-safe version)
func titleCase(s string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range s {
		if unicode.IsSpace(r) {
			capitalizeNext = true
			result.WriteRune(r)
		} else if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(unicode.ToLower(r))
		}
	}
	return result.String()
}

// alertNodeDisplay returns the display name for an alert's node, falling back
// to the raw node name if no display name is set.
func alertNodeDisplay(alert *alerts.Alert) string {
	if alert.NodeDisplayName != "" {
		return alert.NodeDisplayName
	}
	return alert.Node
}

func isPatrolFindingAlert(alert *alerts.Alert) bool {
	return alert != nil && strings.EqualFold(strings.TrimSpace(alert.Type), "patrol_finding")
}

func alertTypeDisplay(alertType string) string {
	switch strings.ToLower(strings.TrimSpace(alertType)) {
	case "cpu":
		return "CPU"
	case "memory":
		return "Memory"
	case "disk":
		return "Disk"
	case "io":
		return "I/O"
	case "patrol_finding":
		return "Patrol Finding"
	default:
		return titleCase(alertType)
	}
}

func patrolFindingCategory(alert *alerts.Alert) string {
	if alert == nil || alert.Metadata == nil {
		return ""
	}
	category, _ := alert.Metadata["category"].(string)
	return titleCase(strings.TrimSpace(category))
}

// EmailTemplate generates a professional HTML email template for alerts
func EmailTemplate(alertList []*alerts.Alert, isSingle bool) (subject, htmlBody, textBody string) {
	if isSingle && len(alertList) == 1 {
		return singleAlertTemplate(alertList[0])
	}
	return groupedAlertTemplate(alertList)
}

func singleAlertTemplate(alert *alerts.Alert) (subject, htmlBody, textBody string) {
	level := alerts.NormalizeAlertLevel(alert.Level)
	levelColor := "#3b82f6"
	levelBg := "#eff6ff"
	switch level {
	case alerts.AlertLevelCritical:
		levelColor = "#ff6b6b"
		levelBg = "#fee"
	case alerts.AlertLevelWarning:
		levelColor = "#ffd93d"
		levelBg = "#fffaeb"
	}

	alertType := alertTypeDisplay(alert.Type)

	subject = fmt.Sprintf("[Pulse Alert] %s: %s on %s",
		titleCase(string(level)), alertType, alert.ResourceName)

	escapedLevel := html.EscapeString(string(level))
	escapedResourceName := html.EscapeString(alert.ResourceName)
	escapedMessage := html.EscapeString(alert.Message)
	escapedCurrentValue := html.EscapeString(formatMetricValue(alert.Type, alert.Value))
	escapedThresholdValue := html.EscapeString(formatMetricThreshold(alert.Type, alert.Threshold))
	escapedResourceID := html.EscapeString(alert.ResourceID)
	escapedAlertType := html.EscapeString(alertType)
	escapedNode := html.EscapeString(alertNodeDisplay(alert))
	escapedInstance := html.EscapeString(alert.Instance)
	escapedStarted := html.EscapeString(formatAlertStartTime(alert.StartTime))
	escapedDuration := html.EscapeString(formatDuration(time.Since(alert.StartTime)))

	metricsHTML := fmt.Sprintf(`
            <div class="metrics">
                <div class="metric">
                    <div class="metric-label">Current Value</div>
                    <div class="metric-value">%s</div>
                </div>
                <div class="metric">
                    <div class="metric-label">Threshold</div>
                    <div class="metric-value">%s</div>
                </div>
            </div>`,
		escapedCurrentValue,
		escapedThresholdValue,
	)
	if isPatrolFindingAlert(alert) {
		metricsHTML = ""
	}

	htmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; background: #f5f5f5; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 20px auto; background: #fff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .header { background: #1a1a1a; color: #fff; padding: 20px; text-align: center; }
        .header h1 { margin: 0; font-size: 24px; font-weight: 500; }
        .pulse-logo { width: 40px; height: 40px; margin: 0 auto 10px; }
        .content { padding: 30px; }
        .alert-box { background: %s; border-left: 4px solid %s; padding: 20px; margin: 20px 0; border-radius: 4px; }
        .alert-level { color: %s; font-weight: bold; text-transform: uppercase; font-size: 14px; }
        .alert-resource { font-size: 18px; font-weight: 500; margin: 10px 0; color: #1a1a1a; }
        .metrics { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin: 20px 0; }
        .metric { background: #f8f9fa; padding: 15px; border-radius: 4px; }
        .metric-label { color: #666; font-size: 12px; text-transform: uppercase; letter-spacing: 0.5px; }
        .metric-value { font-size: 24px; font-weight: 500; color: #1a1a1a; margin-top: 5px; }
        .details { background: #f8f9fa; padding: 20px; border-radius: 4px; margin: 20px 0; }
        .detail-row { display: flex; justify-content: space-between; align-items: center; margin: 10px 0; padding-bottom: 10px; border-bottom: 1px solid #e9ecef; gap: 20px; }
        .detail-row:last-child { border-bottom: none; padding-bottom: 0; }
        .detail-label { color: #666; min-width: 120px; }
        .detail-value { font-weight: 500; color: #1a1a1a; text-align: right; flex: 1; }
        .footer { background: #f8f9fa; padding: 20px; text-align: center; color: #666; font-size: 12px; }
        .footer a { color: #0066cc; text-decoration: none; }
        @media (max-width: 600px) {
            .metrics { grid-template-columns: 1fr; }
            .container { margin: 0; border-radius: 0; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <svg class="pulse-logo" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
                <path d="M10 50 L30 50 L35 30 L40 70 L45 10 L50 90 L55 30 L60 70 L65 50 L90 50" 
                      stroke="#4ade80" stroke-width="3" fill="none"/>
            </svg>
            <h1>Pulse Monitoring Alert</h1>
        </div>
        <div class="content">
            <div class="alert-box">
                <div class="alert-level">%s Alert</div>
                <div class="alert-resource">%s</div>
                <div>%s</div>
            </div>
            %s
            
            <div class="details">
                <div class="detail-row">
                    <span class="detail-label">Resource ID</span>
                    <span class="detail-value">%s</span>
                </div>
                <div class="detail-row">
                    <span class="detail-label">Alert Type</span>
                    <span class="detail-value">%s</span>
                </div>
                <div class="detail-row">
                    <span class="detail-label">Node</span>
                    <span class="detail-value">%s</span>
                </div>
                <div class="detail-row">
                    <span class="detail-label">Instance</span>
                    <span class="detail-value">%s</span>
                </div>
                <div class="detail-row">
                    <span class="detail-label">Started</span>
                    <span class="detail-value">%s</span>
                </div>
                <div class="detail-row">
                    <span class="detail-label">Duration</span>
                    <span class="detail-value">%s</span>
                </div>
            </div>
        </div>
        <div class="footer">
            <p>This is an automated notification from Pulse Monitoring</p>
            <p>View alerts and configure settings in your Pulse dashboard</p>
        </div>
    </div>
</body>
</html>`,
		levelBg, levelColor, levelColor,
		escapedLevel,
		escapedResourceName,
		escapedMessage,
		metricsHTML,
		escapedResourceID,
		escapedAlertType,
		escapedNode,
		escapedInstance,
		escapedStarted,
		escapedDuration,
	)

	if isPatrolFindingAlert(alert) {
		textBody = fmt.Sprintf(`PULSE MONITORING ALERT

%s ALERT: %s

Resource: %s (%s)
Type: %s
Message: %s

Details:
- Node: %s
- Instance: %s
- Started: %s
- Duration: %s

This is an automated notification from Pulse Monitoring.
View alerts and configure settings in your Pulse dashboard.`,
			strings.ToUpper(string(level)),
			alert.ResourceName,
			alert.ResourceName,
			alert.ResourceID,
			alertType,
			alert.Message,
			alertNodeDisplay(alert),
			alert.Instance,
			formatAlertStartTime(alert.StartTime),
			formatDuration(time.Since(alert.StartTime)),
		)
		return subject, htmlBody, textBody
	}

	textBody = fmt.Sprintf(`PULSE MONITORING ALERT

%s ALERT: %s

Resource: %s (%s)
Type: %s
Current Value: %s (Threshold: %s)
Message: %s

Details:
- Node: %s  
- Instance: %s
- Started: %s
- Duration: %s

This is an automated notification from Pulse Monitoring.
View alerts and configure settings in your Pulse dashboard.`,
		strings.ToUpper(string(level)),
		alert.ResourceName,
		alert.ResourceName,
		alert.ResourceID,
		alert.Type,
		formatMetricValue(alert.Type, alert.Value),
		formatMetricThreshold(alert.Type, alert.Threshold),
		alert.Message,
		alertNodeDisplay(alert),
		alert.Instance,
		formatAlertStartTime(alert.StartTime),
		formatDuration(time.Since(alert.StartTime)),
	)

	return subject, htmlBody, textBody
}

func groupedAlertTemplate(alertList []*alerts.Alert) (subject, htmlBody, textBody string) {
	critical := 0
	warning := 0
	info := 0
	patrolFindings := 0
	for _, alert := range alertList {
		switch alerts.NormalizeAlertLevel(alert.Level) {
		case alerts.AlertLevelCritical:
			critical++
		case alerts.AlertLevelWarning:
			warning++
		case alerts.AlertLevelInfo:
			info++
		}
		if isPatrolFindingAlert(alert) {
			patrolFindings++
		}
	}

	// Subject line preserves every severity represented in the group. Treating
	// informational alerts as warnings here would undo destination severity
	// routing at the final user-visible boundary.
	severityParts := make([]string, 0, 3)
	if critical > 0 {
		severityParts = append(severityParts, fmt.Sprintf("%d Critical", critical))
	}
	if warning > 0 {
		severityParts = append(severityParts, fmt.Sprintf("%d Warning", warning))
	}
	if info > 0 {
		severityParts = append(severityParts, fmt.Sprintf("%d Info", info))
	}
	subject = fmt.Sprintf("[Pulse Alert] %s alert%s", strings.Join(severityParts, ", "), pluralize(len(alertList)))

	// Build alert rows
	var alertRows strings.Builder
	for _, alert := range alertList {
		level := alerts.NormalizeAlertLevel(alert.Level)
		levelColor := "#3b82f6"
		switch level {
		case alerts.AlertLevelCritical:
			levelColor = "#ff6b6b"
		case alerts.AlertLevelWarning:
			levelColor = "#ffd93d"
		}

		escapedResourceName := html.EscapeString(alert.ResourceName)
		escapedType := html.EscapeString(alert.Type)
		escapedNode := html.EscapeString(alertNodeDisplay(alert))
		escapedLevel := html.EscapeString(string(level))
		escapedValue := html.EscapeString(formatMetricValue(alert.Type, alert.Value))
		escapedThreshold := html.EscapeString(formatMetricThreshold(alert.Type, alert.Threshold))
		escapedDuration := html.EscapeString(formatDuration(time.Since(alert.StartTime)))
		escapedDetail := escapedType + " on " + escapedNode
		value := fmt.Sprintf(`<div style="font-weight: 500;">%s</div>
                        <div style="font-size: 12px; color: #666;">of %s</div>`, escapedValue, escapedThreshold)
		if isPatrolFindingAlert(alert) {
			escapedDetail = html.EscapeString(alert.Message)
			category := html.EscapeString(patrolFindingCategory(alert))
			value = `<div style="font-weight: 500;">Patrol</div>`
			if category != "" {
				value += fmt.Sprintf(`
                        <div style="font-size: 12px; color: #666;">%s</div>`, category)
			}
		}

		alertRows.WriteString(fmt.Sprintf(`
                <tr>
                    <td style="padding: 12px; border-bottom: 1px solid #e9ecef;">
                        <div style="display: flex; align-items: center;">
                            <span style="display: inline-block; width: 8px; height: 8px; background: %s; border-radius: 50%%; margin-right: 10px;"></span>
                            <div>
                                <div style="font-weight: 500; color: #1a1a1a;">%s</div>
                                <div style="font-size: 12px; color: #666; margin-top: 2px;">%s</div>
                            </div>
                        </div>
                    </td>
                    <td style="padding: 12px; border-bottom: 1px solid #e9ecef; text-align: center;">
                        <span style="color: %s; font-weight: 500; text-transform: uppercase; font-size: 12px;">%s</span>
                    </td>
                    <td style="padding: 12px; border-bottom: 1px solid #e9ecef; text-align: right;">
                        %s
                    </td>
                    <td style="padding: 12px; border-bottom: 1px solid #e9ecef; text-align: right; color: #666; font-size: 12px;">
                        %s ago
                    </td>
                </tr>`,
			levelColor,
			escapedResourceName,
			escapedDetail,
			levelColor, escapedLevel,
			value,
			escapedDuration,
		))
	}

	valueHeading := "Value"
	if patrolFindings == len(alertList) {
		valueHeading = "Finding"
	} else if patrolFindings > 0 {
		valueHeading = "Value / Finding"
	}

	htmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; background: #f5f5f5; margin: 0; padding: 0; }
        .container { max-width: 800px; margin: 20px auto; background: #fff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .header { background: #1a1a1a; color: #fff; padding: 20px; text-align: center; }
        .header h1 { margin: 0; font-size: 24px; font-weight: 500; }
        .pulse-logo { width: 40px; height: 40px; margin: 0 auto 10px; }
        .content { padding: 30px; }
        .summary { background: #f8f9fa; padding: 20px; border-radius: 4px; margin-bottom: 30px; }
        .summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 20px; margin-top: 15px; }
        .summary-item { text-align: center; }
        .summary-count { font-size: 32px; font-weight: 500; }
        .critical-count { color: #ff6b6b; }
        .warning-count { color: #ffd93d; }
		.info-count { color: #3b82f6; }
        .summary-label { color: #666; font-size: 14px; margin-top: 5px; }
        .alerts-table { width: 100%%; margin-top: 20px; border-collapse: collapse; }
        .alerts-table th { text-align: left; padding: 12px; border-bottom: 2px solid #e9ecef; color: #666; font-weight: 500; font-size: 12px; text-transform: uppercase; letter-spacing: 0.5px; }
        .footer { background: #f8f9fa; padding: 20px; text-align: center; color: #666; font-size: 12px; }
        .footer a { color: #0066cc; text-decoration: none; }
        @media (max-width: 600px) {
            .container { margin: 0; border-radius: 0; }
            .alerts-table { font-size: 14px; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <svg class="pulse-logo" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
                <path d="M10 50 L30 50 L35 30 L40 70 L45 10 L50 90 L55 30 L60 70 L65 50 L90 50" 
                      stroke="#4ade80" stroke-width="3" fill="none"/>
            </svg>
            <h1>Pulse Monitoring Alert Summary</h1>
        </div>
        <div class="content">
            <div class="summary">
                <h2 style="margin: 0 0 15px 0; font-size: 18px;">%d New Alert%s</h2>
                <div class="summary-grid">`,
		len(alertList), pluralize(len(alertList)))

	if critical > 0 {
		htmlBody += fmt.Sprintf(`
                    <div class="summary-item">
                        <div class="summary-count critical-count">%d</div>
                        <div class="summary-label">Critical</div>
                    </div>`, critical)
	}

	if warning > 0 {
		htmlBody += fmt.Sprintf(`
                    <div class="summary-item">
                        <div class="summary-count warning-count">%d</div>
                        <div class="summary-label">Warning</div>
                    </div>`, warning)
	}

	if info > 0 {
		htmlBody += fmt.Sprintf(`
                    <div class="summary-item">
                        <div class="summary-count info-count">%d</div>
                        <div class="summary-label">Info</div>
                    </div>`, info)
	}

	htmlBody += fmt.Sprintf(`
                </div>
            </div>
            
            <table class="alerts-table">
                <thead>
                    <tr>
                        <th>Resource</th>
                        <th style="text-align: center;">Level</th>
                        <th style="text-align: right;">%s</th>
                        <th style="text-align: right;">Duration</th>
                    </tr>
                </thead>
                <tbody>%s
                </tbody>
            </table>
        </div>
        <div class="footer">
            <p>This is an automated notification from Pulse Monitoring</p>
            <p>View alerts and configure settings in your Pulse dashboard</p>
        </div>
    </div>
</body>
</html>`, valueHeading, alertRows.String())

	// Plain text version
	var textBuilder strings.Builder
	textBuilder.WriteString("PULSE MONITORING ALERT SUMMARY\n\n")
	textBuilder.WriteString(fmt.Sprintf("%d New Alert%s\n", len(alertList), pluralize(len(alertList))))
	if critical > 0 {
		textBuilder.WriteString(fmt.Sprintf("Critical: %d\n", critical))
	}
	if warning > 0 {
		textBuilder.WriteString(fmt.Sprintf("Warning: %d\n", warning))
	}
	if info > 0 {
		textBuilder.WriteString(fmt.Sprintf("Info: %d\n", info))
	}
	textBuilder.WriteString("\nAlert Details:\n")
	textBuilder.WriteString("─────────────────────────────────────────────────────────────\n")

	for i, alert := range alertList {
		level := alerts.NormalizeAlertLevel(alert.Level)
		textBuilder.WriteString(fmt.Sprintf("\n%d. %s (%s)\n", i+1, alert.ResourceName, alert.ResourceID))
		textBuilder.WriteString(fmt.Sprintf("   Level: %s | Type: %s\n", strings.ToUpper(string(level)), alert.Type))
		if !isPatrolFindingAlert(alert) {
			textBuilder.WriteString(fmt.Sprintf("   Value: %s (Threshold: %s)\n", formatMetricValue(alert.Type, alert.Value), formatMetricThreshold(alert.Type, alert.Threshold)))
		}
		textBuilder.WriteString(fmt.Sprintf("   Node: %s | Started: %s ago\n", alertNodeDisplay(alert), formatDuration(time.Since(alert.StartTime))))
		textBuilder.WriteString(fmt.Sprintf("   Message: %s\n", alert.Message))
	}

	textBuilder.WriteString("\n─────────────────────────────────────────────────────────────\n")
	textBuilder.WriteString("This is an automated notification from Pulse Monitoring.\n")
	textBuilder.WriteString("Configure alert settings in the Pulse dashboard.")

	textBody = textBuilder.String()
	return subject, htmlBody, textBody
}

// formatAlertStartTime renders in the server's local zone with the zone name
// visible. Alert start times are stored in UTC; without the conversion and
// label the email showed a bare UTC clock that read as local time (#1582).
func formatAlertStartTime(t time.Time) string {
	return t.Local().Format("Jan 2, 2006 at 3:04 PM MST")
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	}
	return fmt.Sprintf("%.1f days", d.Hours()/24)
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// formatMetricValue formats a metric value with the appropriate unit
func formatMetricValue(metricType string, value float64) string {
	switch strings.ToLower(metricType) {
	case "diskread", "diskwrite", "networkin", "networkout":
		return fmt.Sprintf("%.1f MB/s", value)
	case "temperature":
		return fmt.Sprintf("%.1f°C", value)
	case "cpu", "memory", "disk", "usage":
		return fmt.Sprintf("%.1f%%", value)
	default:
		return fmt.Sprintf("%.1f", value)
	}
}

// formatMetricThreshold formats a metric threshold with the appropriate unit
func formatMetricThreshold(metricType string, threshold float64) string {
	switch strings.ToLower(metricType) {
	case "diskread", "diskwrite", "networkin", "networkout":
		return fmt.Sprintf("%.0f MB/s", threshold)
	case "temperature":
		return fmt.Sprintf("%.0f°C", threshold)
	case "cpu", "memory", "disk", "usage":
		return fmt.Sprintf("%.0f%%", threshold)
	default:
		return fmt.Sprintf("%.0f", threshold)
	}
}
