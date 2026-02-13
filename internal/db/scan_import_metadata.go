package db

import "database/sql"

func nullableStringValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableIntValue(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func ptrStringFromNull(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func ptrIntFromNull(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}
