# 通用认知判断路径

认知判断路径用于把知识空间中的非结构化证据整理为可检索、可审计的结构化判断链。数据模型不绑定质检领域；合规判断、故障诊断、准入规则、操作流程等领域均通过自定义节点类型和关系类型表达。

## 数据边界

- 每条路径同时受 `tenant_id` 和 `knowledge_base_id` 隔离，不能跨工作区或跨知识空间读取。
- `domain`、`path_type`、`schema_name` 和 `schema_version` 描述领域语义及其版本。
- `subject_*` 与 `scope_conditions` 界定事实适用对象和条件。
- `nodes` 保存有序的领域节点，`edges` 保存节点间的有向关系。
- `sources` 记录知识、分块、Wiki 页面、文档位置和证据摘录。
- `required_node_types` 由领域 schema 定义；服务端据此生成 `missing_node_types` 和完整性状态。
- `path_key` 与 `fact_hash` 由服务端计算。相同 `path_key` 下出现不同且不兼容的事实，是冲突检测的候选，而不是由写入方自行声明冲突。

【示例】质检领域可把“产品实例 → 检测属性 → 检测缺陷 → 检测方法 → 检测动作 → 检测结果 → 等级判定”配置成一种 schema。该节点集合只是领域示例，不是系统固定字段，也不会限制其他领域定义自己的路径。

## 写入和读取职责

工程化同步任务负责解析来源、校验 schema 并批量写入：

```text
POST /api/v1/knowledge-bases/{knowledge_base_id}/cognitive-paths/batch-upsert
```

该接口要求 Admin、知识空间写权限，以及 `ingest` 或 `full` API 权限。服务端会校验节点、边引用和批量大小，并强制重算规范化路径键与事实哈希。

普通用户可通过只读接口检索：

```text
GET /api/v1/knowledge-bases/{knowledge_base_id}/cognitive-paths
```

智能体仅开放 `query_cognitive_paths` 只读工具。工具自动继承智能体绑定的工作区和知识空间范围，不提供 INSERT、UPDATE 或 DELETE 能力。智能体负责提取建议、冲突分析和缺失分析；结构化数据落库仍由工程化任务完成。

## 推荐的三个智能体职责

1. 路径构建智能体：读取 Wiki、来源文档和已有路径，输出经过溯源的通用路径候选及批量写入载荷，不直接写库。
2. 事实冲突智能体：检索相同适用范围和 `path_key` 的候选事实，回读来源文档，排除适用条件或版本差异后发起冲突告警。
3. 内容缺失智能体：检索 `incomplete` 或 `unassessed` 路径，结合领域 schema 与来源文档定位缺失节点，并给出补充位置和建议。

冲突和缺失属于不同判断：应先确认候选事实是否确实互不兼容，再执行缺失检查。没有风险时，不应输出空的“风险提示”区块。
