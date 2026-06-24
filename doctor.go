package slides

// DoctorItem is one environment or deck health check.
type DoctorItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// DoctorReport is returned by Doctor.
type DoctorReport struct {
	Items []DoctorItem `json:"items"`
}

// HasFailures reports whether any doctor item failed.
func (report DoctorReport) HasFailures() bool {
	for _, item := range report.Items {
		if item.Status == "fail" {
			return true
		}
	}
	return false
}
