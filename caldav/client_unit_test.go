package caldav

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	webdav "github.com/yinjun1991/caldav-client-go"
)

// helper to build a client against a test server
func newTestClient(ts *httptest.Server) (*Client, error) {
	return NewClient(webdav.HTTPClientWithBasicAuth(nil, "", ""), ts.URL)
}

func TestCalendarQuerySendsCorrectBody(t *testing.T) {
	var sawCorrectRoot bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Fatalf("expected REPORT, got %s", r.Method)
		}
		// Read and inspect root element
		dec := xml.NewDecoder(r.Body)
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("failed reading request body: %v", err)
			}
			if se, ok := tok.(xml.StartElement); ok {
				if se.Name.Space == "urn:ietf:params:xml:ns:caldav" && se.Name.Local == "calendar-query" {
					sawCorrectRoot = true
				}
				break
			}
		}

		// Minimal multistatus response
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusMultiStatus)
		io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">
  <d:response>
    <d:href>/cal/event1.ics</d:href>
    <d:propstat>
      <d:prop>
        <d:getlastmodified>Mon, 02 Oct 2023 12:00:00 GMT</d:getlastmodified>
        <d:getetag>"etag123"</d:getetag>
        <d:getcontentlength>42</d:getcontentlength>
        <cal:calendar-data>BEGIN:VCALENDAR\nEND:VCALENDAR</cal:calendar-data>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`)
	}))
	defer ts.Close()

	c, err := newTestClient(ts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()
	req := &CalendarQueryRequest{
		CompRequest: CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true},
		Filter:      CompFilter{Name: "VCALENDAR"},
	}
	objs, err := c.CalendarQuery(ctx, "/cal/", req)
	if err != nil {
		t.Fatalf("CalendarQuery error: %v", err)
	}
	if !sawCorrectRoot {
		t.Fatalf("server did not observe calendar-query root element")
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	if objs[0].ETag != "etag123" {
		t.Fatalf("unexpected etag: %q", objs[0].ETag)
	}
}

func TestCalendarQueryRangeBuildsTimeRange(t *testing.T) {
	var rawBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		rawBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if got := r.Header.Get("Depth"); got != "1" {
			t.Fatalf("expected Depth header 1, got %q", got)
		}

		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusMultiStatus)
		io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">
  <d:response>
    <d:href>/cal/event1.ics</d:href>
    <d:propstat>
      <d:prop>
        <d:getlastmodified>Mon, 02 Oct 2023 12:00:00 GMT</d:getlastmodified>
        <d:getetag>"etag123"</d:getetag>
        <d:getcontentlength>42</d:getcontentlength>
        <cal:calendar-data>BEGIN:VCALENDAR\nBEGIN:VEVENT\nEND:VEVENT\nEND:VCALENDAR</cal:calendar-data>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`)
	}))
	defer ts.Close()

	c, err := newTestClient(ts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	objs, err := c.CalendarQueryRange(context.Background(), "/cal/", start, end)
	if err != nil {
		t.Fatalf("CalendarQueryRange error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 calendar object, got %d", len(objs))
	}

	var reqXML reportReq
	if err := xml.Unmarshal(rawBody, &reqXML); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if reqXML.Query == nil {
		t.Fatal("expected calendar-query root in request")
	}

	compFilters := reqXML.Query.Filter.CompFilter.CompFilters
	if len(compFilters) != 1 {
		t.Fatalf("expected single VEVENT filter, got %d", len(compFilters))
	}

	tr := compFilters[0].TimeRange
	if tr == nil {
		t.Fatal("expected VEVENT time-range filter")
	}
	if got := time.Time(tr.Start); !got.Equal(start) {
		t.Fatalf("unexpected time-range start, got %s want %s", got, start)
	}
	if got := time.Time(tr.End); !got.Equal(end) {
		t.Fatalf("unexpected time-range end, got %s want %s", got, end)
	}

	if reqXML.Query.Prop == nil {
		t.Fatal("expected prop element in calendar-query")
	}

	rawCalData := reqXML.Query.Prop.Get(CalendarDataName)
	if rawCalData == nil {
		t.Fatal("expected calendar-data request")
	}

	var calReq calendarDataReq
	if err := rawCalData.Decode(&calReq); err != nil {
		t.Fatalf("decode calendar-data request: %v", err)
	}
	if calReq.Expand == nil {
		t.Fatal("expected expand element to limit recurrence set")
	}

	if got := time.Time(calReq.Expand.Start); !got.Equal(start) {
		t.Fatalf("unexpected expand start, got %s want %s", got, start)
	}
	if got := time.Time(calReq.Expand.End); !got.Equal(end) {
		t.Fatalf("unexpected expand end, got %s want %s", got, end)
	}
}

func TestCalendarQueryRangeSplitsLargeWindow(t *testing.T) {
	var (
		mu      sync.Mutex
		calls   int
		windows []struct {
			start time.Time
			end   time.Time
		}
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if got := r.Header.Get("Depth"); got != "1" {
			t.Fatalf("expected Depth header 1, got %q", got)
		}

		var reqXML reportReq
		if err := xml.Unmarshal(body, &reqXML); err != nil {
			t.Fatalf("parse request: %v", err)
		}
		tr := reqXML.Query.Filter.CompFilter.CompFilters[0].TimeRange
		if tr == nil {
			t.Fatal("expected time-range filter")
		}

		mu.Lock()
		calls++
		id := calls
		windows = append(windows, struct {
			start time.Time
			end   time.Time
		}{time.Time(tr.Start), time.Time(tr.End)})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusMultiStatus)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">
  <d:response>
    <d:href>/cal/event-%d.ics</d:href>
    <d:propstat>
      <d:prop>
        <d:getlastmodified>Mon, 02 Oct 2023 12:00:00 GMT</d:getlastmodified>
        <d:getetag>"etag-%d"</d:getetag>
        <d:getcontentlength>42</d:getcontentlength>
        <cal:calendar-data>BEGIN:VCALENDAR\nBEGIN:VEVENT\nEND:VEVENT\nEND:VCALENDAR</cal:calendar-data>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`, id, id)
	}))
	defer ts.Close()

	c, err := newTestClient(ts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)

	objs, err := c.CalendarQueryRange(context.Background(), "/cal/", start, end)
	if err != nil {
		t.Fatalf("CalendarQueryRange error: %v", err)
	}

	if calls <= 1 {
		t.Fatalf("expected multiple requests for large window, got %d", calls)
	}
	if len(objs) != calls {
		t.Fatalf("expected %d objects, got %d", calls, len(objs))
	}

	// Windows should cover the full span sequentially without gaps.
	if gotStart := windows[0].start; !gotStart.Equal(start) {
		t.Fatalf("unexpected first window start, got %s want %s", gotStart, start)
	}
	if gotEnd := windows[len(windows)-1].end; !gotEnd.Equal(end) {
		t.Fatalf("unexpected last window end, got %s want %s", gotEnd, end)
	}
	for i := 1; i < len(windows); i++ {
		prev := windows[i-1]
		curr := windows[i]
		if !curr.start.Equal(prev.end) {
			t.Fatalf("window %d does not start where previous ended (%s vs %s)", i, curr.start, prev.end)
		}
		if curr.end.Sub(curr.start) > defaultCalendarRangeWindow+time.Second {
			t.Fatalf("window %d exceeds default size", i)
		}
	}
}

func TestCalendarQueryRangeRequiresBounds(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no request should be sent when bounds are missing")
	}))
	defer ts.Close()

	c, err := newTestClient(ts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = c.CalendarQueryRange(context.Background(), "/cal/", time.Time{}, time.Time{})
	if err == nil {
		t.Fatal("expected error when both start and end are zero")
	}
}

func TestPutCalendarObjectConditionalHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != MIMEType {
			t.Fatalf("unexpected content-type: %q", ct)
		}
		// Validate If-Match quoting
		ifMatch := r.Header.Get("If-Match")
		if ifMatch != "\"abc123\"" {
			t.Fatalf("unexpected If-Match: %q", ifMatch)
		}
		w.Header().Set("ETag", "\"newtag\"")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	c, err := newTestClient(ts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()
	body := strings.NewReader("BEGIN:VCALENDAR\nEND:VCALENDAR")
	co, err := c.PutCalendarObject(ctx, "/cal/test.ics", body, &PutCalendarObjectOptions{IfMatch: "abc123"})
	if err != nil {
		t.Fatalf("PutCalendarObject error: %v", err)
	}
	if co.ETag != "newtag" {
		t.Fatalf("expected ETag newtag, got %q", co.ETag)
	}
}

func TestDeleteCalendarObjectErrorHandling(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		switch r.URL.Path {
		case "/cal/missing.ics":
			w.WriteHeader(http.StatusNotFound)
		default:
			if r.Header.Get("If-Match") == "\"wrong\"" {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer ts.Close()

	c, err := newTestClient(ts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()

	// 412 path
	err = c.DeleteCalendarObject(ctx, "/cal/a.ics", &DeleteCalendarObjectOptions{IfMatch: "wrong"})
	if err == nil || !strings.Contains(err.Error(), "precondition failed") {
		t.Fatalf("expected precondition failed error, got %v", err)
	}

	// 204 success
	if err = c.DeleteCalendarObject(ctx, "/cal/a.ics", &DeleteCalendarObjectOptions{IfMatch: "right"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 404 not found
	err = c.DeleteCalendarObject(ctx, "/cal/missing.ics", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestSyncCalendarDecoding(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Fatalf("expected REPORT, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusMultiStatus)
		io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">
  <d:sync-token>token-123</d:sync-token>
  <d:response>
    <d:href>/cal/</d:href>
    <d:propstat>
      <d:prop>
        <d:resourcetype><d:collection/><cal:calendar/></d:resourcetype>
        <d:getetag>"caletag"</d:getetag>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
  <d:response>
    <d:href>/cal/event1.ics</d:href>
    <d:propstat>
      <d:prop>
        <d:getlastmodified>Mon, 02 Oct 2023 12:00:00 GMT</d:getlastmodified>
        <d:getetag>"etag123"</d:getetag>
        <d:getcontentlength>42</d:getcontentlength>
        <cal:calendar-data>BEGIN:VCALENDAR\nBEGIN:VEVENT\nEND:VEVENT\nEND:VCALENDAR</cal:calendar-data>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`)
	}))
	defer ts.Close()

	c, err := newTestClient(ts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()
	resp, err := c.SyncCalendar(ctx, "/cal/", &SyncQuery{SyncToken: ""})
	if err != nil {
		t.Fatalf("SyncCalendar error: %v", err)
	}
	if resp.SyncToken != "token-123" {
		t.Fatalf("unexpected sync token: %q", resp.SyncToken)
	}
	if resp.Calendar == nil {
		t.Fatalf("expected calendar details for collection response")
	}
	if len(resp.Updated) != 1 {
		t.Fatalf("expected 1 updated object, got %d", len(resp.Updated))
	}
	if resp.Updated[0].ETag != "etag123" {
		t.Fatalf("unexpected updated object etag: %q", resp.Updated[0].ETag)
	}
}

func TestSyncCalendarStartTimeFilter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Fatalf("expected REPORT, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusMultiStatus)
		io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav">
  <d:sync-token>token-456</d:sync-token>
  <d:response>
    <d:href>/cal/</d:href>
    <d:propstat>
      <d:prop>
        <d:resourcetype><d:collection/><cal:calendar/></d:resourcetype>
        <d:getetag>"caletag"</d:getetag>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
  <d:response>
    <d:href>/cal/event-before.ics</d:href>
    <d:propstat>
      <d:prop>
        <d:getlastmodified>Mon, 02 Oct 2023 10:00:00 GMT</d:getlastmodified>
        <d:getetag>"before"</d:getetag>
        <d:getcontentlength>10</d:getcontentlength>
        <cal:calendar-data>BEGIN:VCALENDAR\nBEGIN:VEVENT\nEND:VEVENT\nEND:VCALENDAR</cal:calendar-data>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
  <d:response>
    <d:href>/cal/event-after.ics</d:href>
    <d:propstat>
      <d:prop>
        <d:getlastmodified>Mon, 02 Oct 2023 14:00:00 GMT</d:getlastmodified>
        <d:getetag>"after"</d:getetag>
        <d:getcontentlength>15</d:getcontentlength>
        <cal:calendar-data>BEGIN:VCALENDAR\nBEGIN:VEVENT\nEND:VEVENT\nEND:VCALENDAR</cal:calendar-data>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`)
	}))
	defer ts.Close()

	c, err := newTestClient(ts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()
	cutoff := time.Date(2023, 10, 2, 12, 0, 0, 0, time.UTC)
	resp, err := c.SyncCalendar(ctx, "/cal/", &SyncQuery{StartTime: cutoff})
	if err != nil {
		t.Fatalf("SyncCalendar error: %v", err)
	}
	if len(resp.Updated) != 1 {
		t.Fatalf("expected 1 updated object, got %d", len(resp.Updated))
	}
	if resp.Updated[0].Path != "/cal/event-after.ics" {
		t.Fatalf("unexpected object returned: %s", resp.Updated[0].Path)
	}
	if got, want := resp.Updated[0].ETag, "after"; got != want {
		t.Fatalf("unexpected etag, got %s want %s", got, want)
	}
}
