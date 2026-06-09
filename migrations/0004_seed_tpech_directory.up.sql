-- 種子資料:臺北市立聯合醫院官網醫師名冊抽取的 17 筆(爬取 spike 成果)。
-- 來源 source='hospital_directory'、verification_status='unverified'(爬來未驗證),source_url 保留原始頁面以利日後校對 / 下架。
-- 以 source_url 做冪等保護,重跑不會重複插入。
INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院仁愛院區', '消化內科', '李熹昌', '私立中山醫學院醫學系、 國立政治大學經營管理碩士(EMBA)', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=146'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=146');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院中興院區', '眼科', '施智偉', '國立陽明大學醫學系畢業、國立台灣大學臨床醫學研究所碩士畢業', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=2828'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=2828');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院和平院區', '神經內科', '劉建良', '慈濟大學醫學系', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=3586'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=3586');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院仁愛院區', '胸腔外科', '潘滄興', '菲律賓遠東醫學院', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=106'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=106');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院仁愛院區', '一般外科', '蔡孟叡', '台北醫學院醫學系', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=107'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=107');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院仁愛院區', '消化內科', '楊旻達', '高雄醫學院醫學系畢業', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=114'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=114');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院仁愛院區', '放射線腫瘤科', '劉千如', '中山醫學院醫學系', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=117'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=117');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院仁愛院區', '血液腫瘤科', '林哲斌', '國立陽明醫學院醫學院醫學士 國立陽明醫學院公共衛生研究所碩士', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=119'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=119');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院忠孝院區', '急診醫學科', '李彬州', '臺北醫學大學醫學系畢業', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=120'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=120');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院', '中醫內科', '邱榮鵬', '中國醫藥大學中醫學系畢業', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=121'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=121');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院', '家庭醫學科', '林光洋', '台大醫學院醫學系', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=124'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=124');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院仁愛院區', '小兒科', '吳琦森', '台北醫學大學醫學系畢業、 台灣大學附設醫院小兒心臟科進修', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=125'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=125');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院陽明院區', '小兒科、感染科、臨床病理科', '林佩菁', '台北醫學大學醫學系', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=129'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=129');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院仁愛院區', '內分泌及新陳代謝科', '廖學崇', '私立高雄醫學院', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=132'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=132');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院仁愛院區', '消化內科.一般內科', '林志陵', '台灣大學醫學院臨床醫學研究所醫學碩士、 私立中山醫學院醫學系畢業', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=133'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=133');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院仁愛院區', '心臟血管內科', '謝志民', '台北醫學院醫學系醫學士', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=136'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=136');

INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url)
SELECT '臺北市', '臺北市立聯合醫院仁愛院區', '心臟血管內科', '陳鉞忠', '中山醫學大學醫學系、 長榮大學健康科學學院醫學研究所碩士、 國防醫學院醫學科學所博士', 'hospital_directory', 'unverified', 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=137'
WHERE NOT EXISTS (SELECT 1 FROM medical_personnel WHERE source_url = 'https://websrv01.tpech.gov.tw/drview/Home/Read?user_id=137');
