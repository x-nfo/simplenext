package types

import "fmt"

// DocStatus merepresentasikan siklus dokumen transaksi standar ERPNext
type DocStatus int16

const (
	DocStatusDraft     DocStatus = 0 // Masih dapat diedit, belum masuk buku besar
	DocStatusSubmitted DocStatus = 1 // Terkunci (read-only), sudah diposting ke GL
	DocStatusCancelled DocStatus = 2 // Dibatalkan, sudah dilakukan reversal di GL
)

func (s DocStatus) String() string {
	switch s {
	case DocStatusDraft:
		return "Draft"
	case DocStatusSubmitted:
		return "Submitted"
	case DocStatusCancelled:
		return "Cancelled"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

func (s DocStatus) IsDraft() bool {
	return s == DocStatusDraft
}

func (s DocStatus) IsSubmitted() bool {
	return s == DocStatusSubmitted
}

func (s DocStatus) IsCancelled() bool {
	return s == DocStatusCancelled
}
