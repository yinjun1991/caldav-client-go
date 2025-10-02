package caldav

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	webdav "github.com/yinjun1991/caldav-client-go"
	"github.com/yinjun1991/caldav-client-go/internal"
)

const MIMEType = "text/calendar"

// DiscoverContextURL performs a DNS-based CardDAV service discovery as
// described in RFC 6352 section 11. It returns the URL to the CardDAV server.
func DiscoverContextURL(ctx context.Context, domain string) (string, error) {
	return internal.DiscoverContextURL(ctx, "caldav", domain)
}

// Client provides access to a remote CardDAV server.
type Client struct {
	*webdav.Client

	ic *internal.Client
}

func NewClient(c webdav.HTTPClient, endpoint string) (*Client, error) {
	wc, err := webdav.NewClient(c, endpoint)
	if err != nil {
		return nil, err
	}
	ic, err := internal.NewClient(c, endpoint)
	if err != nil {
		return nil, err
	}
	return &Client{wc, ic}, nil
}

func (c *Client) FindCalendarHomeSet(ctx context.Context, principal string) (string, error) {
	propfind := internal.NewPropNamePropFind(CalendarHomeSetName)
	resp, err := c.ic.PropFindFlat(ctx, principal, propfind)
	if err != nil {
		return "", err
	}

	var prop calendarHomeSet
	if err := resp.DecodeProp(&prop); err != nil {
		return "", err
	}

	return prop.Href.Path, nil
}

func (c *Client) FindCalendars(ctx context.Context, calendarHomeSet string) ([]Calendar, error) {
	ms, err := c.ic.PropFind(ctx, calendarHomeSet, internal.DepthOne, calendarPropFind)
	if err != nil {
		return nil, err
	}

	l := make([]Calendar, 0, len(ms.Responses))
	for _, resp := range ms.Responses {
		calendar, err := parseCalendarFromResponse(&resp)
		if err != nil {
			return nil, err
		}
		if calendar == nil {
			continue // 不是日历集合，跳过
		}

		// 验证 MaxResourceSize 的有效性
		if calendar.MaxResourceSize < 0 {
			return nil, fmt.Errorf("caldav: max-resource-size must be a positive integer")
		}

		l = append(l, *calendar)
	}

	return l, nil
}

// GetCalendar retrieves the properties of a single Calendar collection.
// This method performs a PROPFIND request on the specified calendar path
// to fetch its current properties.
//
// The path parameter should be the full path to a calendar collection,
// not a calendar home set path.
func (c *Client) GetCalendar(ctx context.Context, path string) (*Calendar, error) {
	// Use DepthZero to query only the specified calendar collection
	resp, err := c.ic.PropFindFlat(ctx, path, calendarPropFind)
	if err != nil {
		return nil, fmt.Errorf("caldav: failed to get calendar properties: %w", err)
	}

	calendar, err := parseCalendarFromResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("caldav: failed to parse calendar response: %w", err)
	}

	if calendar == nil {
		return nil, fmt.Errorf("caldav: resource at path %s is not a calendar collection", path)
	}

	// 验证 MaxResourceSize 的有效性
	if calendar.MaxResourceSize < 0 {
		return nil, fmt.Errorf("caldav: max-resource-size must be a positive integer")
	}

	return calendar, nil
}

func (c *Client) GetCalendarObject(ctx context.Context, path string) (*CalendarObject, error) {
	req, err := c.ic.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", MIMEType)

	resp, err := c.ic.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(mediaType, MIMEType) {
		return nil, fmt.Errorf("caldav: expected Content-Type %q, got %q", MIMEType, mediaType)
	}

	// 读取响应体数据
	bodyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	co := &CalendarObject{
		Path: resp.Request.URL.Path,
		Data: bodyData,
	}
	if err := populateCalendarObject(co, resp.Header); err != nil {
		return nil, err
	}
	return co, nil
}

func (c *Client) PutCalendarObject(ctx context.Context, path string, body io.Reader, opts *PutCalendarObjectOptions) (*CalendarObject, error) {
	req, err := c.ic.NewRequest(http.MethodPut, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", MIMEType)

	// Add conditional headers for ETag-based optimistic locking
	if opts != nil {
		if opts.IfMatch != "" {
			// RFC 4791: Use If-Match for updating existing resources
			// This implements optimistic locking - the update only succeeds if the current ETag matches
			req.Header.Set("If-Match", fmt.Sprintf(`"%s"`, opts.IfMatch))
		}
		if opts.IfNoneMatch != "" {
			// RFC 4791: Use If-None-Match for creating new resources
			// Setting to "*" ensures the resource doesn't exist before creation
			if opts.IfNoneMatch == "*" {
				req.Header.Set("If-None-Match", "*")
			} else {
				req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, opts.IfNoneMatch))
			}
		}
	}

	resp, err := c.ic.Do(req.WithContext(ctx))
	if err != nil {
		// internal.Client.Do returns *internal.HTTPError for non-2xx
		if httpErr, ok := err.(*internal.HTTPError); ok {
			if httpErr.Code == http.StatusPreconditionFailed {
				return nil, fmt.Errorf("caldav: precondition failed - resource ETag mismatch or conflict")
			}
			return nil, httpErr
		}
		return nil, err
	}
	resp.Body.Close()

	co := &CalendarObject{Path: path}
	if err := populateCalendarObject(co, resp.Header); err != nil {
		return nil, err
	}
	return co, nil
}

// PutCalendarObjectSimple provides a simple interface for PutCalendarObject without options
// This maintains backward compatibility for existing code
func (c *Client) PutCalendarObjectSimple(ctx context.Context, path string, body io.Reader) (*CalendarObject, error) {
	return c.PutCalendarObject(ctx, path, body, nil)
}

// DeleteCalendarObjectOptions contains options for deleting calendar objects
type DeleteCalendarObjectOptions struct {
	// IfMatch specifies the ETag that must match for the delete to succeed.
	// This implements optimistic locking - the delete only succeeds if the current ETag matches.
	// Used to prevent accidental deletion of modified resources.
	// If specified and the ETag doesn't match, returns 412 Precondition Failed.
	IfMatch string
}

// DeleteCalendarObject deletes a calendar object (event, todo, etc.) from the server.
// This method follows CalDAV RFC 4791 specifications for resource deletion.
//
// The path parameter should be the full path to the calendar object to delete.
// The opts parameter can be used to specify conditional deletion based on ETag.
//
// Returns an error if the deletion fails, including:
// - 404 Not Found if the resource doesn't exist
// - 412 Precondition Failed if IfMatch ETag doesn't match
// - Other HTTP errors from the server
func (c *Client) DeleteCalendarObject(ctx context.Context, path string, opts *DeleteCalendarObjectOptions) error {
	req, err := c.ic.NewRequest(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	// Add conditional headers for ETag-based optimistic locking
	if opts != nil && opts.IfMatch != "" {
		// RFC 4791: Use If-Match for conditional deletion
		// This implements optimistic locking - the delete only succeeds if the current ETag matches
		// This prevents accidental deletion of resources that have been modified by others
		req.Header.Set("If-Match", fmt.Sprintf(`"%s"`, opts.IfMatch))
	}

	resp, err := c.ic.Do(req.WithContext(ctx))
	if err != nil {
		if httpErr, ok := err.(*internal.HTTPError); ok {
			switch httpErr.Code {
			case http.StatusPreconditionFailed:
				return fmt.Errorf("caldav: precondition failed - resource ETag mismatch, resource may have been modified")
			case http.StatusNotFound:
				return fmt.Errorf("caldav: calendar object not found at path: %s", path)
			default:
				return httpErr
			}
		}
		return err
	}
	resp.Body.Close()

	return nil
}

// DeleteCalendarObjectSimple provides a simple interface for DeleteCalendarObject without options
// This maintains consistency with other simple methods and provides backward compatibility
func (c *Client) DeleteCalendarObjectSimple(ctx context.Context, path string) error {
	return c.DeleteCalendarObject(ctx, path, nil)
}

// SyncCalendar performs a collection synchronization operation on the
// specified resource, as defined in RFC 6578.
func (c *Client) SyncCalendar(ctx context.Context, path string, query *SyncQuery) (*SyncResponse, error) {
	var limit *internal.Limit
	if query.Limit > 0 {
		limit = &internal.Limit{NResults: uint(query.Limit)}
	}

	// 使用标准的日历组件请求，确保同步时获取完整的日程数据
	// 这符合业内最佳实践：同步操作应返回标准属性集合以保证数据一致性
	standardCompRequest := CalendarCompRequest{
		Name:     "VCALENDAR",
		AllProps: true, // 获取所有属性以确保完整性
		Comps: []CalendarCompRequest{
			{
				Name:     "VEVENT",
				AllProps: true,
			},
		},
	}

	propReq, err := encodeCalendarReq(&standardCompRequest)
	if err != nil {
		return nil, err
	}

	ms, err := c.ic.SyncCollection(ctx, path, query.SyncToken, internal.DepthOne, limit, propReq)
	if err != nil {
		return nil, err
	}

	ret := &SyncResponse{SyncToken: ms.SyncToken}
	for _, resp := range ms.Responses {
		p, err := resp.Path()
		if err != nil {
			if err, ok := err.(*internal.HTTPError); ok && err.Code == http.StatusNotFound {
				ret.Deleted = append(ret.Deleted, p)
				continue
			}
			return nil, err
		}

		// 检查是否是集合本身
		if p == path || path == fmt.Sprintf("%s/", p) || strings.TrimSuffix(p, "/") == strings.TrimSuffix(path, "/") {
			// 解析集合属性
			calendar, err := parseCalendarFromResponse(&resp)
			if err != nil {
				return nil, err
			}
			if calendar != nil {
				ret.Calendar = calendar
			}
			continue
		}

		// 使用响应的实际路径而不是集合路径
		co, err := decodeCalendarObject(resp, p)
		if err != nil {
			return nil, err
		}

		ret.Updated = append(ret.Updated, co)
	}

	return ret, nil
}

// UpdateCalendar updates the properties of a Calendar collection using PROPPATCH.
// This method follows CalDAV RFC 4791 and WebDAV RFC 4918 specifications for
// updating collection properties.
//
// Only non-nil fields in options will be updated. To remove a property,
// pass an empty string pointer.
//
// Returns the updated Calendar object on success.
func (c *Client) UpdateCalendar(ctx context.Context, path string, options *UpdateCalendarOptions) (*Calendar, error) {
	if options == nil {
		return nil, fmt.Errorf("caldav: UpdateCalendarOptions cannot be nil")
	}

	// Build PropertyUpdate request
	var propertyUpdate internal.PropertyUpdate
	var hasUpdates bool

	// Prepare Set operations for non-nil properties
	var setProp internal.Prop

	if options.Name != nil {
		hasUpdates = true
		displayName := internal.DisplayName{Name: *options.Name}
		raw, err := internal.EncodeRawXMLElement(&displayName)
		if err != nil {
			return nil, fmt.Errorf("caldav: failed to encode display name: %w", err)
		}
		setProp.Raw = append(setProp.Raw, *raw)
	}

	if options.Description != nil {
		hasUpdates = true
		desc := calendarDescription{Description: *options.Description}
		raw, err := internal.EncodeRawXMLElement(&desc)
		if err != nil {
			return nil, fmt.Errorf("caldav: failed to encode calendar description: %w", err)
		}
		setProp.Raw = append(setProp.Raw, *raw)
	}

	if options.Color != nil {
		hasUpdates = true
		color := calendarColor{Color: *options.Color}
		raw, err := internal.EncodeRawXMLElement(&color)
		if err != nil {
			return nil, fmt.Errorf("caldav: failed to encode calendar color: %w", err)
		}
		setProp.Raw = append(setProp.Raw, *raw)
	}

	if options.Timezone != nil {
		hasUpdates = true
		timezone := calendarTimezone{Timezone: *options.Timezone}
		raw, err := internal.EncodeRawXMLElement(&timezone)
		if err != nil {
			return nil, fmt.Errorf("caldav: failed to encode calendar timezone: %w", err)
		}
		setProp.Raw = append(setProp.Raw, *raw)
	}

	if !hasUpdates {
		return nil, fmt.Errorf("caldav: no properties to update")
	}

	// Add Set operation to PropertyUpdate
	propertyUpdate.Set = append(propertyUpdate.Set, internal.Set{Prop: setProp})

	// Create PROPPATCH request
	req, err := c.ic.NewXMLRequest("PROPPATCH", path, &propertyUpdate)
	if err != nil {
		return nil, fmt.Errorf("caldav: failed to create PROPPATCH request: %w", err)
	}

	// Execute PROPPATCH request
	ms, err := c.ic.DoMultiStatus(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("caldav: PROPPATCH request failed: %w", err)
	}

	// Validate response
	if len(ms.Responses) != 1 {
		return nil, fmt.Errorf("caldav: expected 1 response, got %d", len(ms.Responses))
	}

	resp := &ms.Responses[0]
	if err = resp.Err(); err != nil {
		return nil, fmt.Errorf("caldav: property update failed: %w", err)
	}

	// Check individual property update status
	for _, propstat := range resp.PropStats {
		if err = propstat.Status.Err(); err != nil {
			return nil, fmt.Errorf("caldav: property update failed: %w", err)
		}
	}

	// Fetch updated calendar to return current state
	// This follows the pattern of other update methods and ensures consistency
	updatedCalendar, err := c.GetCalendar(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("caldav: failed to fetch updated calendar: %w", err)
	}

	return updatedCalendar, nil
}

// CalendarMultiget performs a calendar-multiget REPORT request to fetch
// multiple calendar objects by their paths in a single request.
// This is more efficient than making individual GET requests for each object.
//
// The paths parameter should contain the full paths to the calendar objects.
// The comp parameter specifies which calendar components and properties to retrieve.
func (c *Client) CalendarMultiget(ctx context.Context, paths []string, comp *CalendarCompRequest) ([]*CalendarObject, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	// 构建 href 列表
	hrefs := make([]internal.Href, len(paths))
	for i, path := range paths {
		hrefs[i] = internal.Href{Path: path}
	}

	// 编码日历组件请求
	propReq, err := encodeCalendarReq(comp)
	if err != nil {
		return nil, err
	}

	// 构建 calendar-multiget 请求
	multiget := &calendarMultiget{
		Hrefs: hrefs,
		Prop:  propReq,
	}

	// 执行 REPORT 请求
	// 使用第一个路径的父目录作为请求路径
	basePath := paths[0]
	if idx := strings.LastIndex(basePath, "/"); idx > 0 {
		basePath = basePath[:idx+1]
	}

    ms, err := c.ic.Report(ctx, basePath, multiget)
    if err != nil {
        return nil, err
    }

	// 解析响应
	objects := make([]*CalendarObject, 0, len(ms.Responses))
	for _, resp := range ms.Responses {
		path, err := resp.Path()
		if err != nil {
			return nil, err
		}

		co, err := decodeCalendarObject(resp, path)
		if err != nil {
			return nil, err
		}

		objects = append(objects, co)
	}

	return objects, nil
}

// CalendarQuery performs a calendar-query REPORT request to search for
// calendar objects that match the specified filter criteria.
//
// The path parameter should be the path to a calendar collection.
// The query parameter specifies the search criteria and which properties to retrieve.
func (c *Client) CalendarQuery(ctx context.Context, path string, query *CalendarQueryRequest) ([]CalendarObject, error) {
	// 编码日历组件请求
	propReq, err := encodeCalendarReq(&query.CompRequest)
	if err != nil {
		return nil, err
	}

	// 编码过滤器
	filterReq, err := encodeCompFilter(&query.Filter)
	if err != nil {
		return nil, err
	}

	// 构建 calendar-query 请求
	calQuery := &calendarQuery{
		Prop:   propReq,
		Filter: filter{CompFilter: *filterReq},
	}

    // 执行 REPORT 请求（直接发送 calendar-query 作为根元素）
    ms, err := c.ic.Report(ctx, path, calQuery)
    if err != nil {
        return nil, err
    }

	// 解析响应
	objects := make([]CalendarObject, 0, len(ms.Responses))
	for _, resp := range ms.Responses {
		respPath, err := resp.Path()
		if err != nil {
			return nil, err
		}

		co, err := decodeCalendarObject(resp, respPath)
		if err != nil {
			return nil, err
		}

		objects = append(objects, *co)
	}

	return objects, nil
}

// ListCalendarObjects lists all calendar objects in the specified calendar collection.
// This method uses PROPFIND with Depth 1 to discover all calendar objects,
// then optionally uses CalendarMultiget to fetch their data efficiently.
//
// If fetchData is true, the method will fetch the actual calendar data for each object.
// If fetchData is false, only metadata (path, etag, etc.) will be returned.
func (c *Client) ListCalendarObjects(ctx context.Context, path string, fetchData bool) ([]*CalendarObject, error) {
	// 使用 PROPFIND 发现所有日历对象
	propfind := internal.NewPropNamePropFind(
		internal.GetETagName,
		internal.GetLastModifiedName,
		internal.GetContentLengthName,
		internal.ResourceTypeName,
	)

	ms, err := c.ic.PropFind(ctx, path, internal.DepthOne, propfind)
	if err != nil {
		return nil, err
	}

	// 收集所有日历对象路径
	var objectPaths []string
	var objects []*CalendarObject

	for _, resp := range ms.Responses {
		respPath, err := resp.Path()
		if err != nil {
			continue
		}

		// 跳过集合本身
		if respPath == path || respPath == strings.TrimSuffix(path, "/") {
			continue
		}

		// 检查是否是日历对象（通常以 .ics 结尾或没有 resourcetype）
		var resType internal.ResourceType
		if err := resp.DecodeProp(&resType); err == nil && len(resType.Raw) > 0 {
			// 如果有 resourcetype 且不为空，跳过（可能是子集合）
			continue
		}

		if fetchData {
			objectPaths = append(objectPaths, respPath)
		} else {
			// 只获取元数据
			co, err := decodeCalendarObject(resp, respPath)
			if err != nil {
				continue
			}
			objects = append(objects, co)
		}
	}

	if fetchData && len(objectPaths) > 0 {
		// 使用 CalendarMultiget 批量获取数据
		comp := &CalendarCompRequest{
			Name:     "VCALENDAR",
			AllProps: true,
			AllComps: true,
		}
		return c.CalendarMultiget(ctx, objectPaths, comp)
	}

	return objects, nil
}

// SyncCalendarList 使用 WebDAV Collection Sync 同步日历列表
// 这个方法实现了 RFC 6578 的 sync-collection REPORT，用于增量获取日历列表的变化
func (c *Client) SyncCalendarList(
	ctx context.Context,
	calendarHomeSetURL string,
	syncToken string,
) (*CalendarListSyncResult, error) {
	return c.SyncCalendarListWithLimit(ctx, calendarHomeSetURL, syncToken, 0)
}

// SyncCalendarListWithLimit 使用 WebDAV Collection Sync 同步日历列表，支持分页
// limit 参数控制返回的最大结果数，0 表示不限制
func (c *Client) SyncCalendarListWithLimit(
	ctx context.Context,
	calendarHomeSetURL string,
	syncToken string,
	limit uint,
) (*CalendarListSyncResult, error) {
	// 构建 limit 参数
	var limitPtr *internal.Limit
	if limit > 0 {
		limitPtr = &internal.Limit{NResults: limit}
	}

	// 使用 RFC 6578 的 sync-collection REPORT
	// 参数说明：
	// - path: 日历主集合路径
	// - syncToken: 上次同步的 token，空字符串表示初始同步
	// - level: 深度为 1，只同步直接子集合（日历）
	// - limit: 限制返回数量，nil 表示不限制
	// - prop: 需要获取的属性
	ms, err := c.ic.SyncCollection(ctx, calendarHomeSetURL, syncToken, internal.DepthOne, limitPtr, calendarPropFind.Prop)
	if err != nil {
		return nil, fmt.Errorf("failed to sync calendar list: %w", err)
	}

	result := &CalendarListSyncResult{
		NextSyncToken: ms.SyncToken,
	}

	// 解析响应中的日历信息
	for _, resp := range ms.Responses {
		path, err := resp.Path()
		if err != nil {
			// 如果路径解析失败，记录错误但继续处理其他响应
			continue
		}

		// 跳过日历主集合本身
		if path == calendarHomeSetURL ||
			path == strings.TrimSuffix(calendarHomeSetURL, "/") ||
			strings.TrimSuffix(path, "/") == strings.TrimSuffix(calendarHomeSetURL, "/") {
			continue
		}

		// 检查响应状态，处理删除的日历
		if respErr := resp.Err(); respErr != nil {
			if httpErr, ok := respErr.(*internal.HTTPError); ok && httpErr.Code == http.StatusNotFound {
				// 404 状态表示日历已被删除
				result.DeletedCalendars = append(result.DeletedCalendars, path)
				continue
			}
			// 其他错误继续处理，但记录日志
			continue
		}

		// 解析日历属性
		calendar, err := parseCalendarFromResponse(&resp)
		if err != nil {
			// 解析失败可能是因为资源不是日历，继续处理其他响应
			continue
		}

		// 如果不是日历集合，跳过
		if calendar == nil {
			continue
		}

		// 验证 MaxResourceSize 的有效性
		if calendar.MaxResourceSize < 0 {
			return nil, fmt.Errorf("caldav: max-resource-size must be a positive integer for calendar %s", path)
		}

		// 根据响应状态判断是新增还是更新
		// 在 sync-collection 中，所有返回的资源都被视为"更新"
		// 实际的新增/更新区分需要客户端维护状态
		// 这里我们将所有返回的日历都放入 UpdatedCalendars
		// 客户端可以根据本地状态判断是新增还是更新
		result.UpdatedCalendars = append(result.UpdatedCalendars, calendar)
	}

	return result, nil
}

// FindCurrentUserPrincipal finds the current user's principal path.
func (c *Client) FindCurrentUserPrincipal(ctx context.Context) (string, error) {
	propfind := internal.NewPropNamePropFind(internal.CurrentUserPrincipalName)

	// TODO: consider retrying on the root URI "/" if this fails, as suggested
	// by the RFC?
	resp, err := c.ic.PropFindFlat(ctx, "", propfind)
	if err != nil {
		return "", err
	}

	var prop internal.CurrentUserPrincipal
	if err := resp.DecodeProp(&prop); err != nil {
		return "", err
	}
	if prop.Unauthenticated != nil {
		return "", fmt.Errorf("webdav: unauthenticated")
	}

	return prop.Href.Path, nil
}
