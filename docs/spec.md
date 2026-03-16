# Aleth 平台規格文件 v0.1

## 1. 產品願景

Aleth 是一個以**信任為核心**的社群平台，結合短文討論（Threads 風格）與長文書寫（Substack 風格），並借鑒 BBS 的個版概念，讓使用者同時擁有公開廣場與私人空間。平台透過多層次的身份驗證機制，讓高可信度使用者的內容獲得更廣泛的傳播。

---

## 2. 內容類型

### 2.1 討論文（Thread Post）

- 類似 Threads / Twitter 的短文形式
- 支援巢狀回覆（threaded replies）
- **預設完全公開**，不可設定閱讀權限
- 可標記話題（hashtag）與提及使用者（mention）
- 字數上限：建議 500 字（可附圖、影片、連結預覽）
- 支援轉發（Repost）與引用轉發（Quote Repost）

### 2.2 長文（Article）

- 類似 Substack 的完整文章格式
- 支援 Markdown 或 Rich Text 編輯器
- 發布於使用者的「個版」（Personal Board）
- **閱讀權限由作者控制**（詳見第 3 節）
- 可被討論文引用或連結
- 支援系列文章（Series）功能
- 無字數上限

---

## 3. 個版（Personal Board）

BBS 個版的現代化實作：

### 3.1 權限層級

| 設定 | 說明 |
|------|------|
| 公開 | 所有人可讀 |
| 會員限定 | 需登入（L0 以上）才可讀 |
| 訂閱者限定 | 需訂閱該個版才可讀 |
| 指定名單 | 作者手動核准的讀者才可讀 |
| 信任等級門檻 | 限定 L1 / L2 / L3 以上才可讀 |

### 3.2 個版功能

- 自訂個版名稱、介紹、封面
- 訂閱 / 追蹤機制
- 作者可為個別文章設定不同於個版預設的權限
- 可選擇是否開放評論，以及評論者的信任等級門檻

---

## 4. 使用者信任等級（Trust Level）

### 4.1 等級定義

| 等級 | 名稱 | 達成條件 |
|------|------|---------|
| **L0** | 基本會員 | 使用一般登入方式完成註冊：Email/密碼、Google OAuth、Facebook OAuth |
| **L1** | Passkey 會員 | 在帳號設定中配置並啟用 Passkey（FIDO2/WebAuthn） |
| **L2** | 信譽認證會員 | 通過社群信譽評分系統，綜合評分超過設定門檻 |
| **L3** | 真實身份會員 | 提交並驗證通過 Verifiable Credentials（VC），例如台灣自然人憑證 |

### 4.2 L2 信譽評分系統

參考 [Gitcoin Passport](https://passport.gitcoin.co/) 的設計概念：

- 彙整使用者在各平台的社群活躍度指標，例如：
  - GitHub 帳號年齡與活躍度
  - Twitter / X 帳號年齡與互動量
  - LinkedIn 帳號完整度
  - ENS / 區塊鏈身份（可選）
  - 其他可擴充的 Stamp 來源
- 各項指標分別給分，加總超過平台設定門檻（例如 20 分）即達到 L2
- 評分結果定期重新計算（例如每 30 天）
- 使用者可自主選擇提交哪些來源，但每個來源只計算一次

### 4.3 L3 Verifiable Credentials

- 使用 W3C VC 標準
- 初期支援：台灣自然人憑證（MOICA）
- 未來可擴充：其他國家政府 eID、各類專業認證
- 驗證過程：
  1. 使用者在本地端用憑證簽署一個挑戰碼（challenge）
  2. 平台驗證簽章有效性與憑證未過期
  3. **不儲存個人身份資料**，只記錄「已驗證」狀態與有效期
- 同一真實身份只能對應一個 L3 帳號（防止重複帳號）

---

## 5. 內容簽署（Content Signing）

### 5.1 機制概述

使用者可選擇對發布的內容進行密碼學簽署，以證明內容確實出自本人且未被竄改。

### 5.2 簽署方式

| 簽署方式 | 適用等級 | 說明 |
|---------|---------|------|
| Passkey 簽署 | L1 以上 | 使用 WebAuthn 的私鑰對內容雜湊值簽署 |
| VC 簽署 | L3 | 使用自然人憑證或其他 VC 的私鑰簽署 |

### 5.3 簽署資料結構

```json
{
  "content_hash": "sha256(content)",
  "author_did": "did:example:author_identifier",
  "timestamp": "ISO8601",
  "signature": "base64(sign(content_hash + timestamp))",
  "signing_method": "passkey | vc",
  "public_key_id": "key identifier"
}
```

### 5.4 顯示方式

不同信任等級的簽署內容以不同顏色的勾選標記（checkmark badge）呈現，讓讀者一眼辨識作者的可信度：

| 簽署等級 | 標記樣式 | 說明 |
|---------|---------|------|
| L1（Passkey） | 藍色勾勾 ✓ | 已用裝置綁定的 Passkey 簽署，確認為本人裝置發出 |
| L2（信譽認證） | 綠色勾勾 ✓ | 已通過社群信譽評分，具備一定的社群歷史可追溯 |
| L3（VC 真實身份） | 金色勾勾 ✓ | 已通過政府級身份憑證驗證，為最高信任等級 |

- 標記顯示於作者名稱旁，在討論文與長文中皆適用
- 讀者點擊標記後，可展開詳細資訊面板，顯示：
  - 簽署等級與方式
  - 簽署時間戳
  - 公鑰識別碼（public key ID）
  - 「自行驗證」連結，提供簽章原始資料供進階使用者核驗
- 若內容未簽署，則不顯示任何標記（不顯示「未簽署」標示，以免對 L0 使用者造成污名化）
- 標記需區分色盲友善模式：除顏色外，同時以圖示形狀輔助區分（例如 L1 為空心勾、L2 為實心勾、L3 為帶光暈的勾）

---

## 6. 信任加權傳播系統（Trust-Weighted Reach）

### 6.1 傳播分數（Reach Score）

每則發文（討論文與長文）都有一個傳播分數，影響其在演算法中的觸及率。

### 6.2 基礎分數公式

```
傳播分數 = 內容基礎分 × 作者信任乘數 × 互動衰減係數
```

| 作者等級 | 信任乘數 |
|---------|---------|
| L0 | 1.0x |
| L1 | 1.5x |
| L2 | 2.5x |
| L3 | 4.0x |

### 6.3 互動品質加權

- 來自高等級使用者的「按讚」、「分享」、「引用」給予更高的互動加權
- L3 使用者的一個分享 ≈ L0 使用者的 4 個分享（乘數比例與上表一致）
- 防止洗讚機制：同一帳號的重複互動不計分

### 6.4 冷啟動保護

- 新帳號（L0）的發文不會因為信任乘數低而完全沉沒
- 設定基礎保證曝光次數，讓社群有機會發現新成員的內容

---

## 7. 身份識別（DID）

- 每位使用者在平台上擁有一個去中心化識別碼（DID）
- 格式建議：`did:aleth:<unique_id>`
- DID Document 記錄使用者的公鑰，供內容簽署驗證使用
- 使用者可選擇將自己的 DID 公開或匿名

---

## 8. 隱私設計原則

- **最小化資料收集**：L3 驗證只確認「是否通過驗證」，不儲存個人身份資訊
- **使用者控制**：使用者隨時可撤銷 L2 的各項 Stamp 授權
- **透明度**：演算法傳播分數計算邏輯公開說明，不黑箱操作
- **不連結原則**：L3 驗證的真實身份不會與平台使用者名稱公開連結

---

## 9. 技術架構摘要

| 層面 | 技術選型方向 |
|------|------------|
| 身份驗證 | WebAuthn / FIDO2（Passkey）、OAuth 2.0、W3C VC |
| 內容簽署 | Web Crypto API + Passkey / 自然人憑證 |
| 信譽評分 | 可擴充的 Stamp 系統（參考 Gitcoin Passport SDK）|
| DID | W3C DID Core 規範 |
| 聯邦化 | ActivityPub（W3C 標準）、ActivityStreams 2.0 |
| 資料庫 | 使用者資料與內容分開儲存，支援細粒度存取控制 |

---

## 10. 聯邦化（Federation）與 ActivityPub

Aleth 支援 [ActivityPub](https://www.w3.org/TR/activitypub/) 協定，讓使用者可以與 Fediverse（Mastodon、Misskey、Pixelfed 等）的使用者互動，實現跨平台的去中心化社群。

### 10.1 Actor 對應

每位 Aleth 使用者自動擁有一個 ActivityPub Actor，格式如下：

```
https://aleth.example/@{username}
```

Actor JSON-LD 文件範例：

```json
{
  "@context": ["https://www.w3.org/ns/activitystreams", "https://w3id.org/security/v1"],
  "id": "https://aleth.example/@alice",
  "type": "Person",
  "preferredUsername": "alice",
  "name": "Alice",
  "inbox": "https://aleth.example/@alice/inbox",
  "outbox": "https://aleth.example/@alice/outbox",
  "followers": "https://aleth.example/@alice/followers",
  "following": "https://aleth.example/@alice/following",
  "publicKey": {
    "id": "https://aleth.example/@alice#main-key",
    "owner": "https://aleth.example/@alice",
    "publicKeyPem": "..."
  },
  "alsoKnownAs": ["did:aleth:{uuid}"]
}
```

- `alsoKnownAs` 連結至使用者的 Aleth DID，實現 DID 與 ActivityPub 身份的橋接
- 公鑰用於 HTTP Signatures，驗證傳入活動的來源

### 10.2 內容類型對應

| Aleth 內容 | ActivityStreams 類型 | 說明 |
|-----------|-------------------|------|
| 討論文（Thread Post） | `Note` | 短文，直接對應 Mastodon 的 toot |
| 長文（Article） | `Article` | 長文，帶有 `url` 指向 Aleth 頁面 |
| 轉發（Repost） | `Announce` | 標準 boost 行為 |
| 按讚 | `Like` | 標準 like 行為 |
| 回覆 | `Note` with `inReplyTo` | 巢狀回覆 |

### 10.3 支援的 Activity 類型

**對外發布（Outbox）：**

| Activity | 觸發條件 |
|---------|---------|
| `Create(Note)` | 發布公開討論文 |
| `Create(Article)` | 發布公開長文 |
| `Update(Note/Article)` | 編輯已發布內容 |
| `Delete(Note/Article)` | 刪除內容 |
| `Announce` | 轉發其他人的內容 |
| `Like` | 按讚 |
| `Follow` | 追蹤其他 Fediverse 使用者 |
| `Undo(Follow/Like/Announce)` | 取消追蹤 / 收回按讚 / 收回轉發 |

**接收處理（Inbox）：**

| Activity | 處理方式 |
|---------|---------|
| `Create(Note/Article)` | 存入本地快取，顯示於追蹤者的動態 |
| `Announce` | 顯示為轉發 |
| `Like` | 計入讚數（不影響 Aleth 內部信任乘數） |
| `Follow` | 將遠端使用者加入追蹤清單 |
| `Delete` | 從本地快取移除對應內容 |

### 10.4 隱私與存取控制

- **僅公開內容才聯邦化**：個版設為「會員限定」、「訂閱者限定」或「指定名單」的文章，`to` 欄位不包含 `Public`，不會傳送至遠端伺服器
- **成員專屬內容不出站**：`access_policy = 'members'` 的長文完全不進入 Outbox
- **遠端使用者的信任等級**：來自其他 Fediverse 實例的使用者，若沒有在 Aleth 完成登入，視為 L0 等級，只能讀取公開內容

### 10.5 WebFinger 支援

支援 [WebFinger](https://www.rfc-editor.org/rfc/rfc7033)（`/.well-known/webfinger`），讓 Fediverse 伺服器可以查找 Aleth 使用者：

```
GET /.well-known/webfinger?resource=acct:alice@aleth.example
```

回傳使用者的 Actor URL，供遠端伺服器建立追蹤關係。

### 10.6 HTTP Signatures

- 所有對外的 ActivityPub 請求皆以 **HTTP Signatures**（`draft-cavage-http-signatures`）簽署，使用 Actor 的私鑰
- 收到的 ActivityPub 請求皆驗證 HTTP Signature，拒絕簽章無效的活動
- 每位使用者的 ActivityPub 簽章金鑰與 Passkey 簽章金鑰**分開管理**，ActivityPub 簽章由伺服器端代持（server-side key），不需使用者主動操作

### 10.7 與 Aleth 信任系統的互動

- **遠端使用者不參與 L2 信譽評分**：來自其他實例的 Like / Announce 不計入 Aleth 的信任加權傳播分數，避免被外部操控
- **遠端內容顯示**：聯邦化接收的內容以「來自 Fediverse」標示，與本地內容視覺上區分
- **Aleth 使用者可選擇退出聯邦化**：在帳號設定中可關閉 ActivityPub 功能，使個人頁面與內容不對外公開

### 10.8 實作階段規劃

| 階段 | 功能 |
|------|------|
| Phase 2 | WebFinger、Actor 端點、Outbox（Create/Delete Note）、HTTP Signatures |
| Phase 3 | Inbox 接收、Follow/Unfollow、Announce、Like |
| Phase 4 | Article 聯邦化、遠端使用者快取、封鎖 / 靜音遠端實例 |

---

## 11. 待討論的開放問題

1. **L2 門檻分數**應設定在哪個數值？過高會導致門檻太難達到，過低則失去意義。
2. **Passkey 簽署**在 WebAuthn 標準下，每次簽署需要使用者主動確認，這對「每篇文章都簽署」的 UX 是否可接受？或改為定期簽署（例如每次 session 簽署一次公告）？
3. **傳播乘數**的具體數值需要根據實際測試調整，初期數值僅為建議。
4. **匿名性與信任的平衡**：L3 使用者雖然已驗證真實身份，但對外是否要顯示「真實姓名」或維持匿名，僅顯示已驗證標記？
