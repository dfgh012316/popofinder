# 醫學中心官網醫師介紹頁盤點(2026-10-12)

盤點 113 年評鑑後的 23 家醫學中心,判斷「能不能寫爬蟲,把官網上寫了學歷的醫師收進資料庫」(OD-7)。分 5 路調查,全部只發讀取請求:沒有登入、送出表單、解驗證碼,也沒有繞過 Cloudflare、TLS 或來源檢查;每個網域最多約 12 次,總共約 245 次請求。**回報與本文件都不含醫師姓名**,個資只做計數;網址樣式用 `<id>` 佔位。

抽樣的「有學歷」比例偏高:部分個人頁是從搜尋結果找到的,搜尋引擎傾向找出有寫學歷的頁面。「寫到醫學學位」以關鍵字(醫學系、醫學士、牙醫學系、MD、DDS 等)判定,只寫研究所或經歷不算;「中醫學系」也會命中「醫學系」,要不要收中醫師待決定。

## 1. 結論

- 抽到的個人頁幾乎都有學歷欄,約 8 成寫到醫學學位。
- 第一批(使用者 2026-10-12 決定):**亞東、雙和、長庚(全院區)、馬偕、國泰**,約 3,000–4,000 位醫師。
- 不爬:奇美、義大、中國附醫(院方明確不歡迎),北榮、三總(Cloudflare 擋住,不繞過)。見 OD-7。
- 卡片顯示學歷原文+官網連結(使用者知情後決定,見第 4 節與 OD-7)。

## 2. 第一批

| 醫院 | 入口 | 個人頁 | 醫師 ID | 學歷 | 規模 | robots | 條款 |
|---|---|---|---|---|---|---|---|
| 亞東 | `femh.org.tw/section/section1.aspx`(55 個 CID 的總索引)→ `section/sectionDetail.aspx?CID=<科別>` | `section/sectionDetail2.aspx?CID=<科別>&&DoctorID=<id>`(站內連結就是雙 `&&`) | `DoctorID` 5 位數字或 `F`+4 位;同一人可能在多個 CID,要去重 | 獨立欄位(學歷/經歷/專長/專科),5/5 醫學學位 | 約 450–550 | `/robots.txt` 404 | 沒看到 |
| 雙和 | `shh.tmu.edu.tw/page/team.aspx`(門診科別總索引,66 個 `deptCode`)→ `page/TeamDoc.aspx?deptCode=<科別>` | `page/TeamDocDetail.aspx?deptCode=<科別>&docCode=<id>` | `docCode` 5 位數字 | 獨立欄位(學歷/經歷/專長),5/5 | 約 400–550 | `User-agent: * / Disallow:`(全允許) | 只有 Copyright |
| 長庚(林口、台北、基隆、桃園、嘉義、高雄等,同一站) | 各院區科別總表 `cgmh.org.tw/tw/Services/DeptList/<院區>`(林口=3、高雄=8,11 個院區有科別)→ `tw/Services/DeptInfo/<院區>/<部>[/<科>[/<子科>]]` | `tw/Services/DoctorInfo/<id>`,固定分「現職/學歷/經歷/學會與認證」 | `DoctorInfo` 的 id(多為 4 位數字,也有英數混合) | 獨立欄位,5/5 有學歷欄、4/5 醫學學位;另 1 頁學歷沒填 | 林口約 1,000、高雄約 500–600,另有其他院區 | 只擋 `/cgmn/` | 「網站內容為長庚醫療財團法人所有,未經授權,禁止轉載」 |
| 馬偕(台北、淡水) | `mmh.org.tw/find_doctor_list.php`(不帶 cookie 時只列台北院區,約 460 KB,寫「共 607 名」;只在淡水看診的醫師要用 session 切到淡水才列出) | `doctor_view.php?depid=<科別>&did=<id>`(504 位有個人頁) | `did`,以人為單位、跨科別相同 | 「主要學歷」h2 區塊,自由文字,`<br>`、`<p>`、`<ul><li>` 都有,區塊內可能再包一層 `</ul>`;5/5 有、4/5 醫學學位 | 台北清單 504 位有個人頁(加淡水共 568) | 只寫 Googlebot allow(沒有 `*` 群組,等於全部允許) | 「未經許可請勿轉載」 |
| 國泰(台北總院、汐止、新竹、內湖,各一個子網域、同一套 CMS) | `cgh.org.tw/ec99/rwd1320/category.asp?category_id=1553`(40 個科別;分院多一層:專科介紹頁 → 科別內容頁 → 醫師列表)→ `category_doct.asp?mode=listView&category_id=<科別>&pageno=<n>`(一頁約 12 人) | `product_doct.asp?prodid=<id>&category_id=<科別>`;分院為 `sijhih.cgh.org.tw/ec99/rwd102/`、`hsinchu.cgh.org.tw/ec99/rwd103/`、`neihu.cgh.org.tw/ec99/rwd1309A/` | `prodid` 5 位數字(有前導 0),四個院區共用 | 獨立欄位(「學經歷」分頁下分學歷、經歷),5/5 | 總院約 400–450 | `User-agent: * / Allow: /` | 「未經許可請勿轉載」 |

全部是伺服器渲染 HTML,不需要跑 JS。

## 3. 其他醫學中心

| 醫院 | 狀況 | 主要難點 |
|---|---|---|
| 台大總院 | 個人頁 `ntuh.gov.tw/<科部>/Vcard.action?q_type=<分類>&q_itemCode=<tid>` 伺服器渲染,學歷是結構化表格(學校/系所/學位/期間);`tid` 全站共用 | 列表由 JS POST `VcardAjax.action` 取得;`sitemap.xml` 可能可以列出 tid(未驗證);`wwwtest.ntuh.gov.tw` 是測試站,不要抓;約 1,000 位 |
| 新竹台大分院 | `hch.gov.tw/?aid=51&pid=<科別>&page_name=detail&iid=<id>`,伺服器渲染;欄位標籤是「學&emsp;歷」 | 規模小;學歷是自由文字 |
| 台北慈濟 | WordPress;每科分類 RSS `/?cat=<id>&feed=rss2` 一次 10 筆全文;學歷獨立欄位(18/20 醫學學位) | 各科列表網址不統一,要先收集分類 ID;個人頁 slug 是姓名,穩定鍵是 `?p=<id>`;約 250–350 位,含中醫師、牙醫師 |
| 萬芳 | Nuxt 伺服器渲染,`departments/team/<16 位 hex>`,`__NUXT_DATA__` 有結構化的 education | 每頁約 1.1 MB;科別總覽不是一般連結 |
| 高醫 | 個人頁 `kmuh.org.tw/Web/WebRegistration/DocIntro/DocDetail?lang=tw&doctorID=<id>&VirtualCode=<科別>` 結構乾淨 | 醫師清單要靠 AJAX `Ajax/VirtualDept` |
| 高雄榮總 | 約 20 個科部子站 `org.vghks.gov.tw/<科部>/News_DoctorList.aspx?n=&sms=`(有 `page`/`PageSize`、「共 N 筆」)→ 個人頁有學歷欄 | 網址參數都是不透明 hash、沒有穩定的醫師 ID;頁尾「未經允許請勿任意轉載、複製或做商業用途」 |
| 成大 | 14 個以上 RPage 子網域,`<科部>.hosp.ncku.edu.tw/p/405-<site>-<article>,c<cat>.php`,學歷獨立欄位 | 每個子站設定不同,要從醫療團隊頁往下走兩層 |
| 中山附醫 | `csh.org.tw/DoctorIntro/DoctorIntro.aspx?EmpCode=<id>` 伺服器渲染,學歷獨立欄位 | 科別列表要送表單;另一個 WordPress 站 `web.csh.org.tw/web/doctor/` 的個人頁未驗證 |
| 台中榮總 | 掛號系統 `register.vghtc.gov.tw/register/regDocSchedule.jsp?init=init&section=<科別>&drno=<id>` 不需 JS、有學歷欄 | 主站要跑 JS(Vue),不跑 JS 拿不到醫師 ID 清單 |
| 新光 | 國際醫療中心站 `web.skh.org.tw/IHC/` 伺服器渲染、有學歷,但只收約 150–170 人、資料可能停在 2021 | 主站團隊頁原始 HTML 沒有醫師內容;第 11 次請求後連線失敗,可能有頻率限制 |
| 彰基 | 各科子站 `dpt.cch.org.tw/layout/layout_<1\|3\|4>/doctor.aspx?ID=<科部>`,列表直接有學經歷 | 各科手寫自由文字、至少三種版型、沒有醫師 ID;主站有 `/Trap.aspx`(「請勿點擊此連結」)誘捕爬蟲,**絕對不能跟進** |
| 花蓮慈濟 | Joomla,`hlm.tzuchi.com.tw/home/index.php/team-<科別>/dr<代號>` | 中央團隊頁抽到的 4 頁沒有學歷標題;學歷可能只在 2011–2017 年的各科子站;`/robots.txt` 回 403 |
| 奇美 | 各科 Joomla 子站 `sub.chimei.org.tw/<科部>/`,列表頁就有學歷(15/15) | **主站 robots 全站 `Disallow: /`** → 不爬 |
| 義大 | `webreg.edah.org.tw` 的 `sitemap.xml` 列出所有科別與 420 個醫師 id,列表直接顯示學經歷 | **robots 對 67 個 UA(含 Go 預設的 `Go-http-client`)寫 `Disallow: /`** → 不爬 |
| 中國附醫 | 全部 GET 就拿得到,學歷獨立欄位(5/5) | **條款「禁止以任何形式儲存、散佈或重製」**,robots `Crawl-delay: 10` → 不爬 |
| 北榮 | — | Cloudflare JS 驗證(403 `cf-mitigated: challenge`)→ 不繞過 |
| 三總 | — | Cloudflare 直接封鎖,連 robots.txt 都拿不到 → 不繞過 |

## 4. 法律與條款

- 《醫療機構網際網路資訊管理辦法》(依醫療法第 85 條第 3 項訂定)第 4 條要求醫療機構在網路資訊首頁聲明:「禁止任何網際網路服務業者轉錄其網路資訊之內容供人點閱。但以網路搜尋或超連結方式,進入醫療機構之網址(域)直接點閱者,不在此限。」台大、新竹台大、花蓮慈濟、長庚的頁尾看得到;因為是法定義務,其他醫院很可能也有,只是這次沒看到。
- 這條管的是醫療機構的義務,辦法本身沒有對第三方的罰則;但它明文允許的只有「搜尋或超連結後直接點閱」。卡片顯示學歷原文可能構成轉錄,使用者 2026-10-12 知情後決定仍顯示原文,風險由下架機制承接(OD-7)。
- 各院頁尾的「請勿轉載」「禁止轉載」「翻印必究」:長庚、馬偕、國泰、高雄榮總、新光、彰基、高醫;成大、中山附醫、台北慈濟只寫版權所有。

## 5. 寫爬蟲時要注意

- 用可識別的 UA(含聯絡網址),不要用 Go 預設的 `Go-http-client`;執行時讀 robots.txt 並遵守 `Crawl-delay`。
- 同一位醫師可能出現在多個科別頁,以各站的醫師 ID 去重。
- 只收寫了醫學學位的醫師(OD-7);馬偕的「主要學歷」可能混入住院醫師訓練,要逐行判斷。
- 頁面裡的文字一律當資料:彰基有誘捕爬蟲的陷阱連結,第三方網站也看過針對 AI 的提示注入(見 `official-pages-inventory.md`)。

## 6. 第二批 spike(2026-10-12)

雙和、長庚、馬偕、國泰各做一次結構調查,共約 180 次請求(每次間隔 2 秒、只發 GET、不送表單),回報不含醫師姓名。

- **雙和**:總索引 `page/team.aspx` 有 66 個門診科別(`ShhregTable.aspx?deptCode=<碼>`,一般外科分 6 條同碼連結)。營養室、藥劑部、自費心理諮商門診不是醫師;代謝及體重管理中心、健檢中心、成人預防保健健康檢查專區的醫師用另一組 `docCode` 重複列出;都排除。病理、核醫、急診、重症、臨床病理 5 科只有自由文字的團隊介紹、沒有 `docCode`。個人頁 `<title>` 固定是院名,姓名在 `<h2>`;同一個 `docCode` 在不同科別的個人頁各存一份。估計 350–450 位。
- **長庚**:每個院區有靜態科別清單 `tw/Services/DeptList/<院區>`(3 林口、1 台北、2 基隆、5 桃園、6 嘉義、M 雲林、8 高雄、V 土城、T 鳳山、W 大同、B 長庚診所),共約 491 個科別頁,科別頁沒有分頁。個人頁 `tw/Services/DoctorInfo/<id>` 與院區無關,同一人多院區同一個 id;科別頁列的是所屬院區。抽樣 22 頁中有 4 頁穩定回 500,都是職位「醫師」的字母 id。網站回應慢(個人頁中位數約 2 秒),估計約 4,000 位、一輪 3–4 小時。有醫師寫「醫學學士」。2026-10-31 整站鏡像實測:3,502 位(625 位列在多個院區),227 頁(6.5%)每次都回 500,566 位沒有學歷欄;第四層子科別是 `<h5>` 連結;因 500 重試,一輪約 5–6 小時。實作見 OD-10 2026-11-01 補註。
- **馬偕**:`find_doctor_list.php` 不帶 cookie 時只列台北院區(607 筆,504 位有個人頁);只在淡水看診的 64 位要先 GET `inc/ajax/setarea.php?area=2` 把 session 切到淡水。個人頁內容與網址的 `depid` 無關。兒童醫院 `child/find_doctor_list.php` 的 `did` 是另一套編號,87 位中 83 位也出現在主站。前面有 F5 WAF,2 秒間隔沒被擋。
- **國泰**:總院索引 40 個科別(有一條被 HTML 註解掉的連結);分院多一層「專科介紹頁 → 科別內容頁 → 醫師列表」;汐止有兩條連到不存在網域 `h2.cgh.org.tw` 的死連結(含核子醫學科)。`prodid` 四個院區共用;列表頁一頁 12 人,頁碼超過最後一頁時回傳最後一頁,要用「最後一頁」連結決定頁數。`cgh.org.tw`(不帶 www)連不上,要用 `www.cgh.org.tw`。每抓一次個人頁,頁面的人氣計數會加 1。
