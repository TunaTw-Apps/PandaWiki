# PandaWiki：可設定的文件與後台帳號配額

修改日期：2026-11-06（台灣時間）。

本分支以官方 `chaitin/PandaWiki` 的 `67e1081053e22a4630da9074b1092b6a7ce33199` 為基底，增加自架版的文件節點與後台帳號配額設定。這是自行維護的修改版，授權沿用根目錄的 AGPL-3.0。

## 使用方式

在 API 服務的環境變數設定所需上限，例如：

```dotenv
PANDAWIKI_MAX_NODES=5000
PANDAWIKI_MAX_ADMINS=10
```

以上為可自行調整的範例值。未設定時維持上游的 300 個節點與 1 個後台帳號。

| 設定 | 計數範圍 | 預設 |
| --- | --- | --- |
| `PANDAWIKI_MAX_NODES` | 每個知識庫的文件節點總數，包含資料夾 | 300 |
| `PANDAWIKI_MAX_ADMINS` | 全站所有後台帳號，包含超級管理員與普通管理員 | 1 |

環境變數接受正整數；空值、0、負數、非整數與超出整數範圍的值會讓 API 啟動明確失敗。調整後重新建立 API 容器，使新環境變數生效。

也能使用現有的 `config.yml`：

```yaml
quota:
  max_nodes: 5000
  max_admins: 10
```

部署設定優先順序為環境變數、設定檔、內建預設值。原有的有效版本設定仍有優先權；本分支只修改基礎版本的兩項數量配額。

配額降低至低於既有資料量時，原有資料會保留，後續新增會被拒絕。

## 修改內容

1. `backend/config/` 增加配額設定與啟動時驗證。
2. HTTP middleware 將部署配額傳入既有 request context；登入與權限檢查持續使用同一個 context。
3. 文件建立與後台帳號建立沿用原有資料庫計數與檢查。
4. 管理介面改由後端判定實際配額，移除按版本硬編碼的帳號數預檢。
5. 超限訊息顯示實際上限，並說明文件節點包含資料夾。
6. 獨立的 GitHub Actions 配額檢查，在分支 push、pull request 與手動執行時驗證設定、資料庫邊界與開源服務建置。

站點數量、管理員分權、SSO 等其他功能仍採用上游的既有條件。這個修改包適用於公開的開源核心，沒有加入獨立 Pro 模組。

## 套用修補檔

下載包中的 `pandawiki-configurable-quotas.patch` 包含全部程式修改、測試與本說明。在自己的 checkout 上套用修補檔：

```bash
git clone https://github.com/chaitin/PandaWiki.git pandawiki-quota
cd pandawiki-quota
git switch -c selfhost/configurable-quotas 67e1081053e22a4630da9074b1092b6a7ce33199
git apply --check /absolute/path/to/pandawiki-configurable-quotas.patch
git apply /absolute/path/to/pandawiki-configurable-quotas.patch
```

套用前先完成 `git apply --check`。若基底版本不同，先處理差異並重新驗證，再建立自己的映像。

## 建置自己的映像

以下命令從 repository 根目錄執行。需要 Docker BuildKit、Node.js 20 以上及 pnpm 10.12.1。Go 編譯環境由原有 API Dockerfile 的 Go 1.24.3 builder 提供。

### API

```bash
docker build \
  --file backend/Dockerfile.api \
  --build-arg VERSION=quota-20261106 \
  --tag local/pandawiki-api:quota-20261106 \
  backend
```

### 管理介面

```bash
pnpm --dir web install --frozen-lockfile
NODE_OPTIONS=--max-old-space-size=6144 pnpm --dir web/admin build
docker build \
  --file web/admin/Dockerfile \
  --tag local/pandawiki-admin:quota-20261106 \
  web/admin
```

管理介面也需要使用本分支的建置結果，才能讓新增帳號操作交由新的後端配額判定。

本次管理介面建置使用 6 GiB 的 Node.js heap 上限。可以在記憶體足夠的開發機完成建置，再將映像交給部署主機執行；此值只調整建置程序的記憶體額度。

上述命令建立本機映像，不會推送 registry 或改動正在運行的服務。交付時是否已完成各項建置，請看下載包中的 `VERIFICATION.json`。

## 接入既有部署

先備份資料庫與目前的部署設定，並保留舊映像標籤。將既有部署中的 API 與管理介面映像替換成以上自建標籤，保留其餘服務、磁碟與網路設定。

若使用 Docker Compose，先在實際安裝目錄確認服務名稱：

```bash
docker compose config --services
```

以下片段示範如何讓變數真正傳入 API 容器。請將服務名稱對應到既有 Compose 檔中的實際名稱，再合併設定：

```yaml
services:
  your-api-service:
    image: local/pandawiki-api:quota-20261106
    environment:
      PANDAWIKI_MAX_NODES: "${PANDAWIKI_MAX_NODES:-5000}"
      PANDAWIKI_MAX_ADMINS: "${PANDAWIKI_MAX_ADMINS:-10}"
  your-admin-service:
    image: local/pandawiki-admin:quota-20261106
```

Compose 目錄中的 `.env` 用於變數替換；需要透過 `environment` 或 `env_file` 將兩項設定傳入 API 容器。單純在主機建立 `.env`，不會自動改變容器內的配額。

確認合併後設定，再重新建立對應服務。這次沒有新增資料庫 migration；回復舊映像或移除新增環境變數後，既有資料仍會保留，但新增操作會重新套用該版本的上限。

## 驗證

### 單元與 HTTP middleware 測試

在 `backend/` 執行：

```bash
go test ./config ./domain ./server/http -count=1
```

覆蓋預設值、環境變數與設定檔優先順序、無效環境變數、版本設定優先權、request context 傳遞，以及其他功能旗標的隔離。

### PostgreSQL 整合測試

準備測試用 PostgreSQL，讓測試帳號有建立 schema 的權限，並在 `backend/` 執行：

```bash
PANDAWIKI_TEST_POSTGRES_DSN='host=127.0.0.1 port=5432 user=your_test_user password=your_test_password dbname=your_test_database sslmode=disable' \
  go test ./repo/pg -run '^TestQuotaIntegration' -v -count=1 -timeout 2m
```

測試在各自的臨時 schema 建立 Node、User 資料表，直接呼叫正式 Repository，結束時清除該 schema。沒有設定測試 DSN 時，會明確顯示 Skip。

測試範圍包含：

- 預設 300 個節點／1 個帳號時，下一筆被拒絕。
- 改成 301／2 時，第 301 個節點與第 2 個帳號建立成功，再下一筆被拒絕。
- 資料夾、超級管理員與普通管理員的計數。
- 第二個帳號的密碼驗證。
- 降低配額後，既有資料仍保留。

整合測試驗證循序新增的邊界，沿用上游的 transaction 內 count → insert 行為。嚴格的並行新增配額鎖、完整歷史 migration、AI 模型連線與正式部署另需在實際環境驗證。

### GitHub Actions

`.github/workflows/quota-tests.yml` 提供獨立的 `Quota Regression Check` 工作流程，檢查名稱為 `Quota tests`。流程在所有分支的 push、pull request，以及 `workflow_dispatch` 手動執行時觸發。

流程使用 `backend/go.mod` 指定的 Go 版本與暫時的 PostgreSQL 17 service，執行下列項目：

1. `config`、`domain`、HTTP middleware 測試。
2. PostgreSQL 的文件節點與後台帳號配額邊界測試。
3. 開源版 API、consumer、migrate 的編譯。

資料庫連線使用 GitHub 分配的 service port。CI 先檢查 DSN，再從 Go 的 JSON 測試報告確認兩個預期整合測試都有實際通過；缺少 DSN、測試跳過、測試名稱消失或任一測試失敗，都會讓工作失敗。

這條流程只需 repository 的讀取權限，使用公開核心與臨時測試資料庫。Checkout 明確關閉 Pro 子模組，並以完整 commit SHA 固定兩個 GitHub Actions。既有官方發布流程保持原樣；其官方 registry 與 Pro 存取設定由該流程自行管理。

套用補丁並推送到自己的 GitHub fork 後，在 Actions 頁面確認此流程已啟用。手動執行入口需等 workflow 檔進入預設分支後才會出現。確認第一輪成功後，可將 `Quota tests` 加入預設分支的必要檢查。這條流程涵蓋所有 PR，方便作為合併前的固定檢查。

目前交付包包含本機測試與 workflow 驗證結果；GitHub runner 的實際執行結果需待補丁進入遠端 fork 後確認。

## 升級與公開原始碼

建議將這組修改保持為少量、獨立的提交，更新上游時重新套用並跑配額測試。將自己的映像標籤與來源 commit 對應起來，方便回復與追查。

透過網路提供修改版時，依 AGPL-3.0 第 13 條，向使用者顯著提供與實際運行版本對應的完整相應原始碼取得方式，並保留授權與修改聲明。可以使用自己的公開 fork 或原始碼下載包；資料庫內容與實際憑證應分開管理。
