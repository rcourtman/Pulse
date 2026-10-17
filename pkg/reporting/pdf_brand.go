package reporting

import "github.com/go-pdf/fpdf"

// ReportCoverBrandName and ReportHeaderBrandName share the existing report
// branding policy with extension renderers. Callers pass already-authorized
// effective branding; these primitives do not grant branding entitlement.
func ReportCoverBrandName(brand *ReportBrand) string { return reportCoverBrandName(brand) }
func ReportHeaderBrandName(brand *ReportBrand, fallback string) string {
	return reportHeaderBrandName(brand, fallback)
}

// DrawReportBrandLogo uses the same decoding and sizing as existing reports.
func DrawReportBrandLogo(pdf *fpdf.Fpdf, brand *ReportBrand, x, y, width, height float64) {
	NewPDFGenerator().drawBrandLogo(pdf, brand, x, y, width, height)
}
