package web

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/sloppy/nmaptracker/internal/db"
)

func render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}

func layout(title string, body templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, "<!doctype html><html lang=\"en\"><head>"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<meta charset=\"utf-8\">"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<title>%s</title>", html.EscapeString(title)); err != nil {
			return err
		}
		if _, err := io.WriteString(w, layoutStyles); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "</head><body>"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<main class=\"shell\">"); err != nil {
			return err
		}
		if err := body.Render(ctx, w); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "</main></body></html>"); err != nil {
			return err
		}
		return nil
	})
}

func projectsListPage(projects []db.Project) templ.Component {
	body := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, "<header class=\"page-header\"><p class=\"eyebrow\">NmapTracker</p><h1>Projects</h1><p class=\"subhead\">Create a workspace for scan imports and progress tracking.</p></header>"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<section class=\"card\"><form method=\"post\" action=\"/projects\" class=\"project-form\"><label for=\"project-name\">Project name</label><div class=\"project-form__row\"><input id=\"project-name\" name=\"name\" placeholder=\"Acme Internal\" required><button type=\"submit\">Create</button></div></form></section>"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<section class=\"card\"><h2>Existing projects</h2>"); err != nil {
			return err
		}
		if len(projects) == 0 {
			if _, err := io.WriteString(w, "<p class=\"empty\">No projects yet. Add one to get started.</p></section>"); err != nil {
				return err
			}
			return nil
		}
		if _, err := io.WriteString(w, "<ul class=\"project-list\">"); err != nil {
			return err
		}
		for _, project := range projects {
			escaped := html.EscapeString(project.Name)
			if _, err := fmt.Fprintf(w, "<li><a class=\"project-link\" href=\"/projects/%d\">%s</a><form method=\"post\" action=\"/projects/%d/delete\"><button class=\"ghost\" type=\"submit\">Delete</button></form></li>", project.ID, escaped, project.ID); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "</ul></section>"); err != nil {
			return err
		}
		return nil
	})
	return layout("NmapTracker - Projects", body)
}

func dashboardPage(project db.Project, stats db.DashboardStats, totalFlagged, progress int) templ.Component {
	body := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := fmt.Fprintf(w, "<header class=\"page-header\"><p class=\"eyebrow\">Project</p><h1>%s</h1><p class=\"subhead\">Coverage snapshot for in-scope assets and workflow progress.</p></header>", html.EscapeString(project.Name)); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<section class=\"card\"><h2>Hosts</h2><div class=\"stats-grid\">"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<div><p class=\"stat-label\">Total hosts</p><p class=\"stat-value\">%d</p></div>", stats.TotalHosts); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<div><p class=\"stat-label\">In scope</p><p class=\"stat-value\">%d</p></div>", stats.InScopeHosts); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<div><p class=\"stat-label\">Out of scope</p><p class=\"stat-value\">%d</p></div>", stats.OutScopeHosts); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "</div></section>"); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<section class=\"card\"><h2>Workflow</h2>"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<p class=\"progress\">Progress: <strong>%d / %d</strong> (%d%%)</p>", stats.WorkStatus.Done, totalFlagged, progress); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<div class=\"stats-grid\">"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<div><p class=\"stat-label\">Scanned</p><p class=\"stat-value\">%d</p></div>", stats.WorkStatus.Scanned); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<div><p class=\"stat-label\">Flagged</p><p class=\"stat-value\">%d</p></div>", stats.WorkStatus.Flagged); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<div><p class=\"stat-label\">In progress</p><p class=\"stat-value\">%d</p></div>", stats.WorkStatus.InProgress); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<div><p class=\"stat-label\">Done</p><p class=\"stat-value\">%d</p></div>", stats.WorkStatus.Done); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<div><p class=\"stat-label\">Parking lot</p><p class=\"stat-value\">%d</p></div>", stats.WorkStatus.ParkingLot); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "</div></section>"); err != nil {
			return err
		}

		if _, err := fmt.Fprintf(w, "<div class=\"page-actions\"><a class=\"back-link\" href=\"/projects/%d/hosts\">View hosts</a><a class=\"back-link\" href=\"/projects/%d/export?format=json\">Export JSON</a><a class=\"back-link\" href=\"/projects/%d/export?format=csv\">Export CSV</a><a class=\"back-link\" href=\"/projects\">Back to projects</a></div>", project.ID, project.ID, project.ID); err != nil {
			return err
		}
		return nil
	})
	return layout("NmapTracker - Dashboard", body)
}

func hostListPage(project db.Project, filters hostListFilters, items []db.HostListItem, total int) templ.Component {
	body := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := fmt.Fprintf(w, "<header class=\"page-header\"><p class=\"eyebrow\">Project</p><h1>%s</h1><p class=\"subhead\">Host inventory with port summary and workflow status.</p></header>", html.EscapeString(project.Name)); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<section class=\"card\"><div class=\"filters-wrap\"><form method=\"get\" class=\"filters\">"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<label>Subnet/CIDR<input name=\"subnet\" placeholder=\"10.0.0.0/24\" value=\"%s\"></label>", html.EscapeString(filters.Subnet)); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<label>Status<select name=\"status\">"); err != nil {
			return err
		}
		for _, opt := range []struct {
			Value string
			Label string
		}{
			{"", "Any"},
			{"scanned", "Scanned"},
			{"flagged", "Flagged"},
			{"in_progress", "In progress"},
			{"done", "Done"},
			{"parking_lot", "Parking lot"},
		} {
			selected := ""
			if filters.Status == opt.Value {
				selected = " selected"
			}
			if _, err := fmt.Fprintf(w, "<option value=\"%s\"%s>%s</option>", opt.Value, selected, opt.Label); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "</select></label>"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<label>In scope<select name=\"in_scope\">"); err != nil {
			return err
		}
		for _, opt := range []struct {
			Value string
			Label string
		}{
			{"", "Any"},
			{"true", "In scope"},
			{"false", "Out of scope"},
		} {
			selected := ""
			if filters.InScope == opt.Value {
				selected = " selected"
			}
			if _, err := fmt.Fprintf(w, "<option value=\"%s\"%s>%s</option>", opt.Value, selected, opt.Label); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "</select></label>"); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<label>Sort<select name=\"sort\">"); err != nil {
			return err
		}
		for _, opt := range []struct {
			Value string
			Label string
		}{
			{"ip", "IP address"},
			{"hostname", "Hostname"},
			{"ports", "Port count"},
		} {
			selected := ""
			if filters.Sort == opt.Value {
				selected = " selected"
			}
			if _, err := fmt.Fprintf(w, "<option value=\"%s\"%s>%s</option>", opt.Value, selected, opt.Label); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "</select></label>"); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<label>Direction<select name=\"dir\">"); err != nil {
			return err
		}
		for _, opt := range []struct {
			Value string
			Label string
		}{
			{"asc", "Ascending"},
			{"desc", "Descending"},
		} {
			selected := ""
			if filters.Dir == opt.Value {
				selected = " selected"
			}
			if _, err := fmt.Fprintf(w, "<option value=\"%s\"%s>%s</option>", opt.Value, selected, opt.Label); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "</select></label>"); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<input type=\"hidden\" name=\"page\" value=\"1\"><div class=\"filter-actions\"><button type=\"submit\">Apply filters</button></div></form></div></section>"); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<section class=\"card\"><div class=\"bulk-actions\"><div><h2>Bulk actions</h2><p class=\"subhead\">Apply a work status to all open ports in the current host view.</p></div><form method=\"post\" action=\""); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "/projects/%d/hosts/bulk-status", project.ID); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\" class=\"bulk-form\">"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<input type=\"hidden\" name=\"subnet\" value=\"%s\"><input type=\"hidden\" name=\"status_filter\" value=\"%s\"><input type=\"hidden\" name=\"in_scope\" value=\"%s\">", html.EscapeString(filters.Subnet), html.EscapeString(filters.Status), html.EscapeString(filters.InScope)); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<select name=\"status\">"); err != nil {
			return err
		}
		for _, opt := range []struct {
			Value string
			Label string
		}{
			{"scanned", "Scanned"},
			{"flagged", "Flagged"},
			{"in_progress", "In progress"},
			{"done", "Done"},
			{"parking_lot", "Parking lot"},
		} {
			if _, err := fmt.Fprintf(w, "<option value=\"%s\">%s</option>", opt.Value, opt.Label); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "</select><button type=\"submit\">Apply to filtered hosts</button></form></div></section>"); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<section class=\"card\"><div class=\"bulk-actions\"><div><h2>Update a port across the project</h2><p class=\"subhead\">Set status on a port number across all in-scope hosts.</p></div><form method=\"post\" action=\""); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "/projects/%d/ports/bulk-status", project.ID); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\" class=\"bulk-form\">"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<input type=\"number\" min=\"1\" max=\"65535\" name=\"port_number\" placeholder=\"Port #\" required>"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<select name=\"status\">"); err != nil {
			return err
		}
		for _, opt := range []struct {
			Value string
			Label string
		}{
			{"scanned", "Scanned"},
			{"flagged", "Flagged"},
			{"in_progress", "In progress"},
			{"done", "Done"},
			{"parking_lot", "Parking lot"},
		} {
			if _, err := fmt.Fprintf(w, "<option value=\"%s\">%s</option>", opt.Value, opt.Label); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "</select><button type=\"submit\">Apply to port</button></form></div></section>"); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<section class=\"card\"><div class=\"table-wrap\"><table class=\"host-table\"><thead><tr><th>IP</th><th>Hostname</th><th>Ports</th><th>Status summary</th><th>In scope</th></tr></thead><tbody>"); err != nil {
			return err
		}
		if len(items) == 0 {
			if _, err := io.WriteString(w, "<tr><td colspan=\"5\" class=\"empty\">No hosts match the current filters.</td></tr>"); err != nil {
				return err
			}
		} else {
			for _, item := range items {
				scopeLabel := "Yes"
				if !item.InScope {
					scopeLabel = "No"
				}
				if _, err := fmt.Fprintf(w,
					"<tr><td class=\"mono\"><a href=\"/projects/%d/hosts/%d\">%s</a></td><td>%s</td><td>%d</td><td class=\"status-summary\">S:%d F:%d IP:%d D:%d P:%d</td><td>%s</td></tr>",
					project.ID,
					item.ID,
					html.EscapeString(item.IPAddress),
					html.EscapeString(item.Hostname),
					item.PortCount,
					item.Scanned,
					item.Flagged,
					item.InProgress,
					item.Done,
					item.ParkingLot,
					scopeLabel,
				); err != nil {
					return err
				}
			}
		}
		if _, err := io.WriteString(w, "</tbody></table></div></section>"); err != nil {
			return err
		}

		if total > 0 {
			page := parseInt(filters.Page, 1)
			size := parseInt(filters.Size, 50)
			lastPage := (total + size - 1) / size
			if page > lastPage {
				page = lastPage
			}
			if lastPage < 1 {
				lastPage = 1
			}
			if _, err := io.WriteString(w, "<div class=\"pager\">"); err != nil {
				return err
			}
			if page > 1 {
				prev := buildHostListLink(project.ID, filters, page-1)
				if _, err := fmt.Fprintf(w, "<a class=\"pager-link\" href=\"%s\">Previous</a>", html.EscapeString(prev)); err != nil {
					return err
				}
			} else {
				if _, err := io.WriteString(w, "<span class=\"pager-link disabled\">Previous</span>"); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "<span class=\"pager-status\">Page %d of %d</span>", page, lastPage); err != nil {
				return err
			}
			if page < lastPage {
				next := buildHostListLink(project.ID, filters, page+1)
				if _, err := fmt.Fprintf(w, "<a class=\"pager-link\" href=\"%s\">Next</a>", html.EscapeString(next)); err != nil {
					return err
				}
			} else {
				if _, err := io.WriteString(w, "<span class=\"pager-link disabled\">Next</span>"); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, "</div>"); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(w, "<div class=\"page-actions\"><a class=\"back-link\" href=\"/projects/%d\">Back to dashboard</a><a class=\"back-link\" href=\"/projects\">All projects</a></div>", project.ID); err != nil {
			return err
		}
		return nil
	})
	return layout("NmapTracker - Hosts", body)
}

func hostDetailPage(project db.Project, host db.Host, ports []db.Port, stateFilters map[string]bool) templ.Component {
	body := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		scopeLabel := "In scope"
		if !host.InScope {
			scopeLabel = "Out of scope"
		}
		header := fmt.Sprintf(
			"<header class=\"page-header\"><p class=\"eyebrow\">Host</p><h1>%s</h1><p class=\"subhead\">%s · %s</p></header>",
			html.EscapeString(host.IPAddress),
			html.EscapeString(host.Hostname),
			scopeLabel,
		)
		if _, err := io.WriteString(w, header); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<section class=\"card\"><h2>Details</h2><dl class=\"host-meta\">"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<div><dt>Hostname</dt><dd>%s</dd></div>", html.EscapeString(host.Hostname)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<div><dt>OS Guess</dt><dd>%s</dd></div>", html.EscapeString(host.OSGuess)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<div><dt>Scope</dt><dd>%s</dd></div>", scopeLabel); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "</dl></section>"); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<section class=\"card\"><div class=\"bulk-actions\"><div><h2>Host bulk update</h2><p class=\"subhead\">Set all open ports on this host to a status.</p></div><form method=\"post\" class=\"bulk-form\" action=\""); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "/projects/%d/hosts/%d/bulk-status", project.ID, host.ID); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\"><select name=\"status\">"); err != nil {
			return err
		}
		for _, opt := range []struct {
			Value string
			Label string
		}{
			{"scanned", "Scanned"},
			{"flagged", "Flagged"},
			{"in_progress", "In progress"},
			{"done", "Done"},
			{"parking_lot", "Parking lot"},
		} {
			if _, err := fmt.Fprintf(w, "<option value=\"%s\">%s</option>", opt.Value, opt.Label); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "</select><button type=\"submit\">Update host ports</button></form></div></section>"); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<section class=\"card\"><h2>Host notes</h2><form method=\"post\" class=\"notes-form\" action=\""); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "/projects/%d/hosts/%d/notes", project.ID, host.ID); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\"><textarea name=\"notes\" rows=\"4\" placeholder=\"Add host notes\">"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, html.EscapeString(host.Notes)); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "</textarea><button type=\"submit\">Save host notes</button></form></section>"); err != nil {
			return err
		}

		if _, err := io.WriteString(w, "<section class=\"card\"><div class=\"port-header\"><h2>Ports</h2><form method=\"get\" class=\"state-filters\">"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<div class=\"state-options\">"); err != nil {
			return err
		}
		for _, state := range []struct {
			Value string
			Label string
		}{
			{"open", "Open"},
			{"closed", "Closed"},
			{"filtered", "Filtered"},
		} {
			checked := ""
			if stateFilters != nil && stateFilters[state.Value] {
				checked = " checked"
			}
			if _, err := fmt.Fprintf(w, "<div class=\"checkbox-row\"><input type=\"checkbox\" name=\"state\" value=\"%s\"%s><span>%s</span></div>", state.Value, checked, state.Label); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "</div><div class=\"state-action\"><button type=\"submit\">Filter</button></div></form></div>"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<div class=\"table-wrap\"><table class=\"host-table\"><thead><tr><th>Port</th><th>State</th><th>Service</th><th>Work status</th><th>Notes</th></tr></thead><tbody>"); err != nil {
			return err
		}
		if len(ports) == 0 {
			if _, err := io.WriteString(w, "<tr><td colspan=\"5\" class=\"empty\">No ports recorded for this host.</td></tr>"); err != nil {
				return err
			}
		} else {
			for _, port := range ports {
				if _, err := fmt.Fprintf(w, "<tr id=\"port-%d\">", port.ID); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(w, "<td class=\"mono\">%d/%s</td>", port.PortNumber, html.EscapeString(port.Protocol)); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(port.State)); err != nil {
					return err
				}
				service := strings.TrimSpace(strings.Join([]string{port.Service, port.Product, port.Version, port.ExtraInfo}, " "))
				if _, err := fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(strings.TrimSpace(service))); err != nil {
					return err
				}
				if port.State == "open" {
					if _, err := fmt.Fprintf(w, "<td><form method=\"post\" class=\"status-form\" action=\"/projects/%d/hosts/%d/ports/%d/status\"><select name=\"status\">", project.ID, host.ID, port.ID); err != nil {
						return err
					}
					for _, opt := range []struct {
						Value string
						Label string
					}{
						{"scanned", "Scanned"},
						{"flagged", "Flagged"},
						{"in_progress", "In progress"},
						{"done", "Done"},
						{"parking_lot", "Parking lot"},
					} {
						selected := ""
						if port.WorkStatus == opt.Value {
							selected = " selected"
						}
						if _, err := fmt.Fprintf(w, "<option value=\"%s\"%s>%s</option>", opt.Value, selected, opt.Label); err != nil {
							return err
						}
					}
					if _, err := io.WriteString(w, "</select><button type=\"submit\">Update</button></form></td>"); err != nil {
						return err
					}
				} else {
					if _, err := fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(port.WorkStatus)); err != nil {
						return err
					}
				}
				if port.State == "open" {
					if _, err := fmt.Fprintf(w, "<td><form method=\"post\" class=\"notes-form\" action=\"/projects/%d/hosts/%d/ports/%d/notes\"><textarea name=\"notes\" rows=\"2\" placeholder=\"Port notes\">%s</textarea><button type=\"submit\">Save</button></form>", project.ID, host.ID, port.ID, html.EscapeString(port.Notes)); err != nil {
						return err
					}
				} else {
					if _, err := fmt.Fprintf(w, "<td class=\"muted\">%s</td>", html.EscapeString(port.Notes)); err != nil {
						return err
					}
				}
				if _, err := io.WriteString(w, "</td></tr>"); err != nil {
					return err
				}
				if port.ScriptOutput != "" {
					if _, err := fmt.Fprintf(w, "<tr class=\"script-row\"><td colspan=\"5\"><details><summary>Script output</summary><pre>%s</pre></details></td></tr>", html.EscapeString(port.ScriptOutput)); err != nil {
						return err
					}
				}
			}
		}
		if _, err := io.WriteString(w, "</tbody></table></div></section>"); err != nil {
			return err
		}

		if _, err := fmt.Fprintf(w, "<div class=\"page-actions\"><a class=\"back-link\" href=\"/projects/%d/hosts/%d/export?format=json\">Export JSON</a><a class=\"back-link\" href=\"/projects/%d/hosts/%d/export?format=csv\">Export CSV</a><a class=\"back-link\" href=\"/projects/%d/hosts\">Back to hosts</a><a class=\"back-link\" href=\"/projects/%d\">Back to dashboard</a></div>", project.ID, host.ID, project.ID, host.ID, project.ID, project.ID); err != nil {
			return err
		}
		return nil
	})
	return layout("NmapTracker - Host Detail", body)
}

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

func parseInt(value string, fallback int) int {
	val, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || val <= 0 {
		return fallback
	}
	return val
}

const layoutStyles = `<style>
:root {
  color-scheme: light;
  --bg: #f6f1e8;
  --bg-accent: #e2eef0;
  --ink: #1f262d;
  --muted: #5c6c73;
  --card: rgba(255, 255, 255, 0.78);
  --stroke: rgba(31, 38, 45, 0.12);
  --accent: #2f6f6d;
  --accent-dark: #1e4f52;
  --shadow: 0 16px 40px rgba(15, 23, 28, 0.12);
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  min-height: 100vh;
  font-family: "Iowan Old Style", "Palatino Linotype", "Book Antiqua", serif;
  color: var(--ink);
  background: radial-gradient(circle at 20% 20%, var(--bg-accent), transparent 45%),
    linear-gradient(135deg, #fbf7ef, var(--bg));
}

.shell {
  max-width: 860px;
  margin: 0 auto;
  padding: 48px 24px 72px;
  display: grid;
  gap: 24px;
}

.page-header h1 {
  margin: 8px 0 8px;
  font-size: clamp(2rem, 3vw, 2.6rem);
  letter-spacing: -0.02em;
}

.eyebrow {
  text-transform: uppercase;
  letter-spacing: 0.24em;
  font-size: 0.72rem;
  color: var(--muted);
  margin: 0;
}

.subhead {
  margin: 0;
  color: var(--muted);
  font-size: 1rem;
}

.card {
  background: var(--card);
  border: 1px solid var(--stroke);
  border-radius: 16px;
  padding: 20px 22px;
  box-shadow: var(--shadow);
  backdrop-filter: blur(6px);
}

.project-form {
  display: grid;
  gap: 10px;
}

.project-form__row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

input {
  flex: 1;
  min-width: 200px;
  border-radius: 10px;
  border: 1px solid var(--stroke);
  padding: 10px 12px;
  font-size: 1rem;
  font-family: inherit;
}

button {
  border: none;
  border-radius: 999px;
  padding: 10px 18px;
  background: var(--accent);
  color: white;
  font-size: 0.95rem;
  cursor: pointer;
  font-family: inherit;
}

button:hover {
  background: var(--accent-dark);
}

.ghost {
  background: transparent;
  border: 1px solid var(--stroke);
  color: var(--ink);
}

.ghost:hover {
  background: rgba(47, 111, 109, 0.12);
}

.project-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 12px;
}

.project-list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border: 1px solid var(--stroke);
  border-radius: 12px;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.6);
}

.project-link {
  font-size: 1.05rem;
  color: var(--ink);
  text-decoration: none;
}

.project-link:hover {
  text-decoration: underline;
}

.stats-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  margin-top: 12px;
}

.stat-label {
  margin: 0;
  font-size: 0.85rem;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.1em;
}

.stat-value {
  margin: 6px 0 0;
  font-size: 1.5rem;
}

.progress {
  margin: 0;
  color: var(--muted);
}

.page-actions {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.back-link {
  color: var(--accent);
  text-decoration: none;
  font-weight: 600;
}

.back-link:hover {
  text-decoration: underline;
}

.empty {
  margin: 0;
  color: var(--muted);
}

.filters {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
}

.filters-wrap {
  max-width: 960px;
  margin: 0 auto;
}

.filters label {
  display: grid;
  gap: 6px;
  font-size: 0.85rem;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

select {
  border-radius: 10px;
  border: 1px solid var(--stroke);
  padding: 10px 12px;
  font-size: 1rem;
  font-family: inherit;
  background: white;
}

.filter-actions {
  display: flex;
  align-items: end;
  justify-content: center;
}

.bulk-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.bulk-form {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.bulk-actions .subhead {
  margin-top: 6px;
}

.table-wrap {
  width: 100%;
  overflow-x: auto;
}

.host-table {
  width: 100%;
  border-collapse: collapse;
  min-width: 620px;
}

.host-table th,
.host-table td {
  text-align: left;
  padding: 12px 10px;
  border-bottom: 1px solid var(--stroke);
}

.host-table th {
  font-size: 0.8rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted);
}

.mono {
  font-family: "SFMono-Regular", "Fira Mono", "Source Code Pro", monospace;
}

.status-summary {
  font-size: 0.9rem;
  color: var(--muted);
}

.port-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.state-filters {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  flex-wrap: wrap;
  font-size: 0.85rem;
  color: var(--muted);
}

.state-options {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px 16px;
  border-radius: 12px;
  border: 1px solid var(--stroke);
  background: rgba(255, 255, 255, 0.6);
}

.state-action {
  padding: 12px 16px;
  border-radius: 12px;
  border: 1px solid var(--stroke);
  background: rgba(255, 255, 255, 0.8);
}

.checkbox-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.checkbox-row input {
  margin: 0;
}

.pager {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 16px;
  margin-top: 16px;
}

.pager-link {
  color: var(--accent);
  text-decoration: none;
  font-weight: 600;
}

.pager-link.disabled {
  color: var(--muted);
  cursor: default;
}

.pager-status {
  color: var(--muted);
}

.status-form {
  display: flex;
  align-items: center;
  gap: 8px;
}

.muted {
  color: var(--muted);
}

.host-meta {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  margin: 0;
}

.host-meta div {
  background: rgba(255, 255, 255, 0.6);
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid var(--stroke);
}

.host-meta dt {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--muted);
}

.host-meta dd {
  margin: 6px 0 0;
}

.notes-form {
  display: grid;
  gap: 10px;
}

textarea {
  border-radius: 10px;
  border: 1px solid var(--stroke);
  padding: 10px 12px;
  font-family: inherit;
}

.script-row td {
  background: rgba(255, 255, 255, 0.5);
}

.script-row pre {
  margin: 8px 0 0;
  white-space: pre-wrap;
}

@media (max-width: 600px) {
  .shell {
    padding: 32px 18px 48px;
  }

  .project-form__row {
    flex-direction: column;
    align-items: stretch;
  }

  button {
    width: 100%;
  }

  .filters {
    grid-template-columns: 1fr;
  }

  .filter-actions {
    align-items: stretch;
  }
}
</style>`
