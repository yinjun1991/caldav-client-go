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

	// StartTime filters updated resources so only items modified at or after this time are returned.
	// Use the zero value to include all results from the server. When both SyncToken and StartTime
	// are provided, SyncCalendar ignores StartTime and relies on SyncToken for incremental syncs.
	StartTime time.Time
}

// SyncResponse contains the returned sync-token for next time
type SyncResponse struct {
	SyncToken string
	Calendar  *Calendar // 集合本身的属性
	Updated   []*CalendarObject
	Deleted   []string
}

// CalendarListSyncResult represents the result of a calendar list synchronization
type CalendarListSyncResult struct {
	// AddedCalendars contains newly added calendars
	AddedCalendars []*Calendar
	// UpdatedCalendars contains calendars that have been modified
	UpdatedCalendars []*Calendar
	// DeletedCalendars contains paths of deleted calendars
	DeletedCalendars []string
	// NextSyncToken is the sync token to use for the next synchronization
	NextSyncToken string
}

// PutCalendarObjectOptions contains options for PutCalendarObject
type PutCalendarObjectOptions struct {
	// IfMatch specifies the ETag that the resource must match for the update to succeed.
	// Used for optimistic locking when updating existing resources.
	// If specified and the resource's current ETag doesn't match, returns 412 Precondition Failed.
	IfMatch string

	// IfNoneMatch when set to "*" ensures the resource doesn't exist before creation.
	// Used to prevent accidental overwrites when creating new resources.
	// If specified as "*" and the resource exists, returns 412 Precondition Failed.
	IfNoneMatch string
}

// UpdateCalendarOptions contains options for updating Calendar properties
type UpdateCalendarOptions struct {
	// Name updates the display name of the calendar (displayname property)
	Name *string

	// Description updates the calendar description (calendar-description property)
	Description *string

	// Color updates the calendar color (calendar-color property)
	Color *string

	// Timezone updates the calendar timezone (calendar-timezone property)
	Timezone *string
}
