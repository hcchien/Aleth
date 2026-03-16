# Aleth 架構設計文件 v0.1

## 1. 整體架構概覽

```
┌──────────────────────────────────────────────────────────────┐
│                        Browser / Client                       │
│         Web App (Next.js)  │  Admin Tool  │  Mobile (future)  │
└──────────┬─────────────────────────┬──────────────────────────┘
           │ HTTPS                   │ HTTPS
           │ (公開網路)              │ (內部網路 / VPN only)
           ▼                         ▼
┌──────────────────┐      ┌──────────────────────┐
│   Next.js BFF    │      │  Admin Next.js BFF   │
│  (Server-side)   │      │    (Server-side)     │
│  RSC / Actions   │      │    RSC / Actions     │
└────────┬─────────┘      └──────────┬───────────┘
         │                           │
         │ GraphQL (內部網路)        │ GraphQL (內部網路)
         ▼                           ▼
┌──────────────────┐      ┌──────────────────────┐
│   API Gateway    │      │    Admin Gateway     │
│ (Rate Limit /    │      │  (IP Allowlist /     │
│  Auth Middleware)│      │   Admin Auth)        │
└────┬──────┬──────┘      └──────────┬───────────┘
     │      │                        │
┌────▼─┐ ┌──▼──────┐ ┌──────────────▼─┐ ┌──────────┐
│ Auth │ │Content  │ │  Admin Service │ │  Feed /  │
│ Svc  │ │  Svc    │ │                │ │ Reach Svc│
└────┬─┘ └──┬──────┘ └────────────────┘ └──────────┘
     │      │
┌────▼──────▼──────────────────────────────────────┐
│              Internal Message Bus                 │
│                  (Redis Streams)                  │
└────┬──────────────┬───────────────┬───────────────┘
     │              │               │
┌────▼────┐  ┌──────▼──────┐  ┌────▼────────┐
│  Trust  │  │   Media     │  │  Notif.     │
│ Service │  │  Service    │  │  Service    │
└────┬────┘  └──────┬──────┘  └─────────────┘
     │              │
┌────▼──────────────▼────────────────────────┐
│              Data Layer                     │
│  PostgreSQL │ Redis │ S3-compatible Object  │
└─────────────────────────────────────────────┘
```

**關鍵設計原則：GraphQL 不對外公開**

GraphQL endpoint 僅可從 Next.js BFF（Server-side）呼叫，瀏覽器無法直接存取：

- 瀏覽器 → Next.js BFF（公開） → GraphQL（內部網路）→ 各 Service
- GraphQL endpoint 綁定內部網路，外部 IP 無法連線
- 惡意查詢在 Next.js 層就被阻擋：RSC 只發送預先定義的 query，不接受使用者傳入任意 query string

---

## 2. API 層設計

### 2.1 GraphQL BFF（Backend for Frontend）

GraphQL 作為 Next.js BFF 與各後端 Service 之間的統一查詢介面，**不對外公開**。

**設計原則：**

| 層面 | 說明 |
|------|------|
| 存取來源 | 僅限 Next.js Server-side（RSC、Server Actions、API Routes），綁定內部網路 |
| Query 控制 | Next.js 只發送預先定義的 query，不透傳使用者輸入的任意 query |
| 惡意查詢防護 | 由 Next.js 層攔截，GraphQL 層不需要設定 query depth/cost 限制 |
| Introspection | 生產環境關閉，僅開發環境開啟 |
| Mutation | 透過 Next.js Server Actions 呼叫，保有 CSRF 保護 |

**資料流範例：**

```
// RSC 發送已知的 query，不接受外部輸入
async function ArticleList({ boardId }: { boardId: string }) {
  const data = await gqlClient.query({
    query: GET_BOARD_ARTICLES,   // 預先定義，非動態字串
    variables: { boardId },      // 變數經過型別驗證
  })
  return <ArticleListUI articles={data.articles} />
}

// Server Action 呼叫 mutation（含 CSRF 保護）
async function publishArticle(formData: FormData) {
  'use server'
  await gqlClient.mutate({
    mutation: PUBLISH_ARTICLE,
    variables: { ... }
  })
}
```

**GraphQL Schema 組織方式（Schema Stitching）：**

各 Service 各自維護自己的 sub-schema，由 API Gateway 層的 GraphQL Server 合併：

```
GraphQL Server（API Gateway 層）
  ├── Auth sub-schema     ← Auth Service
  ├── Content sub-schema  ← Content Service
  ├── Feed sub-schema     ← Feed/Reach Service
  └── Trust sub-schema    ← Trust Service
```

---

## 3. 服務拆分

### 3.1 Auth Service

負責所有與身份相關的邏輯。

**職責：**
- L0：Email/Password、Google OAuth、Facebook OAuth 登入
- L1：WebAuthn / Passkey 註冊與驗證
- L2：信譽評分 Stamp 收集與計算（委派給 Trust Service）
- L3：VC Challenge 生成與簽章驗證
- JWT / Session token 發行與撤銷
- DID Document 管理

**GraphQL Schema（供 API Gateway 合併）：**
```graphql
type Mutation {
  register(input: RegisterInput!): AuthPayload!
  login(input: LoginInput!): AuthPayload!
  registerPasskey(input: PasskeyRegisterInput!): PasskeyPayload!
  authenticatePasskey(input: PasskeyAuthInput!): AuthPayload!
  requestVcChallenge: VcChallenge!
  verifyVc(input: VcVerifyInput!): TrustLevelPayload!
  refreshToken: AuthPayload!
  revokeToken: Boolean!
}

type Query {
  didDocument(did: String!): DIDDocument
}
```

### 3.2 Content Service

負責所有內容的 CRUD 與存取控制。

**職責：**
- 討論文（Thread Post）的發布、回覆、轉發
- 長文（Article）的草稿、發布、版本管理
- 個版（Personal Board）設定與權限管理
- 內容簽署資料的儲存與查詢
- 存取控制決策（依個版與文章權限設定）

**GraphQL Schema（供 API Gateway 合併）：**
```graphql
type Query {
  post(id: ID!): Post
  article(id: ID, slug: String): Article
  board(username: String!): Board
  boardSubscribers(boardId: ID!): [User!]!
}

type Mutation {
  createPost(input: CreatePostInput!): Post!
  deletePost(id: ID!): Boolean!
  replyPost(postId: ID!, input: CreatePostInput!): Post!
  repostPost(postId: ID!): Post!
  createArticle(input: CreateArticleInput!): Article!
  updateArticle(id: ID!, input: UpdateArticleInput!): Article!
  deleteArticle(id: ID!): Boolean!
  updateBoardSettings(input: BoardSettingsInput!): Board!
  subscribeBoardBoard(boardId: ID!): Boolean!
  unsubscribeBoard(boardId: ID!): Boolean!
}
```

### 3.3 Feed / Reach Service

負責首頁動態牆的組成與傳播分數計算。

**職責：**
- 計算每則內容的傳播分數（Reach Score）
- 組合使用者的個人化 feed
- 處理按讚、分享、引用等互動事件，更新互動加權分數
- 防洗讚邏輯

**GraphQL Schema（供 API Gateway 合併）：**
```graphql
type Query {
  feed(after: String, limit: Int): FeedConnection!
  exploreFeed(after: String, limit: Int): FeedConnection!
  reachStats(contentId: ID!, contentType: ContentType!): ReachStats!
}

type Mutation {
  recordInteraction(input: InteractionInput!): Interaction!
}
```

### 3.4 Trust Service

負責 L2 信譽評分的所有邏輯。

**職責：**
- 整合各 Stamp 來源（GitHub、Twitter、LinkedIn 等）
- 計算並儲存使用者的信譽分數
- 定期重新計算（排程任務）
- 提供評分明細查詢

**GraphQL Schema（供 API Gateway 合併）：**
```graphql
type Query {
  trustScore(userId: ID!): TrustScore!
  trustStamps(userId: ID!): [TrustStamp!]!
}

type Mutation {
  addTrustStamp(input: AddStampInput!): TrustStamp!
  removeTrustStamp(stampId: ID!): Boolean!
}
```

### 3.5 Media Service

負責圖片、影片等媒體資源的上傳與分發。

**職責：**
- 接收上傳、產生預設尺寸縮圖
- 儲存至 Object Storage（S3-compatible）
- 回傳 CDN 存取 URL

### 3.6 Notification Service

負責即時通知與電子郵件通知。

**職責：**
- 訂閱/追蹤的新文章通知
- 回覆、提及、轉發通知
- 透過 WebSocket 推送即時通知
- 寄送 Email 通知（訂閱週報等）

### 3.7 Counter Service

負責維護 `posts` 與 `articles` 資料表上的 denormalized 計數欄位，讓 feed 查詢不需要做 `COUNT(*)` JOIN。

**職責：**
- 訂閱 GCP Pub/Sub 的 content 事件
- 對對應資料列執行原子性的 `UPDATE ... SET count = count ± 1`

**Counter 對應關係：**

| 事件 | 條件 | 更新目標 |
|------|------|---------|
| `post.created` | `kind = "reply"` | `posts.comment_count += 1`（對 `parent_id`）|
| `comment.created` | — | `articles.comment_count += 1`（對 `article_id`）|
| `reaction.upserted` | — | `posts.reaction_count += 1`（對 `post_id`）|
| `reaction.removed` | — | `posts.reaction_count -= 1`（floor 0，對 `post_id`）|

**設計取捨：最終一致性**

Counter 透過 Pub/Sub 非同步更新，與寫入事件之間存在短暫延遲（正常負載下 < 1 秒）。這對社交網路的顯示需求是可接受的。

選擇此方案而非 Application 層同步更新（Transaction 內同時寫 counter），是因為：
1. Counter Service 可獨立部署與擴展，不增加 Content Service 的 Transaction 範圍
2. 延遲可接受，feed 頁面顯示的計數不需要強一致
3. Pub/Sub 的 at-least-once 投遞語意下，重複投遞最多造成 ±1 的暫時漂移，由每日 reconciliation job 修正

**漂移修正（Reconciliation Job）：**

每日執行一次全量校正，消除因 Pub/Sub 重複投遞或服務重啟期間的漂移：

```sql
-- 校正 posts.comment_count
UPDATE posts p
SET comment_count = (
    SELECT COUNT(*) FROM posts r
    WHERE r.parent_id = p.id AND r.deleted_at IS NULL
);

-- 校正 posts.reaction_count
UPDATE posts p
SET reaction_count = (
    SELECT COUNT(*) FROM reactions r WHERE r.post_id = p.id
);

-- 校正 articles.comment_count
UPDATE articles a
SET comment_count = (
    SELECT COUNT(*) FROM article_comments c WHERE c.article_id = a.id
);
```

### 3.8 Admin Service

負責後台管理功能，僅供具備管理員權限的帳號使用，透過獨立的 Admin Gateway 進入，與一般使用者流量完全隔離。

**職責：**
- 使用者帳號管理（查詢、停權、解除停權、刪除）
- 信任等級手動覆寫（例如撤銷已驗證的 L3）
- 內容管理（下架、強制刪除、標記違規）
- 檢舉案件審核佇列
- 系統公告發布
- 平台數據儀表板（DAU、內容量、各等級使用者分布等）
- 操作稽核日誌查詢（Audit Log）

**GraphQL Schema（供 Admin Gateway 使用，不合併至主站 API Gateway）：**
```graphql
type Query {
  adminUsers(filter: AdminUserFilterInput, after: String, limit: Int): AdminUserConnection!
  adminUser(id: ID!): AdminUserDetail
  adminContents(filter: AdminContentFilterInput, after: String, limit: Int): AdminContentConnection!
  adminContent(id: ID!, contentType: ContentType!): AdminContentDetail
  adminReports(status: ReportStatus, after: String, limit: Int): AdminReportConnection!
  adminReport(id: ID!): AdminReport
  adminStats: PlatformStats!
  adminAuditLogs(filter: AuditLogFilterInput, after: String, limit: Int): AuditLogConnection!
}

type Mutation {
  # 使用者管理
  suspendUser(userId: ID!, reason: String!, expiresAt: DateTime): AdminAction!
  unsuspendUser(userId: ID!): AdminAction!
  deleteUser(userId: ID!): AdminAction!                           # 需二次確認
  overrideTrustLevel(userId: ID!, level: Int!, reason: String!): AdminAction!  # 需二次確認
  revokeVcVerification(userId: ID!, reason: String!): AdminAction!             # 需二次確認

  # 內容管理
  takedownContent(contentId: ID!, contentType: ContentType!, reason: String!): AdminAction!
  deleteContent(contentId: ID!, contentType: ContentType!): AdminAction!
  restoreContent(contentId: ID!, contentType: ContentType!): AdminAction!

  # 檢舉管理
  resolveReport(reportId: ID!, action: ResolveActionInput!): AdminReport!
  dismissReport(reportId: ID!, note: String): AdminReport!

  # 系統
  createAnnouncement(input: AnnouncementInput!): Announcement!
  createAdminAccount(input: CreateAdminInput!): AdminAccount!    # super_admin 限定，需二次確認
  revokeAdminAccount(adminId: ID!): AdminAccount!                # super_admin 限定，需二次確認
}
```

**管理員角色設計：**

| 角色 | 權限範圍 |
|------|---------|
| `super_admin` | 所有操作，包含管理其他 admin 帳號 |
| `moderator` | 內容審核、檢舉處理、使用者停權 |
| `support` | 唯讀查詢使用者與內容資訊 |

**安全要求：**
- Admin 帳號強制啟用 Passkey（不接受密碼登入）
- 每次操作寫入 Audit Log（操作者、時間、對象、動作、前後狀態）
- Admin Gateway 限定內部網路 IP 或 VPN 才可存取
- 敏感操作（刪除帳號、覆寫信任等級）需要二次確認

---

## 4. 資料模型

### 4.1 User

```sql
users (
  id              UUID PRIMARY KEY,
  did             TEXT UNIQUE NOT NULL,       -- did:aleth:<id>
  username        TEXT UNIQUE NOT NULL,
  display_name    TEXT,
  email           TEXT UNIQUE,
  email_verified  BOOLEAN DEFAULT FALSE,
  trust_level     SMALLINT DEFAULT 0,         -- 0~3
  created_at      TIMESTAMPTZ,
  updated_at      TIMESTAMPTZ
)

user_credentials (
  id              UUID PRIMARY KEY,
  user_id         UUID REFERENCES users(id),
  type            TEXT,                       -- 'password' | 'google' | 'facebook' | 'passkey'
  credential_id   TEXT,                       -- passkey credential id / oauth sub
  public_key      BYTEA,                      -- passkey public key
  sign_count      BIGINT,                     -- passkey replay protection
  created_at      TIMESTAMPTZ
)

user_vc_verifications (
  id              UUID PRIMARY KEY,
  user_id         UUID REFERENCES users(id) UNIQUE,
  vc_type         TEXT,                       -- 'tw-moica' | ...
  identity_hash   TEXT UNIQUE,                -- hash(real identity)，防重複帳號，不存明文
  verified_at     TIMESTAMPTZ,
  expires_at      TIMESTAMPTZ
)

user_trust_stamps (
  id              UUID PRIMARY KEY,
  user_id         UUID REFERENCES users(id),
  stamp_type      TEXT,                       -- 'github' | 'twitter' | 'linkedin' | ...
  score           NUMERIC,
  metadata        JSONB,                      -- 不含 PII 的評分依據摘要
  verified_at     TIMESTAMPTZ,
  expires_at      TIMESTAMPTZ,
  UNIQUE(user_id, stamp_type)
)
```

### 4.2 Content

```sql
posts (
  id              UUID PRIMARY KEY,
  author_id       UUID REFERENCES users(id),
  parent_id       UUID REFERENCES posts(id),  -- 回覆用
  root_id         UUID REFERENCES posts(id),  -- thread 根節點
  content         TEXT NOT NULL,
  media_ids       UUID[],
  reach_score     NUMERIC DEFAULT 0,
  comment_count   INT NOT NULL DEFAULT 0,     -- 非同步維護，見 §3.7 Counter Service
  reaction_count  INT NOT NULL DEFAULT 0,     -- 非同步維護，見 §3.7 Counter Service
  signature       JSONB,                      -- 見 5.3 簽署資料結構
  created_at      TIMESTAMPTZ,
  deleted_at      TIMESTAMPTZ                 -- soft delete
)

articles (
  id              UUID PRIMARY KEY,
  board_id        UUID REFERENCES boards(id),
  author_id       UUID REFERENCES users(id),
  series_id       UUID REFERENCES series(id),
  title           TEXT NOT NULL,
  slug            TEXT NOT NULL,
  content_md      TEXT,
  content_json    JSONB,                      -- rich text AST
  status          TEXT,                       -- 'draft' | 'published' | 'unlisted'
  access_policy   TEXT,                       -- 'public' | 'members' | 'subscribers' | 'allowlist' | 'trust_level'
  min_trust_level SMALLINT,                   -- access_policy = 'trust_level' 時使用
  reach_score     NUMERIC DEFAULT 0,
  comment_count   INT NOT NULL DEFAULT 0,     -- 非同步維護，見 §3.7 Counter Service
  signature       JSONB,
  published_at    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ,
  updated_at      TIMESTAMPTZ
)

boards (
  id              UUID PRIMARY KEY,
  owner_id        UUID REFERENCES users(id) UNIQUE,
  name            TEXT NOT NULL,
  description     TEXT,
  cover_image_id  UUID,
  default_access  TEXT DEFAULT 'public',
  min_trust_level SMALLINT DEFAULT 0,
  comment_policy  TEXT DEFAULT 'public',      -- 'public' | 'members' | 'subscribers' | 'trust_level'
  min_comment_trust SMALLINT DEFAULT 0,
  created_at      TIMESTAMPTZ,
  updated_at      TIMESTAMPTZ
)

board_subscribers (
  board_id        UUID REFERENCES boards(id),
  user_id         UUID REFERENCES users(id),
  subscribed_at   TIMESTAMPTZ,
  PRIMARY KEY (board_id, user_id)
)

article_allowlist (
  article_id      UUID REFERENCES articles(id),
  user_id         UUID REFERENCES users(id),
  granted_at      TIMESTAMPTZ,
  PRIMARY KEY (article_id, user_id)
)

series (
  id              UUID PRIMARY KEY,
  board_id        UUID REFERENCES boards(id),
  title           TEXT NOT NULL,
  description     TEXT,
  created_at      TIMESTAMPTZ
)
```

### 4.3 Admin

```sql
admin_accounts (
  id              UUID PRIMARY KEY,
  user_id         UUID REFERENCES users(id) UNIQUE,
  role            TEXT NOT NULL,              -- 'super_admin' | 'moderator' | 'support'
  created_by      UUID REFERENCES admin_accounts(id),
  created_at      TIMESTAMPTZ,
  revoked_at      TIMESTAMPTZ
)

reports (
  id              UUID PRIMARY KEY,
  reporter_id     UUID REFERENCES users(id),
  content_type    TEXT,                       -- 'post' | 'article' | 'user'
  content_id      UUID,
  reason          TEXT,                       -- 'spam' | 'harassment' | 'misinformation' | ...
  description     TEXT,
  status          TEXT DEFAULT 'pending',     -- 'pending' | 'resolved' | 'dismissed'
  resolved_by     UUID REFERENCES admin_accounts(id),
  resolved_at     TIMESTAMPTZ,
  created_at      TIMESTAMPTZ
)

audit_logs (
  id              UUID PRIMARY KEY,
  admin_id        UUID REFERENCES admin_accounts(id),
  action          TEXT NOT NULL,              -- 'suspend_user' | 'takedown_content' | ...
  target_type     TEXT,                       -- 'user' | 'post' | 'article' | 'report'
  target_id       UUID,
  before_state    JSONB,
  after_state     JSONB,
  note            TEXT,
  created_at      TIMESTAMPTZ
)

user_suspensions (
  id              UUID PRIMARY KEY,
  user_id         UUID REFERENCES users(id),
  admin_id        UUID REFERENCES admin_accounts(id),
  reason          TEXT,
  suspended_at    TIMESTAMPTZ,
  expires_at      TIMESTAMPTZ,               -- NULL 表示永久停權
  lifted_at       TIMESTAMPTZ,
  lifted_by       UUID REFERENCES admin_accounts(id)
)
```

### 4.4 Interactions & Reach

```sql
interactions (
  id              UUID PRIMARY KEY,
  user_id         UUID REFERENCES users(id),
  content_type    TEXT,                       -- 'post' | 'article'
  content_id      UUID,
  type            TEXT,                       -- 'like' | 'repost' | 'quote' | 'share'
  weight          NUMERIC,                    -- 依 user trust_level 計算的互動加權
  created_at      TIMESTAMPTZ,
  UNIQUE(user_id, content_type, content_id, type)
)
```

---

## 5. 信任等級流程

### 5.1 L1 Passkey 升級流程

```
Client                        Auth Service                  DB
  │                                │                         │
  │── POST /auth/passkey/register ─▶                        │
  │        (challenge request)     │── generate challenge ──▶│
  │◀─────── challenge ─────────────│                         │
  │                                │                         │
  │  [使用者完成生物辨識 / PIN]     │                         │
  │                                │                         │
  │── POST /auth/passkey/register ─▶                        │
  │     (attestation response)     │── verify attestation   │
  │                                │── store credential ────▶│
  │                                │── set trust_level = 1  │
  │◀─────── 200 OK ────────────────│                         │
```

### 5.2 L2 信譽評分流程

```
Client                  Auth Service          Trust Service        外部 OAuth
  │                          │                     │                   │
  │── 選擇新增 GitHub Stamp ──▶                    │                   │
  │◀── OAuth redirect ───────│                     │                   │
  │────────────────────────────────────────────────────── OAuth flow ──▶│
  │◀──────────────────────────────────────── access_token ─────────────│
  │── POST /trust/stamps (token) ▶              │                   │
  │                          │── fetch GitHub profile ────────────────▶│
  │                          │◀─────────── profile data ───────────────│
  │                          │── calculate stamp score                 │
  │                          │── store stamp ──────▶                   │
  │                          │── recalculate total ─▶                  │
  │                          │◀── new total score ──│                   │
  │                          │  if total >= 門檻:    │                   │
  │                          │── set trust_level = 2                   │
  │◀─────── score + level ───│                     │                   │
```

### 5.3 L3 VC 驗證流程

```
Client (含自然人憑證)       Auth Service                   DB
  │                              │                          │
  │── POST /auth/vc/challenge ───▶                         │
  │◀─────── challenge nonce ─────│                          │
  │                              │                          │
  │  [用自然人憑證私鑰簽署 nonce] │                          │
  │                              │                          │
  │── POST /auth/vc/verify ───────▶                        │
  │   { vc_type, signed_nonce,   │── 1. 驗證憑證鏈          │
  │     certificate_chain }      │── 2. 驗證簽章            │
  │                              │── 3. 驗證憑證未過期       │
  │                              │── 4. hash(identity)      │
  │                              │── 5. 確認 hash 未被使用 ─▶│
  │                              │── 6. 儲存驗證記錄 ────────▶│
  │                              │── 7. set trust_level = 3 │
  │◀──── 200 OK ─────────────────│                          │
```

---

## 6. 內容存取控制

存取控制決策在 Content Service 的 middleware 層執行，不依賴前端隱藏，每次讀取都在後端驗證。

```
request(article_id, requester_token)
        │
        ▼
 解析 requester JWT
 取得 user_id, trust_level
        │
        ▼
 查詢 article.access_policy
        │
  ┌─────┴──────────────────────────────────────────────┐
  │ public      │ members  │ subscribers │ allowlist │ trust_level │
  ▼             ▼          ▼             ▼           ▼
允許所有人    需登入     查 board_    查 article_  trust_level
             (L0+)    subscribers  allowlist    >= min_trust
  └─────┬──────────────────────────────────────────────┘
        │ 通過
        ▼
   回傳內容
        │ 不通過
        ▼
   403 Forbidden（不揭露內容是否存在）
```

---

## 7. 傳播分數計算

### 7.1 計算時機

- 非同步：透過 Message Bus 接收互動事件後觸發
- 不在請求路徑上同步計算，避免延遲

### 7.2 計算邏輯

```
Reach Score 計算流程（Redis Stream Consumer）:

1. 收到 interaction_event:
   { content_id, user_id, type, user_trust_level }

2. 計算此次互動的加權值：
   interaction_weight = base_weight[type] × trust_multiplier[user_trust_level]

   base_weight:
     like   = 1.0
     repost = 3.0
     quote  = 2.0
     share  = 2.0

   trust_multiplier:
     L0 = 1.0, L1 = 1.5, L2 = 2.5, L3 = 4.0

3. 從 DB 讀取文章的 author trust_level

4. 計算最終傳播分數：
   reach_score = Σ(interaction_weights) × author_trust_multiplier × time_decay

   time_decay = e^(-λ × hours_since_publish)
   λ 建議初始值 = 0.1（約 7 小時後分數減半）

5. 寫回 DB（可接受最終一致性，使用 Redis 暫存，定期 flush）
```

### 7.3 Feed 排序

```
Feed Query（個人化首頁）:

SELECT content
FROM (
  posts + articles 作者在 following 清單內
)
ORDER BY
  (reach_score * personalization_factor) DESC,
  published_at DESC
LIMIT 20

personalization_factor:
  - 若作者在訂閱清單內 × 1.3
  - 若曾與作者互動 × 1.1
  - 冷啟動保護：新帳號文章保證最低 base_reach = 100
```

---

## 8. 前端架構

主站與 Admin Tool 統一使用 **Next.js (App Router)**，共用元件庫與型別定義，分別以獨立 Next.js 應用部署，透過不同 domain 區隔（例如 `aleth.app` 與 `admin.aleth.app`）。

### 8.1 技術選型

| 項目 | 選型 | 理由 |
|------|------|------|
| Framework | **Next.js 15 (App Router)** | RSC + PPR + Streaming，支援 component 層級 cache；SSR 利於 SEO |
| Styling | Tailwind CSS | 快速開發，客製化彈性高 |
| UI 元件庫 | shadcn/ui | 基於 Radix UI，accessible，主站與 Admin Tool 共用 |
| 狀態管理 | Zustand（client state）| 輕量，僅管理 UI 狀態；伺服器狀態由 RSC 直接處理 |
| 編輯器 | Tiptap（長文）/ 純 textarea（討論文） | Tiptap 支援 Markdown、Rich Text，可擴充 |
| GraphQL Client（BFF） | **Apollo Client**（`@apollo/client`）| Server Component 用 `getClient().query()`；Client Component 用 hooks；支援 Normalized Cache 避免重複請求 |
| WebAuthn | `@simplewebauthn/browser` | 封裝瀏覽器 WebAuthn API |
| 即時通知 | WebSocket（socket.io-client） | 低延遲推送 |

### 8.2 Next.js Rendering 策略

App Router 的多種渲染機制對應不同頁面特性：

| Component / 頁面 | 策略 | 說明 |
|-----------------|------|------|
| 長文內文 | RSC + ISR (`revalidate: 3600`) | 發布後呼叫 `revalidateTag` 精準失效 |
| 個版 Header | RSC + ISR | 使用者修改設定後失效 |
| Reach Score 顯示 | RSC + `revalidate: 30` | stale-while-revalidate，允許短暫過時 |
| 首頁 Feed | RSC + Streaming（無 cache） | 依 session 個人化，不共用 cache |
| 信任等級標記 | 隨文章資料帶回 | 不需額外請求 |
| 留言區 | Client Component + WebSocket | 即時更新 |

**Partial Prerendering（PPR）應用範例：**

```tsx
// 長文頁面：靜態殼從 CDN 立即送出，動態部分串流補上
export const experimental_ppr = true

export default function ArticlePage({ params }) {
  return (
    <>
      {/* 靜態：build time 預渲染，CDN 直送 */}
      <ArticleContent slug={params.slug} />
      <AuthorBio authorId={params.authorId} />

      {/* 動態：request time 串流 */}
      <Suspense fallback={<CommentsSkeleton />}>
        <Comments articleId={params.id} />
      </Suspense>

      <Suspense fallback={null}>
        <TrustBadge authorId={params.authorId} />
      </Suspense>
    </>
  )
}
```

**Data Cache Tag 失效範例：**

```ts
// 文章更新時，精準失效對應 component 的 cache
import { revalidateTag } from 'next/cache'

export async function onArticlePublished(boardId: string) {
  revalidateTag(`board-articles-${boardId}`)  // 只讓文章列表重新取值
  // 其他 component 的 cache 不受影響
}
```

### 8.3 主站頁面結構

```
/                          首頁 feed（需登入）
/explore                   探索頁（公開）
/[username]                使用者個版（依個版設定決定是否需登入）
/[username]/[slug]         單篇長文
/post/[id]                 單則討論文（含 thread 展開）
/settings/profile          個人設定
/settings/security         信任等級設定（Passkey、VC、Stamps）
/settings/board            個版設定
```

### 8.4 Admin Tool 頁面結構

獨立部署於 `admin.aleth.app`，僅限內部網路或 VPN 存取。

```
/                          儀表板（DAU、內容量、使用者分布）
/users                     使用者列表（搜尋、篩選）
/users/[id]                使用者詳情（帳號資訊、信任等級、停權記錄）
/contents                  內容列表（搜尋、篩選、下架）
/contents/[id]             內容詳情
/reports                   檢舉佇列
/reports/[id]              檢舉審核
/audit-logs                稽核日誌查詢
/announcements             系統公告管理
/settings/admins           管理員帳號管理（super_admin 限定）
```

Admin Tool 頁面性質偏向即時 CRUD，全部使用 **RSC + no-store**（不快取），確保管理員看到的永遠是最新狀態：

```ts
// Admin 頁面的資料請求一律不快取
const data = await fetch(`/admin/users`, {
  cache: 'no-store',
  headers: { Authorization: `Bearer ${adminToken}` }
})
```

---

## 9. 後端技術選型

所有後端 Service 統一使用 **Go**。

### 9.1 選型理由

| 面向 | 說明 |
|------|------|
| 效能 | 社群網站屬 I/O-bound 工作負載（DB、Redis、外部 API），Go goroutine 排程器處理此類場景極為高效，實際吞吐量與 Rust 差距在 5–15% 以內 |
| GraphQL 生態 | **gqlgen** 是非 JS 語言中最成熟的 schema-first GraphQL server，支援 code generation、DataLoader、subscription |
| WebAuthn | **go-webauthn** 成熟穩定，符合 FIDO2 規範 |
| X.509 / 自然人憑證 | 標準庫 `crypto/x509` 支援完善，自行實作 VC 驗證邏輯成本低 |
| 可觀測性 | OpenTelemetry Go SDK 非常成熟，與 Prometheus、Jaeger 整合良好 |
| 部署 | 編譯為單一靜態 binary，Docker image 極小（distroless 約 10–20 MB）|
| 開發速度 | 學習曲線低，重構成本小，CI 編譯快（相較 Rust 快數倍）|

> **未來擴充**：若後期需要獨立的密碼學驗證模組（例如將內容簽署驗證、VC 驗證邏輯包成對外開放的 library），可考慮以 Rust 實作該模組並以 CGO 或獨立 service 形式整合，但這屬於 Phase 4+ 的議題。

### 9.2 Go 套件選型

| 用途 | 套件 | 說明 |
|------|------|------|
| HTTP Framework | **Chi** | 輕量、符合 net/http 介面、middleware 組合彈性高 |
| GraphQL Server | **gqlgen** | Schema-first，code generation，內建 DataLoader 支援 |
| PostgreSQL Driver | **pgx v5** | 效能最佳的 Go PostgreSQL driver，支援 pgxpool 連線池 |
| Query Builder | **sqlc** | 從 SQL 產生 Go 型別安全程式碼，零 ORM 魔法 |
| Redis | **go-redis v9** | 支援 Redis Streams、Cluster、Pipeline |
| JWT | **golang-jwt/jwt v5** | 標準且維護活躍 |
| WebAuthn | **go-webauthn/webauthn** | FIDO2 / WebAuthn Level 2 相容 |
| DB Migration | **pressly/goose v3** | SQL-first migration，支援 up/down/status，與 sqlc 工作流契合 |
| 排程任務 | **robfig/cron v3** + **BullMQ（Redis）** | Cron 負責觸發，BullMQ（透過 go-redis）負責分散式 job queue |
| 設定管理 | **spf13/viper** | 支援環境變數、config file、熱重載 |
| 可觀測性 | **OpenTelemetry Go SDK** | Trace、Metric、Log 三合一 |
| 結構化日誌 | **rs/zerolog** | 零 allocation，JSON 輸出 |
| 測試 | **testify** + **gomock** | 標準 assertion + mock generation |

### 9.3 Service 間通訊

各 Service 之間的同步呼叫使用 **gRPC**（`google.golang.org/grpc`），非同步事件透過 **Redis Streams**：

```
同步（gRPC）：
  API Gateway → Auth Service       （token 驗證）
  API Gateway → Content Service    （存取控制決策）
  Auth Service → Trust Service     （升級 L2 觸發評分）

非同步（Redis Streams）：
  Content Service → Feed/Reach Svc （新內容發布事件）
  Feed/Reach Svc  → Notif. Service （觸及率更新通知）
  Any Service     → Notif. Service  （通知事件）
```

---

## 10. 安全設計

### 10.1 主站認證與授權

- JWT 使用短效期（15 分鐘）+ Refresh Token（Rotation 機制，防 token 竊取）
- Passkey 驗證採用 Server-side Challenge，防止重放攻擊（replay attack）
- WebAuthn Sign Count 驗證，偵測 credential 被複製
- 所有 API 透過 API Gateway 強制驗證，Content Service 不直接對外暴露

### 10.2 Admin Tool 獨立授權設計

Admin Tool 使用完全獨立於主站的授權系統，兩套 token 互不通用。即使是同一個人，主站的登入狀態**不能**直接存取 Admin Tool。

#### 10.2.1 Admin 登入流程

```
Admin Browser                Admin Gateway              Auth Service (Admin)
     │                            │                            │
     │── POST /admin/auth/login ──▶                           │
     │   { credential_id,         │── 驗證來源 IP / VPN ──────│
     │     passkey_assertion }    │── 驗證 Passkey 簽章 ───────▶
     │                            │◀── admin_id, role ─────────│
     │                            │── 確認 admin_accounts ─────▶
     │                            │── 產生 Admin JWT ──────────│
     │◀── Set-Cookie: admin_session (HttpOnly, Secure, SameSite=Strict)
     │    admin_jwt (short-lived, 15min)
     │    admin_refresh (longer-lived, 4hr)
```

**與主站 token 的關鍵差異：**

| 項目 | 主站 JWT | Admin JWT |
|------|---------|-----------|
| 登入方式 | 密碼 / OAuth / Passkey 擇一 | **強制 Passkey，不接受密碼或 OAuth** |
| Token 有效期 | 15 分鐘 | 15 分鐘 |
| Refresh Token 有效期 | 7 天 | **4 小時**（閒置 30 分鐘自動失效） |
| Token 儲存 | HttpOnly Cookie | HttpOnly Cookie，**額外綁定 IP** |
| Audience claim | `aleth:app` | `aleth:admin`（Admin Gateway 拒絕 `aleth:app` token） |
| 跨 domain 使用 | `aleth.app` | **限定 `admin.aleth.app`** |
| 並行 session 數量 | 不限 | **最多 2 個**（超過自動踢除最舊 session） |

#### 10.2.2 Admin Gateway 驗證邏輯

```
每個 Admin API 請求的驗證流程：

1. 確認請求來源 IP 在允許清單（內部網路 / VPN CIDR）
   → 不符合：直接 403，不透露原因

2. 驗證 admin_jwt：
   - 簽章有效
   - 未過期
   - audience = 'aleth:admin'
   - IP binding 符合（token 發行時記錄的 IP）
   → 不符合：401，要求重新登入

3. 從 admin_jwt 取出 admin_id、role
   → 查詢 admin_accounts 確認帳號未被撤銷（revoked_at IS NULL）

4. 對照 RBAC 規則，確認此 role 有權執行此 API
   → 不符合：403 Forbidden

5. 通過後轉發至 Admin Service，附上 x-admin-id、x-admin-role header
```

#### 10.2.3 RBAC 權限矩陣

| 操作 | super_admin | moderator | support |
|------|:-----------:|:---------:|:-------:|
| 查詢使用者資訊 | ✓ | ✓ | ✓ |
| 停權 / 解除停權使用者 | ✓ | ✓ | ✗ |
| 刪除使用者帳號 | ✓ | ✗ | ✗ |
| 手動覆寫信任等級 | ✓ | ✗ | ✗ |
| 撤銷 L3 VC 驗證 | ✓ | ✗ | ✗ |
| 查詢內容 | ✓ | ✓ | ✓ |
| 下架內容 | ✓ | ✓ | ✗ |
| 強制刪除內容 | ✓ | ✗ | ✗ |
| 檢舉審核（resolve / dismiss） | ✓ | ✓ | ✗ |
| 查詢稽核日誌 | ✓ | ✓（僅自己的操作）| ✗ |
| 發布系統公告 | ✓ | ✗ | ✗ |
| 管理 admin 帳號 | ✓ | ✗ | ✗ |

#### 10.2.4 敏感操作二次確認

下列操作在 Admin Gateway 層強制要求二次確認，即使 JWT 有效也不例外：

- 刪除使用者帳號
- 手動覆寫信任等級
- 撤銷 L3 VC 驗證
- 管理其他 admin 帳號

二次確認機制：要求 admin 再次執行 Passkey assertion，產生包含操作摘要的 challenge，確認確實是本人當下主動操作。

```
// challenge 內容範例（admin 用 Passkey 對此簽署）
{
  "action": "delete_user",
  "target_user_id": "uuid-xxxx",
  "requested_by": "admin_id-yyyy",
  "expires_at": "ISO8601 +5min"
}
```

#### 10.2.5 Admin Session 稽核

每次 Admin 登入與登出皆寫入 `audit_logs`，包含：
- 登入來源 IP
- 使用的 Passkey credential ID
- Session 起訖時間
- 登出原因（主動登出 / token 過期 / 強制踢除）

### 10.3 L3 隱私保護

- 自然人憑證驗證後**不儲存任何明文個資**
- 儲存 `hash(cert_serial_number + platform_salt)`，僅用於防止同一真實身份重複註冊
- platform_salt 定期輪換時，需重新驗證（設定合理有效期）

### 10.4 內容簽署驗證

- 簽署發生在客戶端（Client-side），私鑰不離開裝置
- 後端儲存簽章與公鑰 ID，不儲存私鑰
- 公鑰存於 DID Document，任何人皆可查詢並自行驗證

### 10.5 防濫用

- Rate Limiting：API Gateway 依 IP 與 user_id 雙重限速
- 互動防洗讚：DB 層 UNIQUE constraint 保證同帳號同內容同類型只計一次
- L2 Stamp 防重複：每個外部帳號只能綁定一個 Aleth 帳號

---

## 11. 部署架構

雲端服務商：**Google Cloud Platform（GCP）**

```
                    ┌──────────────────┐
  Users ──HTTPS──▶  │  Cloudflare CDN  │  (DNS proxy + WAF + DDoS protection)
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │  Cloud Load      │
                    │  Balancing       │
                    └────────┬─────────┘
                             │
           ┌─────────────────┼─────────────────┐
           ▼                 ▼                 ▼
      ┌─────────┐      ┌─────────┐      ┌─────────┐
      │Next.js  │      │Next.js  │      │Next.js  │  GKE Pod (Auto-scaled)
      │ Pod     │      │ Pod     │      │ Pod     │
      └────┬────┘      └────┬────┘      └────┬────┘
           └─────────────────┼───────────────┘
                             │ (GKE 內部網路)
                    ┌────────▼─────────┐
                    │   API Gateway    │
                    └────────┬─────────┘
                             │
        ┌────────────────────┼────────────────────┐
        ▼                    ▼                    ▼
  ┌──────────┐        ┌──────────┐         ┌──────────┐
  │Auth Svc  │        │Content   │         │Feed/Reach│
  │(2+ pods) │        │Svc       │         │Svc       │
  └─────┬────┘        └─────┬────┘         └─────┬────┘
        │                   │                    │
        └───────────────────┴────────────────────┘
                            │
              ┌─────────────┼─────────────┐
              ▼             ▼             ▼
        ┌──────────┐  ┌──────────┐  ┌──────────────┐
        │Cloud SQL │  │Memorystore│  │Cloud Storage │
        │(PostgreSQL│  │(Redis)   │  │  (GCS)       │
        │HA + Read  │  └──────────┘  └──────────────┘
        │ Replica) │
        └──────────┘
```

### 11.1 GCP 服務對應

| 用途 | GCP 服務 | 說明 |
|------|---------|------|
| 容器編排 | **GKE（Google Kubernetes Engine）** | Autopilot 模式，依負載自動 scale |
| PostgreSQL | **Cloud SQL for PostgreSQL** | HA 設定，自動備份，Read Replica |
| Redis | **Memorystore for Redis** | Cluster 模式支援 Redis Streams |
| Object Storage | **Cloud Storage（GCS）** | 媒體檔案儲存，搭配 CDN |
| CDN / WAF | **Cloudflare**（DNS proxy）| DDoS 防護、WAF 規則、HTTP/3 |
| Container Registry | **Artifact Registry** | Docker image 存放，整合 Cloud Build |
| CI/CD | **Cloud Build** | 見 §11.3 |
| Secrets 管理 | **Secret Manager** | 透過 Workload Identity 讓 GKE Pod 存取，不使用靜態 key |
| Load Balancer | **Cloud Load Balancing** | Global HTTPS LB，搭配 Cloudflare |
| 監控 | **Cloud Monitoring + Cloud Logging** | 搭配 OpenTelemetry SDK 輸出 Trace/Metric |

### 11.2 資料庫策略

| 用途 | 儲存方案 |
|------|---------|
| 主要資料（使用者、內容、關係） | Cloud SQL PostgreSQL（Primary + Read Replica） |
| Session / Token / Rate Limit | Memorystore Redis |
| Feed Cache / Reach Score 暫存 | Memorystore Redis |
| Message Bus | Memorystore Redis Streams |
| 媒體檔案 | Cloud Storage（GCS） |

### 11.3 CI/CD（Cloud Build）

**Pipeline 觸發條件：**

| 觸發 | 動作 |
|------|------|
| PR → `main` | 執行 test、lint、build（不部署） |
| Merge → `main` | 自動部署到 **staging** 環境 |
| Git tag `v*.*.*` | 部署到 **production**（需 Cloud Build manual approval） |

**Cloud Build 步驟（`cloudbuild.yaml`）：**

```yaml
steps:
  # Go services
  - name: golang:1.23
    entrypoint: go
    args: [test, ./...]
    dir: services

  # Next.js apps
  - name: node:22
    entrypoint: pnpm
    args: [run, lint]
    dir: apps

  # DB migration（staging / prod 部署前執行）
  - name: gcr.io/$PROJECT_ID/goose
    args: [up]
    secretEnv: [DB_URL]

  # Build & push Docker images
  - name: gcr.io/cloud-builders/docker
    args: [build, -t, "$REGION-docker.pkg.dev/$PROJECT_ID/aleth/$_SERVICE:$SHORT_SHA", .]

  - name: gcr.io/cloud-builders/docker
    args: [push, "$REGION-docker.pkg.dev/$PROJECT_ID/aleth/$_SERVICE:$SHORT_SHA"]

  # Deploy to GKE
  - name: gcr.io/cloud-builders/kubectl
    args: [set, image, deployment/$_SERVICE, $_SERVICE=$IMAGE_URL]
    env: [CLOUDSDK_COMPUTE_REGION=$_REGION, CLOUDSDK_CONTAINER_CLUSTER=aleth-$_ENV]

availableSecrets:
  secretManager:
    - versionName: projects/$PROJECT_ID/secrets/db-url/versions/latest
      env: DB_URL
```

**Secret Manager 整合（GKE Workload Identity）：**

```
GKE Pod → [Workload Identity] → Service Account → Secret Manager
（不需要在 Pod 中注入靜態 key，權限由 IAM 控管）
```

各 Service 在啟動時透過 GCP Secret Manager API 讀取 secret，viper 負責將 secret 值注入設定：

```go
// 啟動時從 Secret Manager 讀取，不寫入磁碟
secret, _ := secretManagerClient.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
    Name: "projects/aleth/secrets/db-password/versions/latest",
})
os.Setenv("DB_PASSWORD", string(secret.Payload.Data))
```

---

## 12. 開發階段規劃

### Phase 1 — MVP

- L0 登入（Email/密碼 + Google OAuth）
- 討論文發布、回覆、按讚
- 個版建立 + 長文發布（僅 public / members 兩種權限）
- 基礎 feed（時序排列，無傳播分數加權）
- Admin Tool 基礎版：使用者查詢與停權、內容下架、檢舉佇列

### Phase 2 — 信任系統

- L1 Passkey 支援
- L2 信譽評分（初期支援 GitHub + Twitter Stamp）
- 傳播分數加權 feed
- 內容簽署 + 標記顯示

### Phase 3 — 完整權限與 VC

- L3 台灣自然人憑證驗證
- 個版完整權限設定（訂閱者限定、指定名單、信任等級門檻）
- 系列文章功能
- 評論信任等級門檻設定

### Phase 4 — 擴充

- 更多 L2 Stamp 來源（LinkedIn、ENS 等）
- ActivityPub Federation（Fediverse 互通）
- 更多國家 VC 支援
