// Package assistantctx builds the read-only text snapshot the AI assistant is
// grounded on, by composing the existing dashboard, brief, task and report
// services. It carries no emails, employee numbers or credentials.
package assistantctx

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/brief"
	"github.com/dua-os/dua-os-be/internal/app/dashboard"
	"github.com/dua-os/dua-os-be/internal/app/report"
	"github.com/dua-os/dua-os-be/internal/app/task"
)

const (
	maxTasks   = 5
	maxReports = 3
	maxCards   = 4
	maxText    = 80
)

type Dashboard interface {
	ForGM(ctx context.Context, ac actor.Context) (dashboard.View, error)
}

type Brief interface {
	ForGM(ctx context.Context, ac actor.Context) (brief.View, error)
}

type Tasks interface {
	List(ctx context.Context, ac actor.Context) ([]task.View, error)
}

type Reports interface {
	List(ctx context.Context, ac actor.Context) ([]report.View, error)
}

type Provider struct {
	dash    Dashboard
	brief   Brief
	tasks   Tasks
	reports Reports
}

func New(d Dashboard, b Brief, t Tasks, r Reports) *Provider {
	return &Provider{dash: d, brief: b, tasks: t, reports: r}
}

// Snapshot degrades per section: one failing source never blocks the chat.
func (p *Provider) Snapshot(ctx context.Context, ac actor.Context) (string, error) {
	var in Input
	if v, err := p.dash.ForGM(ctx, ac); err == nil {
		in.Dashboard = &v
	}
	if v, err := p.brief.ForGM(ctx, ac); err == nil {
		in.Brief = &v
	}
	if v, err := p.tasks.List(ctx, ac); err == nil {
		in.Tasks, in.TasksOK = v, true
	}
	if v, err := p.reports.List(ctx, ac); err == nil {
		in.Reports, in.ReportsOK = v, true
	}
	return Format(in), nil
}

type Input struct {
	Dashboard *dashboard.View
	Brief     *brief.View
	Tasks     []task.View
	TasksOK   bool
	Reports   []report.View
	ReportsOK bool
}

// Format renders the snapshot as compact plain text.
func Format(in Input) string {
	var b strings.Builder
	names := map[string]string{}
	if d := in.Dashboard; d != nil {
		fmt.Fprintf(&b, "Business date: %s\n", line(d.Date))
		p := d.Pulse
		fmt.Fprintf(&b, "Pulse: staff=%d, open tasks=%d, overdue tasks=%d, completed today=%d, open staff reports=%d, overloaded staff=%d, pending spend packs=%d\n",
			p.Employees, p.OpenTasks, p.OverdueTasks, p.CompletedToday, p.OpenReports, p.OverloadedStaff, p.PendingApprovals)
		for _, t := range d.Tiles {
			state := t.Status
			if t.Status != "live" {
				state = "not connected"
			}
			fmt.Fprintf(&b, "Tile %s: %s (value %d)\n", line(t.Module), state, t.Value)
		}
		if s := d.Sales; s != nil {
			if s.Status == "live" {
				fmt.Fprintf(&b, "POS day %s: live tickets=%d sales=%d returns=%d net=%d basket=%d closed=%t (recorded only)\n",
					line(s.BusinessDate), s.TicketCount, s.SaleCount, s.ReturnCount, s.Net, s.Basket, s.Closed)
			} else {
				b.WriteString("POS: not connected\n")
			}
		}
		if c := d.Cash; c != nil {
			fmt.Fprintf(&b, "Till close for %s: %s", line(c.BusinessDate), line(c.Status))
			if c.CountedAmount != nil {
				fmt.Fprintf(&b, ", counted=%d", *c.CountedAmount)
			}
			if c.ExpectedAmount != nil {
				fmt.Fprintf(&b, ", expected=%d", *c.ExpectedAmount)
			}
			fmt.Fprintf(&b, ", variance=%d\n", c.Variance)
		}
		for _, a := range d.Approvals {
			fmt.Fprintf(&b, "Spend pack: %q supplier=%q amount=%d %s status=%s version=%d evidence=%t (approve does not pay)\n",
				line(a.Title), line(a.Supplier), a.Amount, line(a.Currency), line(a.Status), a.Version, a.Evidence != "")
		}
		for _, w := range d.Workload {
			names[w.UserID] = w.Name
			fmt.Fprintf(&b, "Workload: %s (%s) open=%d overdue=%d\n", line(w.Name), line(w.Department), w.OpenTasks, w.OverdueTasks)
		}
	} else {
		b.WriteString("Dashboard data: unavailable right now.\n")
	}
	if br := in.Brief; br != nil {
		fmt.Fprintf(&b, "Brief: overdue=%d, pending decisions=%d, assigned follow-ups=%d, snoozed=%d, all clear=%t\n",
			br.Headline.OverdueTasks, br.Headline.PendingDecisions, br.Headline.AssignedFollowups, br.SnoozedCount, br.IsClear)
		if p := br.Pulse; p.WaitingPrep+p.ReadyPrep+p.ServedPrep > 0 {
			fmt.Fprintf(&b, "Prep: waiting=%d plated=%d served=%d (recorded tickets only)\n", p.WaitingPrep, p.ReadyPrep, p.ServedPrep)
		}
		for i, c := range br.Cards {
			if i == maxCards {
				break
			}
			fmt.Fprintf(&b, "Brief card: %s severity=%s count=%d %s\n", line(c.TitleKey), line(c.Severity), c.Count, line(c.Detail))
		}
	}
	writeTasks(&b, in, names)
	writeReports(&b, in)
	return strings.TrimSpace(b.String())
}

func writeTasks(b *strings.Builder, in Input, names map[string]string) {
	if !in.TasksOK {
		b.WriteString("Tasks: unavailable right now.\n")
		return
	}
	open := make([]task.View, 0, len(in.Tasks))
	for _, t := range in.Tasks {
		if t.Status != "completed" && t.Status != "cancelled" {
			open = append(open, t)
		}
	}
	sort.SliceStable(open, func(i, j int) bool {
		if open[i].IsOverdue != open[j].IsOverdue {
			return open[i].IsOverdue
		}
		return open[i].ScheduledEnd < open[j].ScheduledEnd
	})
	fmt.Fprintf(b, "Open tasks listed (%d of %d, overdue first):\n", min(len(open), maxTasks), len(open))
	for i, t := range open {
		if i == maxTasks {
			break
		}
		who := names[t.AssigneeUserID]
		if who == "" {
			who = "unassigned"
		}
		fmt.Fprintf(b, "- %q for %s, due %s, status=%s, overdue=%t\n", line(t.Title), line(who), line(t.ScheduledEnd), line(t.Status), t.IsOverdue)
	}
}

func writeReports(b *strings.Builder, in Input) {
	if !in.ReportsOK {
		return
	}
	var open []report.View
	for _, r := range in.Reports {
		if r.Status != "resolved" {
			open = append(open, r)
		}
	}
	fmt.Fprintf(b, "Unresolved staff reports (%d of %d):\n", min(len(open), maxReports), len(open))
	for i, r := range open {
		if i == maxReports {
			break
		}
		fmt.Fprintf(b, "- %s %q by %s, status=%s\n", line(r.Kind), line(r.Title), line(r.ReporterName), line(r.Status))
	}
}

// line flattens user-authored text to one short, markup-free line.
func line(s string) string {
	s = strings.NewReplacer("<", "", ">", "", "\n", " ", "\r", " ").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxText {
		s = string(r[:maxText]) + "…"
	}
	return s
}
