// Package caldav provides a client and server CalDAV implementation.
//
// CalDAV is defined in RFC 4791.
package caldav

import (
	"time"
)

type Calendar struct {
	Path                  string
	Name                  string
	Description           string
	MaxResourceSize       int64
	SupportedComponentSet []string
	Color                 string
	Timezone              string
	SyncToken             string
	CurrentUserPrivileges []string
}

type CalendarCompRequest struct {
	Name string

	AllProps bool
	Props    []string

	AllComps bool
	Comps    []CalendarCompRequest

	Expand *CalendarExpandRequest
}

type CalendarExpandRequest struct {
	Start, End time.Time
}

// CalendarQueryRequest represents a calendar-query request
type CalendarQueryRequest struct {
	CompRequest CalendarCompRequest
	Filter      CompFilter
}

type CompFilter struct {
	Name         string
	IsNotDefined bool
	Start, End   time.Time
	Props        []PropFilter
	Comps        []CompFilter
}

type ParamFilter struct {
	Name         string
	IsNotDefined bool
	TextMatch    *TextMatch
}

type PropFilter struct {
	Name         string
	IsNotDefined bool
	Start, End   time.Time
	TextMatch    *TextMatch
	ParamFilter  []ParamFilter
}

type TextMatch struct {
	Text            string
	NegateCondition bool
}

type CalendarObject struct {
	Path          string
	ModTime       time.Time
	ContentLength int64
	ETag          string
	Data          []byte
}

// SyncQuery is the query struct represents a sync-collection request
type SyncQuery struct {
	SyncToken string
	Limit     int // <= 0 means unlimited
}

// SyncResponse contains the returned sync-token for next time
type SyncResponse struct {
	SyncToken string
	Calendar  *Calendar // 集合本身的属性
	Updated   []*CalendarObject
	Deleted   []string
}
