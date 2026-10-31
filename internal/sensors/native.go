package sensors

import (
	"math"
	"sort"
	"strconv"
	"strings"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog/log"
)

// The inventory vocabulary is the agent report's, so the parser and the wire
// format cannot drift.
const (
	NativeSensorKindTemperature = agentshost.NativeSensorKindTemperature
	NativeSensorKindFan         = agentshost.NativeSensorKindFan

	NativeSensorUnitCelsius = agentshost.NativeSensorUnitCelsius
	NativeSensorUnitRPM     = agentshost.NativeSensorUnitRPM

	NativeSensorStatusOK      = agentshost.NativeSensorStatusOK
	NativeSensorStatusInvalid = agentshost.NativeSensorStatusInvalid

	// MaxNativeSensors bounds the inventory one host reports. Laptops and
	// boards with an SMC or Super I/O chip expose about 50 inputs; anything
	// beyond this is truncated in stable ID order rather than growing the
	// report without limit.
	MaxNativeSensors = 256

	// Common sensor ICs are rated from -55 °C, and the parser already treats
	// readings above 150 °C as implausible. Chips report values outside that
	// band, such as applesmc's -127 °C, for inputs with nothing attached.
	minPlausibleNativeCelsius = -55.0
	maxPlausibleNativeCelsius = 150.0
)

// NativeSensor is one lm-sensors temperature or fan input as the host
// reported it. ID joins the full chip name, including its bus address, to the
// native input channel. It is a host-scoped source key: two chips of one type
// and labels that differ only by case get separate IDs, and an ID stays the
// same while the kernel enumerates the chip the same way. It is not a
// physical-device identity, so a moved PCI address or renumbered channel can
// change or reuse it. Label keeps the chip's original feature name. Values are
// the Celsius or RPM lm-sensors reports, unconverted; a finite value outside
// the plausible range is kept with an invalid status, and a non-finite value is
// omitted.
type NativeSensor struct {
	ID      string
	Chip    string
	Adapter string
	Channel string
	Label   string
	Kind    string
	Unit    string
	Value   *float64
	Status  string
}

// collectNativeSensors lists every temperature and fan input of one chip.
func collectNativeSensors(chipName string, chipMap map[string]interface{}) []NativeSensor {
	adapter, _ := chipMap["Adapter"].(string)
	var result []NativeSensor
	for label, featureData := range chipMap {
		featureMap, ok := featureData.(map[string]interface{})
		if !ok {
			continue
		}
		for key, raw := range featureMap {
			channel, found := strings.CutSuffix(key, "_input")
			if !found {
				continue
			}
			kind, ok := NativeSensorChannelKind(channel)
			if !ok {
				continue
			}
			sensor := NativeSensor{
				ID:      chipName + "/" + channel,
				Chip:    chipName,
				Adapter: adapter,
				Channel: channel,
				Label:   label,
				Kind:    kind,
			}
			if value, finite := nativeSensorValue(raw); finite {
				sensor.Value = &value
			}
			sensor.Status = NativeSensorStatus(kind, sensor.Value)
			if kind == NativeSensorKindTemperature {
				sensor.Unit = NativeSensorUnitCelsius
			} else {
				sensor.Unit = NativeSensorUnitRPM
			}
			result = append(result, sensor)
		}
	}
	return result
}

// NativeSensorChannelKind returns the kind of a canonical lm-sensors channel
// name: tempN or fanN, where N is decimal without leading zeros. Any other
// spelling is rejected so one input cannot appear under two channel names.
func NativeSensorChannelKind(channel string) (string, bool) {
	var kind, index string
	switch {
	case strings.HasPrefix(channel, "temp"):
		kind, index = NativeSensorKindTemperature, strings.TrimPrefix(channel, "temp")
	case strings.HasPrefix(channel, "fan"):
		kind, index = NativeSensorKindFan, strings.TrimPrefix(channel, "fan")
	default:
		return "", false
	}
	if index == "" || (len(index) > 1 && index[0] == '0') {
		return "", false
	}
	for _, digit := range index {
		if digit < '0' || digit > '9' {
			return "", false
		}
	}
	return kind, true
}

// NativeSensorStatus is the plausibility policy for a native reading: ok
// needs a finite value, from -55 to 150 °C for a temperature or at least
// 0 RPM for a fan. The agent and the server's report ingest both apply it, so
// the server's judgement does not depend on the agent's version.
func NativeSensorStatus(kind string, value *float64) string {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return NativeSensorStatusInvalid
	}
	switch kind {
	case NativeSensorKindTemperature:
		if *value >= minPlausibleNativeCelsius && *value <= maxPlausibleNativeCelsius {
			return NativeSensorStatusOK
		}
	case NativeSensorKindFan:
		// A stopped fan reads 0 RPM, which is a real observation.
		if *value >= 0 {
			return NativeSensorStatusOK
		}
	}
	return NativeSensorStatusInvalid
}

func nativeSensorValue(raw interface{}) (float64, bool) {
	var value float64
	switch v := raw.(type) {
	case float64:
		value = v
	case int:
		value = float64(v)
	case int64:
		value = float64(v)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, false
		}
		value = parsed
	default:
		return 0, false
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func nativeSensorChannelIndex(sensor NativeSensor) int {
	prefix := "temp"
	if sensor.Kind == NativeSensorKindFan {
		prefix = "fan"
	}
	index, _ := strconv.Atoi(strings.TrimPrefix(sensor.Channel, prefix))
	return index
}

// sortAndBoundNativeSensors orders the inventory by chip, then kind, then
// channel number, keeps one entry per ID and drops entries beyond
// MaxNativeSensors. lm-sensors names each subfeature once per chip; a
// malformed report that repeats one keeps the first label in sort order.
// Truncation drops the last chips in name order, so a newly added chip can
// displace another chip's inputs; consumers must not read a missing entry as
// a removed sensor.
func sortAndBoundNativeSensors(inventory []NativeSensor) []NativeSensor {
	sort.Slice(inventory, func(i, j int) bool {
		a, b := inventory[i], inventory[j]
		if a.Chip != b.Chip {
			return a.Chip < b.Chip
		}
		if a.Kind != b.Kind {
			return a.Kind == NativeSensorKindTemperature
		}
		if ai, bi := nativeSensorChannelIndex(a), nativeSensorChannelIndex(b); ai != bi {
			return ai < bi
		}
		return a.Label < b.Label
	})
	unique := inventory[:0]
	seen := make(map[string]struct{}, len(inventory))
	for _, sensor := range inventory {
		if _, duplicate := seen[sensor.ID]; duplicate {
			continue
		}
		seen[sensor.ID] = struct{}{}
		unique = append(unique, sensor)
	}
	inventory = unique
	if len(inventory) > MaxNativeSensors {
		log.Debug().
			Int("reported", len(inventory)).
			Int("kept", MaxNativeSensors).
			Msg("Truncated native sensor inventory")
		inventory = inventory[:MaxNativeSensors]
	}
	return inventory
}
