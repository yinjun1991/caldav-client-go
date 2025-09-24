package caldav

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	webdav "github.com/yinjun1991/caldav-client-go"
)

const appleID = "<appleID>"
const appSpecificPassword = "<app specific password>"

// createAppleClient 创建连接到 Apple iCloud CalDAV 服务器的客户端
func createAppleClient() (*Client, error) {
	// Apple iCloud CalDAV 服务器地址
	endpoint := "https://caldav.icloud.com"

	// 使用基本认证创建 HTTP 客户端
	httpClient := webdav.HTTPClientWithBasicAuth(nil, appleID, appSpecificPassword)

	// 创建 CalDAV 客户端
	client, err := NewClient(httpClient, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create CalDAV client: %w", err)
	}

	return client, nil
}

/**

2025/09/24 14:56:53
--- Step 1: Finding Current User Principal ---
2025/09/24 14:56:55 Current User Principal: /1697197445/principal/
2025/09/24 14:56:55
--- Step 2: Finding Calendar Home Set ---
2025/09/24 14:56:55 Calendar Home Set: /1697197445/calendars/
2025/09/24 14:56:55
--- Step 3: Finding All Calendars ---
2025/09/24 14:56:56 Found 5 calendars:
2025/09/24 14:56:56 Calendar 1:
2025/09/24 14:56:56   Path: /1697197445/calendars/24A16B19-908D-498D-BF5A-1A95ABA65EC4/
2025/09/24 14:56:56   Name: test
2025/09/24 14:56:56   Description:
2025/09/24 14:56:56   Color: #CC73E1FF
2025/09/24 14:56:56   Supported Components: [VEVENT]
2025/09/24 14:56:56   Max Resource Size: 0
2025/09/24 14:56:56   Sync Token: HwoQEgwAAGfwCeyKEQAAAAEYARgAIhUI5pex0rzj+KoeEKbW9KrE0+TvswEoAEgA
2025/09/24 14:56:56   Current User Privileges: [read read-free-busy read-current-user-privilege-set write write-properties write-content bind unbind]
2025/09/24 14:56:56   ---
2025/09/24 14:56:56 Calendar 2:
2025/09/24 14:56:56   Path: /1697197445/calendars/78bb606f-9762-4459-8386-49d9ab7a21f4/
2025/09/24 14:56:56   Name: 提醒 ⚠️
2025/09/24 14:56:56   Description:
2025/09/24 14:56:56   Color: #B14BC9FF
2025/09/24 14:56:56   Supported Components: [VTODO]
2025/09/24 14:56:56   Max Resource Size: 0
2025/09/24 14:56:56   Sync Token: HwoQEgwAAAnbTcMSSwAAAAAYARgAIhYIspDxgaCv7KPXARCa6eGvnJTE+rwBKABIAA==
2025/09/24 14:56:56   Current User Privileges: [read read-free-busy read-current-user-privilege-set write write-properties write-content bind unbind]
2025/09/24 14:56:56   ---
2025/09/24 14:56:56 Calendar 3:
2025/09/24 14:56:56   Path: /1697197445/calendars/9E9CFA1C-52EB-4745-986A-E04FFB2BC9A7/
2025/09/24 14:56:56   Name: Home
2025/09/24 14:56:56   Description:
2025/09/24 14:56:56   Color: #1D9BF6FF
2025/09/24 14:56:56   Supported Components: [VEVENT]
2025/09/24 14:56:56   Max Resource Size: 0
2025/09/24 14:56:56   Sync Token: HwoQEgwAAGUxTr9ivwAKAAAYARgAIhYIhYmZ8e6unNqYARCjybq42q2jz6sBKABIAA==
2025/09/24 14:56:56   Current User Privileges: [read read-free-busy read-current-user-privilege-set write write-properties write-content bind unbind]
2025/09/24 14:56:56   ---
2025/09/24 14:56:56 Calendar 4:
2025/09/24 14:56:56   Path: /1697197445/calendars/home/
2025/09/24 14:56:56   Name: 个人
2025/09/24 14:56:56   Description:
2025/09/24 14:56:56   Color: #34AADCFF
2025/09/24 14:56:56   Supported Components: [VEVENT]
2025/09/24 14:56:56   Max Resource Size: 0
2025/09/24 14:56:56   Sync Token: HwoQEgwAAGe/o8G1bQADAAEYARgAIhUI0o2Fkb70wd98EPykioXCp9vwswEoAEgA
2025/09/24 14:56:56   Current User Privileges: [read read-free-busy read-current-user-privilege-set write write-properties write-content bind unbind]
2025/09/24 14:56:56   ---
2025/09/24 14:56:56 Calendar 5:
2025/09/24 14:56:56   Path: /1697197445/calendars/work/
2025/09/24 14:56:56   Name: 工作
2025/09/24 14:56:56   Description:
2025/09/24 14:56:56   Color: #999999FF
2025/09/24 14:56:56   Supported Components: [VEVENT]
2025/09/24 14:56:56   Max Resource Size: 0
2025/09/24 14:56:56   Sync Token: HwoQEgwAAGe/XTbwxwAAAAAYARgAIhMIypzdpuWrMhDw/dyM54OshIYBKABIAA==
2025/09/24 14:56:56   Current User Privileges: [read read-free-busy read-current-user-privilege-set write write-properties write-content bind unbind]

*/

func TestAppleFindCalendars(t *testing.T) {
	ctx := context.Background()

	// 创建 Apple CalDAV 客户端
	client, err := createAppleClient()
	if err != nil {
		log.Printf("Failed to create Apple client: %v", err)
		return
	}

	log.Println("=== Apple CalDAV Client Created Successfully ===")

	// 1. 查找当前用户主体 (Current User Principal)
	log.Println("\n--- Step 1: Finding Current User Principal ---")
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		log.Printf("Failed to find current user principal: %v", err)
		return
	}
	log.Printf("Current User Principal: %s", principal)

	// 2. 查找日历主集合 (Calendar Home Set)
	log.Println("\n--- Step 2: Finding Calendar Home Set ---")
	calendarHomeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		log.Printf("Failed to find calendar home set: %v", err)
		return
	}
	log.Printf("Calendar Home Set: %s", calendarHomeSet)

	// 3. 查找所有日历
	log.Println("\n--- Step 3: Finding All Calendars ---")
	calendars, err := client.FindCalendars(ctx, calendarHomeSet)
	if err != nil {
		log.Printf("Failed to find calendars: %v", err)
		return
	}

	log.Printf("Found %d calendars:", len(calendars))
	for i, cal := range calendars {
		log.Printf("Calendar %d:", i+1)
		log.Printf("  Path: %s", cal.Path)
		log.Printf("  Name: %s", cal.Name)
		log.Printf("  Description: %s", cal.Description)
		log.Printf("  Color: %s", cal.Color)
		// [VTODO]
		// [VEVENT]
		log.Printf("  Supported Components: %v", cal.SupportedComponentSet)
		log.Printf("  Max Resource Size: %d", cal.MaxResourceSize)
		log.Printf("  Sync Token: %s", cal.SyncToken)
		// [read read-free-busy read-current-user-privilege-set write write-properties write-content bind unbind]
		log.Printf("  Current User Privileges: %v", cal.CurrentUserPrivileges)
		log.Println("  ---")
	}
}

func TestAppleSyncCalendar(t *testing.T) {
	ctx := context.Background()

	// 创建 Apple CalDAV 客户端
	client, err := createAppleClient()
	if err != nil {
		log.Printf("Failed to create Apple client: %v", err)
		t.Fatal(err)
		return
	}

	log.Println("=== Testing Apple Calendar Sync ===")

	// 首先获取日历列表
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		log.Printf("Failed to find current user principal: %v", err)
		t.Fatal(err)
		return
	}

	calendarHomeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		log.Printf("Failed to find calendar home set: %v", err)
		t.Fatal(err)
		return
	}

	calendars, err := client.FindCalendars(ctx, calendarHomeSet)
	if err != nil {
		log.Printf("Failed to find calendars: %v", err)
		t.Fatal(err)
		return
	}

	if len(calendars) == 0 {
		log.Println("No calendars found to sync")
		return
	}

	// 选择第一个支持 VEVENT 的日历进行同步测试
	var calendar Calendar
	found := false
	for _, cal := range calendars {
		// 检查是否支持 VEVENT 组件
		for _, comp := range cal.SupportedComponentSet {
			if comp == "VEVENT" {
				calendar = cal
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		log.Println("No calendar supporting VEVENT found")
		return
	}

	log.Printf("Testing sync for calendar: %s (%s)", calendar.Name, calendar.Path)
	log.Printf("Supported components: %v", calendar.SupportedComponentSet)

	// 执行初始同步（不提供 sync token）
	log.Println("\n--- Initial Sync ---")
	syncQuery := &SyncQuery{
		SyncToken: "", // 空 token 表示初始同步
		Limit:     0,  // 限制返回 10 个对象
	}

	syncResp, err := client.SyncCalendar(ctx, calendar.Path, syncQuery)
	if err != nil {
		log.Printf("Failed to sync calendar: %v", err)
		t.Fatal(err)
		return
	}

	log.Printf("Initial sync completed. SyncToken: %s", syncResp.SyncToken)
	log.Printf("Updated objects: %d", len(syncResp.Updated))
	log.Printf("Deleted objects: %d", len(syncResp.Deleted))

	// 打印所有更新的对象详情
	// 首先收集需要获取数据的对象路径
	var pathsToFetch []string
	var objectsWithData []*CalendarObject

	for _, obj := range syncResp.Updated {
		if obj.Data != nil {
			objectsWithData = append(objectsWithData, obj)
		} else {
			pathsToFetch = append(pathsToFetch, obj.Path)
		}
	}

	// 使用 CalendarMultiget 批量获取缺失的数据
	var fetchedObjects []*CalendarObject
	if len(pathsToFetch) > 0 {
		log.Printf("Using CalendarMultiget to fetch %d objects with missing data", len(pathsToFetch))

		// 打印路径详情用于调试
		for i, path := range pathsToFetch {
			log.Printf("  Path %d: %s", i+1, path)
		}

		comp := &CalendarCompRequest{
			Name:     "VCALENDAR",
			AllProps: true,
			AllComps: true,
		}

		fetchedObjects, err = client.CalendarMultiget(ctx, pathsToFetch, comp)
		if err != nil {
			log.Printf("Failed to fetch objects with CalendarMultiget: %v", err)
			t.Fatal(err)
		} else {
			log.Printf("Successfully fetched %d objects with CalendarMultiget", len(fetchedObjects))
		}
	}

	// 创建路径到获取对象的映射
	fetchedMap := make(map[string]*CalendarObject)
	for i := range fetchedObjects {
		fetchedMap[fetchedObjects[i].Path] = fetchedObjects[i]
	}

	// 打印所有对象的详情
	for i, obj := range syncResp.Updated {
		log.Printf("Updated object %d: Path=%s, ETag=%s", i+1, obj.Path, obj.ETag)

		var dataToShow []byte
		var dataSource string

		if obj.Data != nil {
			dataToShow = obj.Data
			dataSource = "from sync response"
		} else if fetchedObj, found := fetchedMap[obj.Path]; found {
			dataToShow = fetchedObj.Data
			dataSource = "from CalendarMultiget"
		}

		if dataToShow != nil {
			preview := string(dataToShow)
			log.Printf("  Data preview (%s): %s", dataSource, preview)
		} else {
			log.Printf("  Data: nil (无法获取)")
		}
	}
}

func TestAppleUpdateCalendar(t *testing.T) {
	ctx := context.Background()

	// 创建 Apple CalDAV 客户端
	client, err := createAppleClient()
	if err != nil {
		log.Printf("Failed to create Apple client: %v", err)
		return
	}

	log.Println("=== Testing Apple Calendar Update ===")

	// 获取日历列表
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		log.Printf("Failed to find current user principal: %v", err)
		return
	}

	calendarHomeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		log.Printf("Failed to find calendar home set: %v", err)
		return
	}

	calendars, err := client.FindCalendars(ctx, calendarHomeSet)
	if err != nil {
		log.Printf("Failed to find calendars: %v", err)
		return
	}

	if len(calendars) == 0 {
		log.Println("No calendars found to update")
		return
	}

	// 选择第一个日历进行更新测试
	calendar := calendars[0]
	log.Printf("Testing update for calendar: %s (%s)", calendar.Name, calendar.Path)

	// 显示当前日历属性
	log.Println("\n--- Current Calendar Properties ---")
	log.Printf("Name: %s", calendar.Name)
	log.Printf("Description: %s", calendar.Description)
	log.Printf("Color: %s", calendar.Color)
	log.Printf("Timezone: %s", calendar.Timezone)

	// 准备更新选项
	newName := fmt.Sprintf("Updated Calendar - %s", time.Now().Format("15:04:05"))
	newDescription := fmt.Sprintf("Updated description at %s", time.Now().Format("2006-01-02 15:04:05"))
	newColor := "#FF5733" // 橙红色

	updateOptions := &UpdateCalendarOptions{
		Name:        &newName,
		Description: &newDescription,
		Color:       &newColor,
		// Timezone 保持不变，不更新
	}

	log.Println("\n--- Updating Calendar Properties ---")
	log.Printf("New Name: %s", newName)
	log.Printf("New Description: %s", newDescription)
	log.Printf("New Color: %s", newColor)

	// 执行更新
	updatedCalendar, err := client.UpdateCalendar(ctx, calendar.Path, updateOptions)
	if err != nil {
		log.Printf("Failed to update calendar: %v", err)
		return
	}

	// 显示更新后的属性
	log.Println("\n--- Updated Calendar Properties ---")
	log.Printf("Name: %s", updatedCalendar.Name)
	log.Printf("Description: %s", updatedCalendar.Description)
	log.Printf("Color: %s", updatedCalendar.Color)
	log.Printf("Timezone: %s", updatedCalendar.Timezone)

	// 验证更新是否成功
	log.Println("\n--- Verification ---")
	if updatedCalendar.Name == newName {
		log.Println("✓ Name updated successfully")
	} else {
		log.Printf("✗ Name update failed: expected %s, got %s", newName, updatedCalendar.Name)
	}

	if updatedCalendar.Description == newDescription {
		log.Println("✓ Description updated successfully")
	} else {
		log.Printf("✗ Description update failed: expected %s, got %s", newDescription, updatedCalendar.Description)
	}

	if updatedCalendar.Color == newColor {
		log.Println("✓ Color updated successfully")
	} else {
		log.Printf("✗ Color update failed: expected %s, got %s", newColor, updatedCalendar.Color)
	}
}

// createICalendarEvent 创建标准的 iCalendar 事件字符串
func createICalendarEvent(uid, summary, description string, startTime, endTime time.Time) string {
	now := time.Now().UTC()

	// 格式化时间为 iCalendar 格式 (YYYYMMDDTHHMMSSZ)
	formatTime := func(t time.Time) string {
		return t.UTC().Format("20060102T150405Z")
	}

	return fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test Event//EN
BEGIN:VEVENT
UID:%s
DTSTART:%s
DTEND:%s
DTSTAMP:%s
CREATED:%s
LAST-MODIFIED:%s
SUMMARY:%s
DESCRIPTION:%s
END:VEVENT
END:VCALENDAR`,
		uid,
		formatTime(startTime),
		formatTime(endTime),
		formatTime(now),
		formatTime(now),
		formatTime(now),
		summary,
		description)
}

func TestAppleUpdateEvents(t *testing.T) {
	ctx := context.Background()

	// 创建 Apple CalDAV 客户端
	client, err := createAppleClient()
	if err != nil {
		log.Printf("Failed to create Apple client: %v", err)
		return
	}

	log.Println("=== Testing Apple Calendar Events ===")

	// 获取日历列表
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		log.Printf("Failed to find current user principal: %v", err)
		return
	}

	calendarHomeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		log.Printf("Failed to find calendar home set: %v", err)
		return
	}

	calendars, err := client.FindCalendars(ctx, calendarHomeSet)
	if err != nil {
		log.Printf("Failed to find calendars: %v", err)
		return
	}

	if len(calendars) == 0 {
		log.Println("No calendars found to create events")
		return
	}

	// 选择第一个日历进行事件操作
	calendar := calendars[0]
	log.Printf("Testing events for calendar: %s (%s)", calendar.Name, calendar.Path)

	// 创建一个测试事件
	log.Println("\n--- Creating Test Event ---")

	eventUID := fmt.Sprintf("test-event-%d@example.com", time.Now().Unix())
	summary := "Test Event from Go CalDAV Client"
	description := "This is a test event created by the Go CalDAV client"

	// 设置事件时间（1小时后开始，持续1小时）
	startTime := time.Now().Add(1 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	// 生成事件路径
	eventPath := fmt.Sprintf("%s/%s.ics", strings.TrimSuffix(calendar.Path, "/"), eventUID)

	log.Printf("Event UID: %s", eventUID)
	log.Printf("Event Path: %s", eventPath)
	log.Printf("Start Time: %s", startTime.Format(time.RFC3339))
	log.Printf("End Time: %s", endTime.Format(time.RFC3339))

	// 创建 iCalendar 数据
	calData := createICalendarEvent(eventUID, summary, description, startTime, endTime)

	// 创建事件
	calendarObject, err := client.PutCalendarObjectSimple(ctx, eventPath, strings.NewReader(calData))
	if err != nil {
		log.Printf("Failed to create event: %v", err)
		return
	}

	log.Printf("Event created successfully!")
	log.Printf("  Path: %s", calendarObject.Path)
	log.Printf("  ETag: %s", calendarObject.ETag)
	log.Printf("  Content Length: %d", calendarObject.ContentLength)

	// 获取刚创建的事件
	log.Println("\n--- Retrieving Created Event ---")
	retrievedEvent, err := client.GetCalendarObject(ctx, eventPath)
	if err != nil {
		log.Printf("Failed to retrieve event: %v", err)
		return
	}

	log.Printf("Retrieved event:")
	log.Printf("  Path: %s", retrievedEvent.Path)
	log.Printf("  ETag: %s", retrievedEvent.ETag)
	log.Printf("  Content Length: %d", retrievedEvent.ContentLength)
	log.Printf("  Mod Time: %s", retrievedEvent.ModTime.Format(time.RFC3339))

	// 显示事件数据的前200个字符
	dataPreview := string(retrievedEvent.Data)
	if len(dataPreview) > 200 {
		dataPreview = dataPreview[:200] + "..."
	}
	log.Printf("  Data Preview: %s", strings.ReplaceAll(dataPreview, "\n", "\\n"))

	// 更新事件
	log.Println("\n--- Updating Event ---")

	// 创建更新后的 iCalendar 数据
	updatedSummary := "Updated Test Event from Go CalDAV Client"
	updatedDescription := "This event has been updated"
	updatedCalData := createICalendarEvent(eventUID, updatedSummary, updatedDescription, startTime, endTime)

	// 使用 ETag 进行条件更新
	updateOptions := &PutCalendarObjectOptions{
		IfMatch: retrievedEvent.ETag, // 使用 ETag 进行乐观锁定
	}

	updatedCalendarObject, err := client.PutCalendarObject(ctx, eventPath, strings.NewReader(updatedCalData), updateOptions)
	if err != nil {
		log.Printf("Failed to update event: %v", err)
		return
	}

	log.Printf("Event updated successfully!")
	log.Printf("  New ETag: %s", updatedCalendarObject.ETag)
	log.Printf("  Content Length: %d", updatedCalendarObject.ContentLength)

	log.Println("\n=== Event Operations Completed ===")
}

func TestAppleDeleteEvents(t *testing.T) {
	ctx := context.Background()

	// 创建 Apple CalDAV 客户端
	client, err := createAppleClient()
	if err != nil {
		log.Printf("Failed to create Apple client: %v", err)
		return
	}

	log.Println("=== Testing Apple Calendar Event Deletion ===")

	// 获取日历列表
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		log.Printf("Failed to find current user principal: %v", err)
		return
	}

	calendarHomeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		log.Printf("Failed to find calendar home set: %v", err)
		return
	}

	calendars, err := client.FindCalendars(ctx, calendarHomeSet)
	if err != nil {
		log.Printf("Failed to find calendars: %v", err)
		return
	}

	if len(calendars) == 0 {
		log.Println("No calendars found for delete testing")
		return
	}

	// 选择第一个支持 VEVENT 的日历
	var calendar Calendar
	for _, cal := range calendars {
		for _, comp := range cal.SupportedComponentSet {
			if comp == "VEVENT" {
				calendar = cal
				break
			}
		}
		if calendar.Path != "" {
			break
		}
	}

	if calendar.Path == "" {
		log.Println("No calendar supporting VEVENT found for delete testing")
		return
	}

	log.Printf("Testing event deletion for calendar: %s (%s)", calendar.Name, calendar.Path)

	// 创建一个测试事件用于删除
	log.Println("\n--- Creating Test Event for Deletion ---")

	eventUID := fmt.Sprintf("delete-test-event-%d@go-webdav", time.Now().Unix())
	summary := "Test Event for Deletion"
	description := "This event will be deleted as part of the delete functionality test"

	// 设置事件时间（1小时后开始，持续1小时）
	startTime := time.Now().Add(1 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	// 生成事件路径和内容
	eventPath := fmt.Sprintf("%s%s.ics", calendar.Path, eventUID)
	eventData := createICalendarEvent(eventUID, summary, description, startTime, endTime)

	log.Printf("Creating event at path: %s", eventPath)

	// 创建事件
	createdEvent, err := client.PutCalendarObject(ctx, eventPath, strings.NewReader(eventData), nil)
	if err != nil {
		log.Printf("Failed to create test event: %v", err)
		return
	}

	log.Printf("Test event created successfully!")
	log.Printf("  Path: %s", createdEvent.Path)
	log.Printf("  ETag: %s", createdEvent.ETag)

	// 测试 1: 简单删除（不带条件）
	log.Println("\n--- Test 1: Simple Delete (without conditions) ---")

	// 创建另一个事件用于简单删除测试
	simpleDeleteUID := fmt.Sprintf("simple-delete-test-%d@go-webdav", time.Now().Unix())
	simpleDeletePath := fmt.Sprintf("%s%s.ics", calendar.Path, simpleDeleteUID)
	simpleDeleteData := createICalendarEvent(simpleDeleteUID, "Simple Delete Test", "Event for simple delete test", startTime, endTime)

	_, err = client.PutCalendarObject(ctx, simpleDeletePath, strings.NewReader(simpleDeleteData), nil)
	if err != nil {
		log.Printf("Failed to create simple delete test event: %v", err)
		return
	}

	// 执行简单删除
	err = client.DeleteCalendarObjectSimple(ctx, simpleDeletePath)
	if err != nil {
		log.Printf("Simple delete failed: %v", err)
	} else {
		log.Printf("Simple delete successful for path: %s", simpleDeletePath)
	}

	// 验证事件已被删除
	_, err = client.GetCalendarObject(ctx, simpleDeletePath)
	if err != nil {
		log.Printf("Verification successful: Event no longer exists (expected error: %v)", err)
	} else {
		log.Printf("WARNING: Event still exists after deletion!")
	}

	// 测试 2: 条件删除（使用正确的 ETag）
	log.Println("\n--- Test 2: Conditional Delete with Correct ETag ---")

	deleteOptions := &DeleteCalendarObjectOptions{
		IfMatch: createdEvent.ETag,
	}

	err = client.DeleteCalendarObject(ctx, createdEvent.Path, deleteOptions)
	if err != nil {
		log.Printf("Conditional delete with correct ETag failed: %v", err)
	} else {
		log.Printf("Conditional delete with correct ETag successful for path: %s", createdEvent.Path)
	}

	// 验证事件已被删除
	_, err = client.GetCalendarObject(ctx, createdEvent.Path)
	if err != nil {
		log.Printf("Verification successful: Event no longer exists (expected error: %v)", err)
	} else {
		log.Printf("WARNING: Event still exists after conditional deletion!")
	}

	// 测试 3: 条件删除（使用错误的 ETag）
	log.Println("\n--- Test 3: Conditional Delete with Wrong ETag ---")

	// 创建另一个事件用于错误 ETag 测试
	wrongETagUID := fmt.Sprintf("wrong-etag-test-%d@go-webdav", time.Now().Unix())
	wrongETagPath := fmt.Sprintf("%s%s.ics", calendar.Path, wrongETagUID)
	wrongETagData := createICalendarEvent(wrongETagUID, "Wrong ETag Test", "Event for wrong ETag test", startTime, endTime)

	wrongETagEvent, err := client.PutCalendarObject(ctx, wrongETagPath, strings.NewReader(wrongETagData), nil)
	if err != nil {
		log.Printf("Failed to create wrong ETag test event: %v", err)
		return
	}

	// 尝试使用错误的 ETag 删除
	wrongDeleteOptions := &DeleteCalendarObjectOptions{
		IfMatch: "wrong-etag-value",
	}

	err = client.DeleteCalendarObject(ctx, wrongETagEvent.Path, wrongDeleteOptions)
	if err != nil {
		log.Printf("Expected failure with wrong ETag: %v", err)
	} else {
		log.Printf("WARNING: Delete with wrong ETag should have failed but succeeded!")
	}

	// 验证事件仍然存在
	_, err = client.GetCalendarObject(ctx, wrongETagEvent.Path)
	if err != nil {
		log.Printf("WARNING: Event was deleted despite wrong ETag!")
	} else {
		log.Printf("Verification successful: Event still exists after failed conditional delete")
	}

	// 清理：删除剩余的测试事件
	log.Println("\n--- Cleanup: Deleting Remaining Test Events ---")
	err = client.DeleteCalendarObjectSimple(ctx, wrongETagEvent.Path)
	if err != nil {
		log.Printf("Cleanup failed for %s: %v", wrongETagEvent.Path, err)
	} else {
		log.Printf("Cleanup successful for %s", wrongETagEvent.Path)
	}

	// 测试 4: 删除不存在的事件
	log.Println("\n--- Test 4: Delete Non-existent Event ---")

	nonExistentPath := fmt.Sprintf("%snon-existent-event-%d@go-webdav.ics", calendar.Path, time.Now().Unix())
	err = client.DeleteCalendarObjectSimple(ctx, nonExistentPath)
	if err != nil {
		log.Printf("Expected failure for non-existent event: %v", err)
	} else {
		log.Printf("WARNING: Delete of non-existent event should have failed but succeeded!")
	}

	log.Println("\n=== Event Deletion Tests Completed ===")
}
