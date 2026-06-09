package shared

const (
	DefaultPageSize = 20
	MaxPageSize     = 200
)

type PageRequest struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func (r PageRequest) Normalize() PageRequest {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 {
		r.PageSize = DefaultPageSize
	}
	if r.PageSize > MaxPageSize {
		r.PageSize = MaxPageSize
	}
	return r
}

func (r PageRequest) Offset() int {
	r = r.Normalize()
	return (r.Page - 1) * r.PageSize
}

type PageResult[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func NewPageResult[T any](items []T, total int64, req PageRequest) PageResult[T] {
	req = req.Normalize()
	if items == nil {
		items = []T{}
	}
	return PageResult[T]{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}
}
