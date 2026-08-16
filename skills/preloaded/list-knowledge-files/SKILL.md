---
name: list-knowledge-files
description: Use when a user asks to list, enumerate, inventory, count, or inspect all files or documents in the current or bound WeKnora knowledge base.
---

# 获取知识库文件列表

## 概述

获取当前作用域内一个知识库的完整文件列表。始终查询全部分页，不要把运行时上下文中的最近文档当作完整列表。

## 流程

1. 从当前消息的 `<runtime_context>` 读取 `<bound_knowledge_bases>`。
2. 没有绑定知识库时，提示用户先绑定知识库并停止。
3. 只绑定一个知识库时直接使用；绑定多个时列出名称，请用户选择一个并停止本轮查询。
4. 确认 `database_query` 可用；不可用时提示在智能体工具配置中开启“查询数据库”。
5. 使用下方 SQL 查询，每页 200 条，从 `OFFSET 0` 开始：

```sql
SELECT id, file_name, title, file_type, parse_status, updated_at
FROM knowledges
WHERE knowledge_base_id = '<知识库短 ID>'
ORDER BY updated_at DESC, id ASC
LIMIT 200 OFFSET <offset>
```

6. 每次把 `offset` 增加 200；返回恰好 200 条时继续，少于 200 条时结束。
7. 汇总全部页面后再输出总数和表格。

不要添加 `tenant_id` 或 `deleted_at` 条件；工具会自动注入租户、软删除和 Agent 知识范围限制。

## 输出

输出知识库名称、文件总数，以及以下 Markdown 表格：

| 文件名 | 标题 | 文件类型 | 解析状态 | 更新时间 |
|---|---|---|---|---|

文件名为空时使用标题；两者均为空时显示“未命名”。保留工具返回的时间和时区。空结果输出“当前知识库暂无文件”。

## 失败处理

- 查询失败时返回错误摘要，不编造文件。
- 后续分页失败时说明已获取条数和失败的 offset，不把部分结果称为完整列表。
- 内容较长时分段输出，不静默截断。
