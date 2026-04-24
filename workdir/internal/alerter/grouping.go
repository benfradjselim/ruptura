package alerter

import (
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/internal/analyzer"
	"github.com/benfradjselim/kairo-core/pkg/models"
	"github.com/benfradjselim/kairo-core/pkg/utils"
)

const suppressionCeiling = 5 * time.Minute

// GroupingEngine provides dependency-aware alert suppression.
// When a critical alert fires on a parent service, alerts on downstream
// dependents are marked suppressed for up to suppressionCeiling.
type GroupingEngine struct {
	mu       sync.RWMutex
	topology *analyzer.TopologyAnalyzer
	groups   map[string]*models.AlertGroup // groupID → group
	// index: alert ID → groupID
	alertGroup map[string]string
}

// NewGroupingEngine creates a grouping engine that uses the topology graph.
func NewGroupingEngine(topology *analyzer.TopologyAnalyzer) *GroupingEngine {
	return &GroupingEngine{
		topology:   topology,
		groups:     make(map[string]*models.AlertGroup),
		alertGroup: make(map[string]string),
	}
}

// Classify enriches an alert with suppression and group metadata.
// It returns the enriched alert.
// Must NOT be called with Alerter.mu held; it acquires its own lock.
func (g *GroupingEngine) Classify(alert *models.Alert, activeAlerts []*models.Alert) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Only attempt suppression for non-critical alert severities
	if alert.Severity == models.SeverityCritical || alert.Severity == models.SeverityEmergency {
		// This is a potential parent alert; add to or create a group
		g.upsertGroup(alert)
		return
	}

	// Check whether a critical parent alert exists for this host's dependencies
	parentID := g.findParentAlert(alert.Host, activeAlerts)
	if parentID == "" {
		return
	}
	if pa := g.getAlertByID(activeAlerts, parentID); pa != nil {
		if time.Since(pa.CreatedAt) < suppressionCeiling {
			alert.Suppressed = true
			alert.SuppressedBy = parentID
			// Add to parent's group
			if gid, ok := g.alertGroup[parentID]; ok {
				if grp, ok := g.groups[gid]; ok {
					grp.SuppressedIDs = append(grp.SuppressedIDs, alert.ID)
					alert.GroupID = gid
				}
			}
		}
	}
}

func (g *GroupingEngine) upsertGroup(alert *models.Alert) {
	// Reuse existing group for same host if within suppressionCeiling
	for _, grp := range g.groups {
		if g.getRepresentativeHost(grp, g.alertGroup) == alert.Host &&
			time.Since(grp.CreatedAt) < suppressionCeiling {
			grp.AlertIDs = append(grp.AlertIDs, alert.ID)
			g.alertGroup[alert.ID] = grp.ID
			alert.GroupID = grp.ID
			return
		}
	}
	grp := &models.AlertGroup{
		ID:             utils.GenerateID(8),
		Representative: alert.ID,
		AlertIDs:       []string{alert.ID},
		CreatedAt:      time.Now(),
	}
	g.groups[grp.ID] = grp
	g.alertGroup[alert.ID] = grp.ID
	alert.GroupID = grp.ID
}

// GetGroups returns all alert groups.
func (g *GroupingEngine) GetGroups() []*models.AlertGroup {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*models.AlertGroup, 0, len(g.groups))
	for _, grp := range g.groups {
		cp := *grp
		out = append(out, &cp)
	}
	return out
}

// GetGroup returns a group by ID.
func (g *GroupingEngine) GetGroup(id string) (*models.AlertGroup, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	grp, ok := g.groups[id]
	if !ok {
		return nil, false
	}
	cp := *grp
	return &cp, true
}

// ExpandGroup removes suppressions for all alerts in a group, forcing them visible.
func (g *GroupingEngine) ExpandGroup(groupID string, alrt *Alerter) {
	g.mu.Lock()
	grp, ok := g.groups[groupID]
	if !ok {
		g.mu.Unlock()
		return
	}
	suppressed := make([]string, len(grp.SuppressedIDs))
	copy(suppressed, grp.SuppressedIDs)
	g.mu.Unlock()

	for _, id := range suppressed {
		alrt.mu.Lock()
		if al, ok := alrt.active[id]; ok {
			al.Suppressed = false
			al.SuppressedBy = ""
		}
		alrt.mu.Unlock()
	}
}

func (g *GroupingEngine) findParentAlert(host string, actives []*models.Alert) string {
	if g.topology == nil {
		return ""
	}
	// Check if any service that host depends on has an active critical alert
	deps := g.topology.UpstreamDeps(host)
	for _, dep := range deps {
		for _, al := range actives {
			if al.Host == dep &&
				(al.Severity == models.SeverityCritical || al.Severity == models.SeverityEmergency) &&
				al.Status == models.StatusActive {
				return al.ID
			}
		}
	}
	return ""
}

func (g *GroupingEngine) getRepresentativeHost(grp *models.AlertGroup, idx map[string]string) string {
	_ = idx
	return grp.Representative // reuse representative ID as a stable anchor; host lookup via Alerter not needed here
}

func (g *GroupingEngine) getAlertByID(alerts []*models.Alert, id string) *models.Alert {
	for _, al := range alerts {
		if al.ID == id {
			return al
		}
	}
	return nil
}
