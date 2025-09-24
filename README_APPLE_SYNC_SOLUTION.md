# Apple Calendar 同步问题解决方案

## 问题背景

在使用 go-webdav 库与 Apple iCloud CalDAV 服务进行同步时，发现 `SyncCalendar` 方法虽然能够返回事件列表，但事件对象的 `Data` 字段为空，无法获取到实际的 iCalendar 数据。

## 根本原因分析

### Apple iCloud CalDAV 的优化策略

Apple iCloud CalDAV 服务对 `sync-collection` REPORT 请求进行了性能优化：

1. **元数据优先返回**：`sync-collection` 只返回事件的元数据（路径、ETag等），不包含完整的 iCalendar 数据
2. **减少网络传输**：避免在同步过程中传输大量的事件内容数据
3. **提升响应速度**：客户端可以快速获知哪些事件发生了变化

### 业内最佳实践

主流日历应用（Google Calendar、Outlook、Apple Calendar）都采用类似的两阶段同步策略：

1. **发现阶段**：使用 `sync-collection` 获取变更的事件列表
2. **获取阶段**：使用 `calendar-multiget` 或 `calendar-query` 获取具体事件数据

## 解决方案实现

### 1. 添加通用 Report 方法

为 `internal.Client` 添加了通用的 `Report` 方法，支持各种 REPORT 操作：

```go
func (c *Client) Report(ctx context.Context, path string, depth Depth, query interface{}) (*http.Response, error)
```

### 2. 实现 CalendarMultiget 方法

支持批量获取指定路径的事件数据：

```go
func (c *Client) CalendarMultiget(ctx context.Context, paths []string, req *CalendarCompRequest) ([]CalendarObject, error)
```

**使用场景**：
- 根据 `SyncCalendar` 返回的事件路径批量获取事件数据
- 获取特定事件的完整 iCalendar 数据

### 3. 实现 CalendarQuery 方法

支持按条件查询事件：

```go
func (c *Client) CalendarQuery(ctx context.Context, calendar string, query *CalendarQueryRequest) ([]CalendarObject, error)
```

**使用场景**：
- 按时间范围查询事件
- 按事件类型过滤（VEVENT、VTODO等）
- 复杂条件查询

### 4. 完整的同步策略

推荐的 Apple Calendar 同步流程：

```go
// 1. 使用 SyncCalendar 发现变更
syncResp, err := client.SyncCalendar(ctx, calendarPath, &caldav.SyncQuery{
    SyncToken: lastSyncToken,
    Limit:     100,
})

// 2. 检查是否有事件数据
hasEventData := false
for _, obj := range syncResp.Updated {
    if len(obj.Data) > 0 {
        hasEventData = true
        break
    }
}

// 3. 如果没有事件数据，使用 CalendarMultiget 获取
if !hasEventData && len(syncResp.Updated) > 0 {
    paths := make([]string, len(syncResp.Updated))
    for i, obj := range syncResp.Updated {
        paths[i] = obj.Path
    }
    
    events, err := client.CalendarMultiget(ctx, paths, &caldav.CalendarCompRequest{
        Name:     "VCALENDAR",
        AllProps: true,
        AllComps: true,
    })
}

// 4. 或者使用 CalendarQuery 按时间范围查询
query := &caldav.CalendarQueryRequest{
    CompRequest: caldav.CalendarCompRequest{
        Name:     "VCALENDAR",
        AllProps: true,
        AllComps: true,
    },
    Filter: caldav.CompFilter{
        Name: "VCALENDAR",
        Comps: []caldav.CompFilter{
            {
                Name:  "VEVENT",
                Start: startTime,
                End:   endTime,
            },
        },
    },
}

events, err := client.CalendarQuery(ctx, calendarPath, query)
```

## 技术实现细节

### 1. XML 编码支持

实现了完整的 CalDAV 过滤器编码：

- `encodeCompFilter`：组件过滤器编码
- `encodePropFilter`：属性过滤器编码  
- `encodeParamFilter`：参数过滤器编码

### 2. 时间范围处理

支持 RFC 3339 格式的时间范围查询：

```go
type CompFilter struct {
    Name  string
    Start time.Time  // 开始时间
    End   time.Time  // 结束时间
    // ... 其他字段
}
```

### 3. 错误处理

- 网络错误重试机制
- HTTP 状态码检查
- XML 解析错误处理

## 性能优化建议

### 1. 批量操作

- 使用 `CalendarMultiget` 批量获取事件，减少网络请求次数
- 合理设置批次大小（建议 50-100 个事件）

### 2. 增量同步

- 保存并使用 `sync-token` 进行增量同步
- 定期进行全量同步以确保数据一致性

### 3. 缓存策略

- 缓存事件数据减少重复请求
- 使用 ETag 进行条件请求

## 测试验证

### 单元测试

```bash
go test ./caldav -v
```

### 集成测试

使用 `examples/apple_calendar_sync.go` 进行完整的同步流程测试：

```bash
export APPLE_CALDAV_USERNAME="your_username"
export APPLE_CALDAV_PASSWORD="your_app_password"
go run examples/apple_calendar_sync.go
```

## 兼容性说明

### 支持的 CalDAV 服务

- ✅ Apple iCloud CalDAV
- ✅ Google Calendar CalDAV  
- ✅ Microsoft Exchange CalDAV
- ✅ 标准 CalDAV 服务器

### RFC 规范遵循

- RFC 4791: CalDAV
- RFC 6638: CalDAV Scheduling Extensions
- RFC 5545: iCalendar

## 总结

通过实现 `CalendarMultiget` 和 `CalendarQuery` 方法，完美解决了 Apple Calendar 同步不返回事件数据的问题。该解决方案：

1. **符合 CalDAV 规范**：严格按照 RFC 4791 实现
2. **性能优化**：支持批量操作和增量同步
3. **兼容性好**：适用于各种 CalDAV 服务
4. **易于使用**：提供简洁的 API 接口

这种两阶段同步策略已成为业内标准做法，既保证了同步的完整性，又优化了网络传输效率。