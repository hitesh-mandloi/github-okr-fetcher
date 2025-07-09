package ports

// AnalysisService defines the interface for OKR analysis
type AnalysisService interface {
	AnalyzeOKRs(okrData string) (string, error)
}