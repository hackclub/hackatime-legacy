package v1

import (
	"strconv"
	"time"

	"github.com/hackclub/hackatime/models"
	"github.com/hackclub/hackatime/utils"
)

type HeartbeatsViewModel struct {
	Data []*HeartbeatEntry `json:"data"`
}

// Incomplete, for now, only the subset of fields is implemented
// that is actually required for the import

type HeartbeatEntry struct {
	Id               string    `json:"id"`
	Branch           string    `json:"branch"`
	Category         string    `json:"category"`
	Entity           string    `json:"entity"`
	Editor           string    `json:"editor,omitempty"`
	IsWrite          bool      `json:"is_write"`
	Language         string    `json:"language"`
	Project          string    `json:"project"`
	ProjectRootCount uint32    `json:"project_root_count,omitempty"`
	LineAdditions    uint32    `json:"line_additions,omitempty"`
	LineDeletions    uint32    `json:"line_deletions,omitempty"`
	Lines            uint32    `json:"lines"`
	LineNumber       uint32    `json:"lineno,omitempty"`
	CursorPosition   uint32    `json:"cursorpos,omitempty"`
	Dependencies     []string  `json:"dependencies,omitempty"`
	Time             float64   `json:"time"`
	Type             string    `json:"type"`
	UserId           string    `json:"user_id"`
	MachineNameId    string    `json:"machine_name_id"`
	OperatingSystem  string    `json:"operating_system"`
	UserAgentId      string    `json:"user_agent_id"`
	CreatedAt        time.Time `json:"created_at"`
}

func HeartbeatsToCompat(entries []*models.Heartbeat) []*HeartbeatEntry {
	out := make([]*HeartbeatEntry, len(entries))
	for i := 0; i < len(entries); i++ {
		out[i] = HeartbeatToCompat(entries[i])
	}
	return out
}

func HeartbeatToCompat(entry *models.Heartbeat) *HeartbeatEntry {
	opSys, editor, _ := utils.ParseUserAgent(entry.UserAgent)
	return &HeartbeatEntry{
		Id:               strconv.FormatUint(entry.ID, 10),
		Branch:           entry.Branch,
		Category:         entry.Category,
		Entity:           entry.Entity,
		Editor:           editor,
		OperatingSystem:  opSys,
		IsWrite:          entry.IsWrite,
		Language:         entry.Language,
		Project:          entry.Project,
		ProjectRootCount: entry.ProjectRootCount,
		LineAdditions:    entry.LineAdditions,
		LineDeletions:    entry.LineDeletions,
		Lines:            entry.Lines,
		LineNumber:       entry.LineNumber,
		CursorPosition:   entry.CursorPosition,
		Dependencies:     entry.Dependencies,
		Time:             float64(entry.Time.T().Unix()),
		Type:             entry.Type,
		UserId:           entry.UserID,
		MachineNameId:    entry.Machine,
		UserAgentId:      entry.UserAgent,
		CreatedAt:        entry.CreatedAt.T(),
	}
}
