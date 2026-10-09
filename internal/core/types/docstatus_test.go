package types

import "testing"

func TestDocStatus(t *testing.T) {
	if DocStatusDraft.String() != "Draft" {
		t.Errorf("expected Draft, got %s", DocStatusDraft.String())
	}
	if !DocStatusDraft.IsDraft() {
		t.Errorf("expected IsDraft to be true")
	}

	if DocStatusSubmitted.String() != "Submitted" {
		t.Errorf("expected Submitted, got %s", DocStatusSubmitted.String())
	}
	if !DocStatusSubmitted.IsSubmitted() {
		t.Errorf("expected IsSubmitted to be true")
	}

	if DocStatusCancelled.String() != "Cancelled" {
		t.Errorf("expected Cancelled, got %s", DocStatusCancelled.String())
	}
	if !DocStatusCancelled.IsCancelled() {
		t.Errorf("expected IsCancelled to be true")
	}

	unknown := DocStatus(99)
	if unknown.String() != "Unknown(99)" {
		t.Errorf("expected Unknown(99), got %s", unknown.String())
	}
}
