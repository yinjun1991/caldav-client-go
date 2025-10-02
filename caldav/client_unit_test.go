package caldav

import (
    "context"
    "encoding/xml"
    "io"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

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

