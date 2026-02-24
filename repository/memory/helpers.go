package memory

const defaultPageSize = 50
const maxPageSize = 100

func paginate(total, page, pageSize int) (start, end int) {
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	if page <= 0 {
		page = 1
	}

	start = (page - 1) * pageSize
	if start > total {
		start = total
	}

	end = start + pageSize
	if end > total {
		end = total
	}

	return start, end
}
