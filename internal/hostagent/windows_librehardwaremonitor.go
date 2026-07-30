package hostagent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const (
	libreHardwareMonitorDataURL        = "http://127.0.0.1:8085/data.json"
	libreHardwareMonitorQueryTimeout   = 2 * time.Second
	libreHardwareMonitorMaxOutputBytes = 1024 * 1024
	libreHardwareMonitorMaxNodes       = 2048
	libreHardwareMonitorMaxDepth       = 16
	libreHardwareMonitorMaxSensors     = 256
	libreHardwareMonitorMaxKeyLength   = 128
)

type libreHardwareMonitorNode struct {
	Text       string                     `json:"Text"`
	SensorID   string                     `json:"SensorId"`
	Type       string                     `json:"Type"`
	RawValue   *float64                   `json:"RawValue"`
	HardwareID string                     `json:"HardwareId"`
	Children   []libreHardwareMonitorNode `json:"Children"`
}

type libreHardwareMonitorTraversal struct {
	node       *libreHardwareMonitorNode
	hardwareID string
	depth      int
}

func (a *Agent) mergeLibreHardwareMonitorTemperatures(ctx context.Context, result *agentshost.Sensors) {
	endpoint := a.libreHardwareMonitorEndpoint
	if endpoint == "" {
		endpoint = libreHardwareMonitorDataURL
	}

	temperatures, err := collectLibreHardwareMonitorTemperatures(ctx, endpoint)
	if err != nil {
		a.logger.Debug().Err(err).Msg("Failed to collect LibreHardwareMonitor temperatures")
		return
	}
	if len(temperatures) == 0 {
		return
	}

	if result.TemperatureCelsius == nil {
		result.TemperatureCelsius = make(map[string]float64, len(temperatures))
	}
	for key, value := range temperatures {
		result.TemperatureCelsius[key] = value
	}

	a.logger.Debug().
		Int("sensorCount", len(temperatures)).
		Msg("Collected LibreHardwareMonitor CPU and motherboard temperatures")
}

func collectLibreHardwareMonitorTemperatures(ctx context.Context, endpoint string) (map[string]float64, error) {
	queryCtx, cancel := context.WithTimeout(ctx, libreHardwareMonitorQueryTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(queryCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create local LibreHardwareMonitor request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	client := &http.Client{
		Timeout: libreHardwareMonitorQueryTimeout,
		Transport: &http.Transport{
			Proxy:             nil,
			DisableKeepAlives: true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("query local LibreHardwareMonitor: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("local LibreHardwareMonitor returned %s", response.Status)
	}
	if response.ContentLength > libreHardwareMonitorMaxOutputBytes {
		return nil, fmt.Errorf(
			"LibreHardwareMonitor response exceeds %d bytes",
			libreHardwareMonitorMaxOutputBytes,
		)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, libreHardwareMonitorMaxOutputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read local LibreHardwareMonitor response: %w", err)
	}
	if len(body) > libreHardwareMonitorMaxOutputBytes {
		return nil, fmt.Errorf(
			"LibreHardwareMonitor response exceeds %d bytes",
			libreHardwareMonitorMaxOutputBytes,
		)
	}

	temperatures, err := parseLibreHardwareMonitorTemperatures(body)
	if err != nil {
		return nil, fmt.Errorf("parse local LibreHardwareMonitor response: %w", err)
	}
	return temperatures, nil
}

func parseLibreHardwareMonitorTemperatures(body []byte) (map[string]float64, error) {
	var root libreHardwareMonitorNode
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}

	result := make(map[string]float64)
	stack := []libreHardwareMonitorTraversal{{node: &root}}
	nodeCount := 0

	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]

		nodeCount++
		if nodeCount > libreHardwareMonitorMaxNodes {
			return nil, fmt.Errorf("LibreHardwareMonitor tree exceeds %d nodes", libreHardwareMonitorMaxNodes)
		}
		if current.depth > libreHardwareMonitorMaxDepth {
			return nil, fmt.Errorf("LibreHardwareMonitor tree exceeds depth %d", libreHardwareMonitorMaxDepth)
		}

		hardwareID := current.hardwareID
		if candidate := strings.TrimSpace(current.node.HardwareID); candidate != "" {
			hardwareID = candidate
		}

		if strings.EqualFold(strings.TrimSpace(current.node.Type), "Temperature") &&
			current.node.RawValue != nil &&
			!math.IsNaN(*current.node.RawValue) &&
			!math.IsInf(*current.node.RawValue, 0) &&
			*current.node.RawValue > 0 &&
			*current.node.RawValue <= 150 {
			if category := classifyLibreHardwareMonitorTemperature(hardwareID); category != "" {
				key := normalizeLibreHardwareMonitorTemperatureKey(
					category,
					current.node.SensorID,
				)
				if key != "" {
					result[key] = *current.node.RawValue
					if len(result) > libreHardwareMonitorMaxSensors {
						return nil, fmt.Errorf(
							"LibreHardwareMonitor response exceeds %d supported temperature sensors",
							libreHardwareMonitorMaxSensors,
						)
					}
				}
			}
		}

		for index := len(current.node.Children) - 1; index >= 0; index-- {
			stack = append(stack, libreHardwareMonitorTraversal{
				node:       &current.node.Children[index],
				hardwareID: hardwareID,
				depth:      current.depth + 1,
			})
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func classifyLibreHardwareMonitorTemperature(hardwareID string) string {
	hardwareID = "/" + strings.Trim(strings.ToLower(strings.TrimSpace(hardwareID)), "/") + "/"
	switch {
	case strings.Contains(hardwareID, "/intelcpu/"),
		strings.Contains(hardwareID, "/amdcpu/"),
		strings.Contains(hardwareID, "/cpu/"):
		return "cpu"
	case strings.Contains(hardwareID, "/lpc/"),
		strings.Contains(hardwareID, "/motherboard/"),
		strings.Contains(hardwareID, "/superio/"),
		strings.Contains(hardwareID, "/embeddedcontroller/"):
		return "motherboard"
	default:
		return ""
	}
}

func normalizeLibreHardwareMonitorTemperatureKey(category, sensorID string) string {
	identity := strings.TrimSpace(sensorID)
	if identity == "" {
		return ""
	}

	var normalized strings.Builder
	normalized.Grow(len(identity))
	lastUnderscore := false
	for _, char := range strings.ToLower(identity) {
		isAlphaNumeric := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
		if isAlphaNumeric {
			normalized.WriteRune(char)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			normalized.WriteByte('_')
			lastUnderscore = true
		}
	}

	suffix := strings.Trim(normalized.String(), "_")
	if suffix == "" {
		return ""
	}
	prefix := category + "_lhm_"
	key := prefix + suffix
	if len(key) > libreHardwareMonitorMaxKeyLength {
		hash := sha256.Sum256([]byte(identity))
		hashSuffix := fmt.Sprintf("_%x", hash[:4])
		suffixLength := libreHardwareMonitorMaxKeyLength - len(prefix) - len(hashSuffix)
		key = prefix + strings.TrimRight(suffix[:suffixLength], "_") + hashSuffix
	}
	return key
}
