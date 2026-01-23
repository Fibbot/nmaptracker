package web

import (
	"fmt"
	"net/url"
	"strings"
)

func buildHostListLink(projectID int64, filters hostListFilters, page int) string {
	values := make([]string, 0, 8)
	if filters.Subnet != "" {
		values = append(values, "subnet="+url.QueryEscape(filters.Subnet))
	}
	if filters.Status != "" {
		values = append(values, "status="+url.QueryEscape(filters.Status))
	}
	if filters.InScope != "" {
		values = append(values, "in_scope="+url.QueryEscape(filters.InScope))
	}
	if filters.Sort != "" {
		values = append(values, "sort="+url.QueryEscape(filters.Sort))
	}
	if filters.Dir != "" {
		values = append(values, "dir="+url.QueryEscape(filters.Dir))
	}
	values = append(values, fmt.Sprintf("page=%d", page))
	if filters.Size != "" {
		values = append(values, "page_size="+url.QueryEscape(filters.Size))
	}
	if len(values) == 0 {
		return fmt.Sprintf("/projects/%d/hosts", projectID)
	}
	return fmt.Sprintf("/projects/%d/hosts?%s", projectID, strings.Join(values, "&"))
}
