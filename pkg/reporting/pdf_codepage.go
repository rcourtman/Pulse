package reporting

import "reflect"

// The PDF renderer uses fpdf core fonts (Arial), which read text as cp1252
// bytes. Free-form strings entering ReportData/MultiReportData are UTF-8:
// AI narrative prose full of em dashes and curly quotes, user-controlled
// resource names, alert messages, brand display names. Writing them
// untranslated renders mojibake in the PDF (an em dash becomes "â€”").
//
// translateReportStrings and translateMultiReportStrings rewrite every
// reachable string field through fpdf's cp1252 translator once, before
// rendering, so individual write sites never have to think about encoding.
// Reflection (rather than a hand-maintained field list) keeps newly added
// narrative, branding, and enrichment fields covered without anyone having
// to remember this layer exists. Runes cp1252 cannot represent degrade to
// "." (fpdf's documented behaviour), which is unfortunate but readable;
// raw UTF-8 bytes are neither.
//
// The translator must be created per Generate call: fpdf's translator
// closure reuses an internal buffer, so it is not safe for concurrent use,
// and one PDFGenerator is shared across concurrent requests. Translation
// mutates the data in place; report data is built per request, so no caller
// observes the rewrite.

// translateReportStrings rewrites every string field reachable from data
// into the cp1252 byte space the core PDF fonts expect. tr is the output of
// pdf.UnicodeTranslatorFromDescriptor for the document being generated.
func translateReportStrings(data *ReportData, tr func(string) string) {
	if data == nil || tr == nil {
		return
	}
	translateStringFields(reflect.ValueOf(data).Elem(), tr, map[uintptr]bool{})
}

// translateMultiReportStrings is the fleet-report counterpart of
// translateReportStrings, covering the fleet narrative and every
// per-resource ReportData.
func translateMultiReportStrings(data *MultiReportData, tr func(string) string) {
	if data == nil || tr == nil {
		return
	}
	translateStringFields(reflect.ValueOf(data).Elem(), tr, map[uintptr]bool{})
}

// translateStringFields walks v depth-first and rewrites every settable
// string through tr. visited guards pointer aliasing (e.g. a *ReportBrand
// shared between MultiReportData and its per-resource ReportData entries):
// cp1252 translation is not idempotent, because a second pass would read
// the raw high bytes as invalid UTF-8 and flatten them to ".".
func translateStringFields(v reflect.Value, tr func(string) string, visited map[uintptr]bool) {
	switch v.Kind() {
	case reflect.String:
		if v.CanSet() {
			v.SetString(tr(v.String()))
		}
	case reflect.Pointer:
		if v.IsNil() || visited[v.Pointer()] {
			return
		}
		visited[v.Pointer()] = true
		translateStringFields(v.Elem(), tr, visited)
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			// Unexported fields (time.Time internals) are read-only
			// through reflection and hold nothing user-visible.
			if !t.Field(i).IsExported() {
				continue
			}
			translateStringFields(v.Field(i), tr, visited)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			translateStringFields(v.Index(i), tr, visited)
		}
	case reflect.Map:
		// Keys stay untouched: they are lookup identifiers (metric
		// names), and translating them would break renderer lookups.
		for _, k := range v.MapKeys() {
			mv := v.MapIndex(k)
			switch mv.Kind() {
			case reflect.String:
				v.SetMapIndex(k, reflect.ValueOf(tr(mv.String())).Convert(mv.Type()))
			case reflect.Struct:
				// Map values are not addressable; translate a
				// copy and store it back.
				cp := reflect.New(mv.Type()).Elem()
				cp.Set(mv)
				translateStringFields(cp, tr, visited)
				v.SetMapIndex(k, cp)
			case reflect.Pointer, reflect.Slice, reflect.Map:
				translateStringFields(mv, tr, visited)
			}
		}
	}
}
