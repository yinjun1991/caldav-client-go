package caldav

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yinjun1991/caldav-client-go/internal"
)

// parseCalendarFromResponse 从 WebDAV 响应中解析日历属性
func parseCalendarFromResponse(resp *internal.Response) (*Calendar, error) {
	path, err := resp.Path()
	if err != nil {
		return nil, err
	}

	var resType internal.ResourceType
	if err := resp.DecodeProp(&resType); err != nil {
		// 如果 DAV:resourcetype 属性缺失，在同步场景下我们假设这是一个日历集合
		// 这是因为同步请求通常只针对已知的日历集合路径进行
		// Apple iCloud 等服务器在同步响应中可能不返回完整的属性集合
		if !internal.IsNotFound(err) {
			return nil, err
		}
		// 继续处理，假设这是一个日历集合
	} else if !resType.Is(CalendarName) {
		return nil, nil // 不是日历集合
	}

	var desc calendarDescription
	if err := resp.DecodeProp(&desc); err != nil && !internal.IsNotFound(err) {
		return nil, err
	}

	var dispName internal.DisplayName
	if err := resp.DecodeProp(&dispName); err != nil && !internal.IsNotFound(err) {
		return nil, err
	}

	var maxResSize maxResourceSize
	if err := resp.DecodeProp(&maxResSize); err != nil && !internal.IsNotFound(err) {
		return nil, err
	}

	var supportedCompSet supportedCalendarComponentSet
	if err := resp.DecodeProp(&supportedCompSet); err != nil && !internal.IsNotFound(err) {
		return nil, err
	}

	compNames := make([]string, 0, len(supportedCompSet.Comp))
	for _, comp := range supportedCompSet.Comp {
		compNames = append(compNames, comp.Name)
	}

	var calColor calendarColor
	if err := resp.DecodeProp(&calColor); err != nil && !internal.IsNotFound(err) {
		return nil, err
	}

	var calTimezone calendarTimezone
	if err := resp.DecodeProp(&calTimezone); err != nil && !internal.IsNotFound(err) {
		return nil, err
	}

	var syncToken string
	if rawSyncToken := resp.PropStats[0].Prop.Get(internal.SyncTokenName); rawSyncToken != nil {
		if err := rawSyncToken.Decode(&syncToken); err != nil {
			return nil, err
		}
	}

	var currentUserPrivileges []string
	var privSet internal.CurrentUserPrivilegeSet
	if err := resp.DecodeProp(&privSet); err != nil && !internal.IsNotFound(err) {
		return nil, err
	}
	for _, priv := range privSet.Privileges {
		for _, raw := range priv.Raw {
			if name, ok := raw.XMLName(); ok {
				currentUserPrivileges = append(currentUserPrivileges, name.Local)
			}
		}
	}

	return &Calendar{
		Path:                  path,
		Name:                  dispName.Name,
		Description:           desc.Description,
		MaxResourceSize:       maxResSize.Size,
		SupportedComponentSet: compNames,
		Color:                 calColor.Color,
		Timezone:              calTimezone.Timezone,
		SyncToken:             syncToken,
		CurrentUserPrivileges: currentUserPrivileges,
	}, nil
}

func encodeCalendarCompReq(c *CalendarCompRequest) (*comp, error) {
	encoded := comp{Name: c.Name}

	if c.AllProps {
		encoded.Allprop = &struct{}{}
	}
	for _, name := range c.Props {
		encoded.Prop = append(encoded.Prop, prop{Name: name})
	}

	if c.AllComps {
		encoded.Allcomp = &struct{}{}
	}
	for _, child := range c.Comps {
		encodedChild, err := encodeCalendarCompReq(&child)
		if err != nil {
			return nil, err
		}
		encoded.Comp = append(encoded.Comp, *encodedChild)
	}

	return &encoded, nil
}

func encodeCalendarReq(c *CalendarCompRequest) (*internal.Prop, error) {
	compReq, err := encodeCalendarCompReq(c)
	if err != nil {
		return nil, err
	}

	expandReq := encodeExpandRequest(c.Expand)

	calDataReq := calendarDataReq{Comp: compReq, Expand: expandReq}

	getLastModReq := internal.NewRawXMLElement(internal.GetLastModifiedName, nil, nil)
	getETagReq := internal.NewRawXMLElement(internal.GetETagName, nil, nil)
	return internal.EncodeProp(&calDataReq, getLastModReq, getETagReq)
}

func encodeExpandRequest(e *CalendarExpandRequest) *expand {
	if e == nil {
		return nil
	}
	encoded := expand{
		Start: dateWithUTCTime(e.Start),
		End:   dateWithUTCTime(e.End),
	}
	return &encoded
}

func decodeCalendarObject(resp internal.Response, path string) (*CalendarObject, error) {
	var calData calendarDataResp
	if err := resp.DecodeProp(&calData); err != nil {
		// Apple iCloud 等服务器在同步响应中可能不返回 calendar-data 属性
		// 这是为了性能优化，符合 RFC 6578 的实现灵活性
		// 在这种情况下，客户端需要单独获取日历对象数据
		if !internal.IsNotFound(err) {
			return nil, err
		}
		// calendar-data 缺失时，Data 字段为 nil，客户端可以根据需要单独获取
	}

	var getLastMod internal.GetLastModified
	if err := resp.DecodeProp(&getLastMod); err != nil && !internal.IsNotFound(err) {
		return nil, err
	}

	var getETag internal.GetETag
	if err := resp.DecodeProp(&getETag); err != nil && !internal.IsNotFound(err) {
		return nil, err
	}

	var getContentLength internal.GetContentLength
	if err := resp.DecodeProp(&getContentLength); err != nil && !internal.IsNotFound(err) {
		return nil, err
	}

	return &CalendarObject{
		Path:          path,
		ModTime:       time.Time(getLastMod.LastModified),
		ContentLength: getContentLength.Length,
		ETag:          string(getETag.ETag),
		Data:          calData.Data, // 可能为 nil，表示需要单独获取
	}, nil
}

func populateCalendarObject(co *CalendarObject, h http.Header) error {
	if loc := h.Get("Location"); loc != "" {
		u, err := url.Parse(loc)
		if err != nil {
			return err
		}
		co.Path = u.Path
	}
	if etag := h.Get("ETag"); etag != "" {
		etag, err := strconv.Unquote(etag)
		if err != nil {
			return err
		}
		co.ETag = etag
	}
	if contentLength := h.Get("Content-Length"); contentLength != "" {
		n, err := strconv.ParseInt(contentLength, 10, 64)
		if err != nil {
			return err
		}
		co.ContentLength = n
	}
	if lastModified := h.Get("Last-Modified"); lastModified != "" {
		t, err := http.ParseTime(lastModified)
		if err != nil {
			return err
		}
		co.ModTime = t
	}

	return nil
}

func sameCollectionPath(a, b string) bool {
	if a == b {
		return true
	}
	return normalizeCollectionPath(a) == normalizeCollectionPath(b)
}

func normalizeCollectionPath(p string) string {
	if p == "" || p == "/" {
		return p
	}
	return strings.TrimRight(p, "/")
}

func encodeCompFilter(cf *CompFilter) (*compFilter, error) {
	encoded := compFilter{Name: cf.Name}

	if cf.IsNotDefined {
		encoded.IsNotDefined = &struct{}{}
	}

	// 添加时间范围过滤器
	if !cf.Start.IsZero() || !cf.End.IsZero() {
		encoded.TimeRange = &timeRange{
			Start: dateWithUTCTime(cf.Start),
			End:   dateWithUTCTime(cf.End),
		}
	}

	// 递归编码属性过滤器
	for _, pf := range cf.Props {
		encodedProp, err := encodePropFilter(&pf)
		if err != nil {
			return nil, err
		}
		encoded.PropFilters = append(encoded.PropFilters, *encodedProp)
	}

	// 递归编码组件过滤器
	for _, childCf := range cf.Comps {
		encodedChild, err := encodeCompFilter(&childCf)
		if err != nil {
			return nil, err
		}
		encoded.CompFilters = append(encoded.CompFilters, *encodedChild)
	}

	return &encoded, nil
}

func encodePropFilter(pf *PropFilter) (*propFilter, error) {
	encoded := propFilter{Name: pf.Name}

	if pf.IsNotDefined {
		encoded.IsNotDefined = &struct{}{}
	}

	// 添加时间范围过滤器
	if !pf.Start.IsZero() || !pf.End.IsZero() {
		encoded.TimeRange = &timeRange{
			Start: dateWithUTCTime(pf.Start),
			End:   dateWithUTCTime(pf.End),
		}
	}

	// 添加文本匹配过滤器
	if pf.TextMatch != nil {
		encoded.TextMatch = &textMatch{
			Text:            pf.TextMatch.Text,
			NegateCondition: negateCondition(pf.TextMatch.NegateCondition),
		}
	}

	// 编码参数过滤器
	for _, paramF := range pf.ParamFilter {
		encodedParam, err := encodeParamFilter(&paramF)
		if err != nil {
			return nil, err
		}
		encoded.ParamFilter = append(encoded.ParamFilter, *encodedParam)
	}

	return &encoded, nil
}

func encodeParamFilter(pf *ParamFilter) (*paramFilter, error) {
	encoded := paramFilter{Name: pf.Name}

	if pf.IsNotDefined {
		encoded.IsNotDefined = &struct{}{}
	}

	// 添加文本匹配过滤器
	if pf.TextMatch != nil {
		encoded.TextMatch = &textMatch{
			Text:            pf.TextMatch.Text,
			NegateCondition: negateCondition(pf.TextMatch.NegateCondition),
		}
	}

	return &encoded, nil
}
