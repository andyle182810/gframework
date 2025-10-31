package pagination

const (
	DefaultPage     = 1
	DefaultPageSize = 100
	MaxPageSize     = 100
)

func Normalize(page, pageSize int) (int, int, int) {
	if page < 1 {
		page = DefaultPage
	}

	if pageSize <= 0 {
		pageSize = DefaultPageSize
	} else if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	offset := (page - 1) * pageSize

	return page, pageSize, offset
}

func ComputeTotals(totalCount, pageSize int) int {
	totalPages := 0
	if pageSize > 0 {
		totalPages = (totalCount + pageSize - 1) / pageSize
	}

	return totalPages
}
