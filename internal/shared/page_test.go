package shared_test

import (
	"testing"

	"github.com/shareinto/paas/internal/shared"
)

func TestPageRequestNormalizeAndOffset(t *testing.T) {
	req := shared.PageRequest{Page: 0, PageSize: -1}.Normalize()
	if req.Page != 1 || req.PageSize != shared.DefaultPageSize {
		t.Fatalf("Normalize() = %+v", req)
	}

	req = shared.PageRequest{Page: 3, PageSize: shared.MaxPageSize + 1}.Normalize()
	if req.PageSize != shared.MaxPageSize {
		t.Fatalf("PageSize = %d, want %d", req.PageSize, shared.MaxPageSize)
	}
	if got := req.Offset(); got != 2*shared.MaxPageSize {
		t.Fatalf("Offset() = %d", got)
	}
}

func TestNewPageResultNormalizesNilItems(t *testing.T) {
	result := shared.NewPageResult[string](nil, 3, shared.PageRequest{})

	if result.Items == nil {
		t.Fatalf("Items should be an empty slice, not nil")
	}
	if result.Page != 1 || result.PageSize != shared.DefaultPageSize || result.Total != 3 {
		t.Fatalf("unexpected page result: %+v", result)
	}
}
