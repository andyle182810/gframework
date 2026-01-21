package httpserver

const (
	DefaultPage     = 1
	DefaultPageSize = 50
	MaxPageSize     = 100
)

func NormalizePage(page, pageSize int) (normalizedPage, normalizedSize, offset int) { //nolint:nonamedreturns
	if page < 1 {
		page = DefaultPage
	}

	if pageSize <= 0 {
		pageSize = DefaultPageSize
	} else if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	return page, pageSize, (page - 1) * pageSize
}

func NewPagination(page, pageSize, totalCount int) *Pagination {
	totalPages := 0
	if pageSize > 0 {
		totalPages = (totalCount + pageSize - 1) / pageSize
	}

	return &Pagination{
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: totalPages,
	}
}
